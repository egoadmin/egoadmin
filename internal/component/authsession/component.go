package authsession

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/gotomicro/ego/core/elog"
)

type Component struct {
	name      string
	config    *Config
	logger    *elog.Component
	cache     recordCache
	store     indexStore
	keys      keyBuilder
	validator ContextValidator
	recorder  EventRecorder
	idgen     IDGenerator
	signKey   []byte
}

func newComponent(
	name string,
	config *Config,
	logger *elog.Component,
	cache recordCache,
	store indexStore,
	validator ContextValidator,
	recorder EventRecorder,
	idgen IDGenerator,
) (*Component, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	config.normalize()
	if config.JWTSignKey == "" {
		return nil, fmt.Errorf("%w: jwt sign key is empty", ErrInvalidConfig)
	}
	if cache == nil {
		return nil, fmt.Errorf("%w: record cache is nil", ErrInvalidConfig)
	}
	if store == nil {
		return nil, fmt.Errorf("%w: index store is nil", ErrInvalidConfig)
	}
	if idgen == nil {
		return nil, fmt.Errorf("%w: id generator is nil", ErrInvalidConfig)
	}

	return &Component{
		name:      name,
		config:    config,
		logger:    logger,
		cache:     cache,
		store:     store,
		keys:      newKeyBuilder(config.KeyPrefix),
		validator: validator,
		recorder:  recorder,
		idgen:     idgen,
		signKey:   []byte(config.JWTSignKey),
	}, nil
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) PackageName() string {
	return PackageName
}

func (c *Component) ExpiresAt() time.Time {
	return time.Now().Add(c.config.AccessTokenTTL - c.config.AccessTokenDisplaySkew)
}

func (c *Component) Issue(ctx context.Context, req IssueRequest) (*IssueResult, error) {
	if req.UserID == 0 || req.Username == "" || req.UA == "" {
		return nil, ErrSubjectInvalid
	}

	now := time.Now()
	auth := &AuthContext{
		UserID:      req.UserID,
		Username:    req.Username,
		UserType:    req.UserType,
		UA:          req.UA,
		WorkspaceID: req.WorkspaceID,
		Subject:     req.Username,
		IssuedAt:    now,
		ExpiresAt:   now.Add(c.config.AccessTokenTTL),
	}
	if err := c.validateContext(ctx, auth); err != nil {
		return nil, err
	}

	if err := c.prepareSessionSlot(ctx, req, now); err != nil {
		return nil, err
	}

	sid, err := c.idgen.NewID()
	if err != nil {
		return nil, err
	}
	jti, err := c.idgen.NewID()
	if err != nil {
		return nil, err
	}
	refreshToken, err := c.idgen.NewOpaqueToken()
	if err != nil {
		return nil, err
	}

	auth.SessionID = sid
	auth.TokenID = jti
	refreshHash := tokenHash(refreshToken)
	accessToken, err := c.signAccessToken(auth, now)
	if err != nil {
		return nil, err
	}

	session := &SessionRecord{
		ID:                 sid,
		UserID:             req.UserID,
		Username:           req.Username,
		UserType:           req.UserType,
		UA:                 req.UA,
		DeviceHash:         deviceHash(req.UA),
		IP:                 req.IP,
		WorkspaceID:        req.WorkspaceID,
		CurrentAccessID:    jti,
		CurrentRefreshHash: refreshHash,
		Status:             StatusActive,
		LoginAt:            now,
		LastActiveAt:       now,
		ExpiresAt:          now.Add(c.config.RefreshTokenTTL),
	}
	access := &AccessRecord{
		ID:        jti,
		SessionID: sid,
		UserID:    req.UserID,
		UA:        req.UA,
		TokenHash: tokenHash(accessToken),
		Status:    StatusActive,
		IssuedAt:  now,
		ExpiresAt: now.Add(c.config.AccessTokenTTL),
	}
	refresh := &RefreshRecord{
		Hash:      refreshHash,
		SessionID: sid,
		AccessID:  jti,
		UserID:    req.UserID,
		Status:    StatusActive,
		IssuedAt:  now,
		ExpiresAt: now.Add(c.config.RefreshTokenTTL),
	}

	if err := c.saveSession(ctx, session, c.config.RefreshTokenTTL); err != nil {
		return nil, err
	}
	if err := c.saveAccess(ctx, access, c.config.AccessTokenTTL); err != nil {
		return nil, err
	}
	if err := c.saveRefresh(ctx, refresh, c.config.RefreshTokenTTL); err != nil {
		return nil, err
	}
	if err := c.store.AddUserSession(ctx, c.keys.userSessions(req.UserID), sid, float64(now.UnixMilli()), c.config.RefreshTokenTTL); err != nil {
		return nil, err
	}
	if c.config.SameDeviceStrategy != SameDeviceAllow {
		if err := c.store.SetDeviceSession(ctx, c.keys.deviceSession(req.UserID, session.DeviceHash), sid, c.config.RefreshTokenTTL); err != nil {
			return nil, err
		}
	}

	c.record(ctx, "issue", auth, req.IP, StatusActive)
	return &IssueResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    c.ExpiresAt(),
		Auth:         auth.Clone(),
	}, nil
}

func (c *Component) Refresh(ctx context.Context, req RefreshRequest) (*IssueResult, error) {
	if req.RefreshToken == "" {
		return nil, errRefreshHashMissing
	}

	now := time.Now()
	refreshHash := tokenHash(req.RefreshToken)
	refresh, err := c.getRefresh(ctx, refreshHash)
	if err != nil {
		return nil, err
	}
	if refresh.Status == StatusRotated || refresh.Status == StatusRefreshReused {
		_ = c.RevokeSession(ctx, refresh.SessionID, StatusRefreshReused)
		return nil, ErrRefreshReused
	}
	if refresh.Status != StatusActive {
		return nil, ErrTokenRevoked
	}
	if !refresh.ExpiresAt.IsZero() && now.After(refresh.ExpiresAt) {
		return nil, ErrRefreshExpired
	}

	session, err := c.getSession(ctx, refresh.SessionID)
	if err != nil {
		return nil, err
	}
	if session.Status != StatusActive {
		return nil, ErrSessionRevoked
	}
	if session.CurrentRefreshHash != refresh.Hash {
		_ = c.RevokeSession(ctx, session.ID, StatusRefreshReused)
		return nil, ErrRefreshReused
	}

	auth := authContextFromSession(session)
	auth.IssuedAt = now
	auth.ExpiresAt = now.Add(c.config.AccessTokenTTL)
	if validateErr := c.validateContext(ctx, auth); validateErr != nil {
		return nil, validateErr
	}

	jti, err := c.idgen.NewID()
	if err != nil {
		return nil, err
	}
	newRefreshToken, err := c.idgen.NewOpaqueToken()
	if err != nil {
		return nil, err
	}
	auth.TokenID = jti
	accessToken, err := c.signAccessToken(auth, now)
	if err != nil {
		return nil, err
	}
	newRefreshHash := tokenHash(newRefreshToken)

	if oldAccess, er := c.getAccess(ctx, refresh.AccessID); er == nil {
		oldAccess.Status = StatusRevoked
		oldAccess.RevokedAt = now
		oldAccess.RevokeReason = StatusRotated
		_ = c.saveAccess(ctx, oldAccess, c.config.RevokedRecordTTL)
	}
	refresh.Status = StatusRotated
	refresh.RotatedAt = now
	refresh.NextTokenHash = newRefreshHash
	if err := c.saveRefresh(ctx, refresh, c.config.RevokedRecordTTL); err != nil {
		return nil, err
	}

	session.CurrentAccessID = jti
	session.CurrentRefreshHash = newRefreshHash
	session.LastActiveAt = now
	if err := c.saveSession(ctx, session, time.Until(session.ExpiresAt)); err != nil {
		return nil, err
	}

	access := &AccessRecord{
		ID:        jti,
		SessionID: session.ID,
		UserID:    session.UserID,
		UA:        session.UA,
		TokenHash: tokenHash(accessToken),
		Status:    StatusActive,
		IssuedAt:  now,
		ExpiresAt: now.Add(c.config.AccessTokenTTL),
	}
	nextRefresh := &RefreshRecord{
		Hash:      newRefreshHash,
		SessionID: session.ID,
		AccessID:  jti,
		UserID:    session.UserID,
		Status:    StatusActive,
		IssuedAt:  now,
		ExpiresAt: session.ExpiresAt,
	}
	if err := c.saveAccess(ctx, access, c.config.AccessTokenTTL); err != nil {
		return nil, err
	}
	if err := c.saveRefresh(ctx, nextRefresh, time.Until(session.ExpiresAt)); err != nil {
		return nil, err
	}

	c.record(ctx, "refresh", auth, req.IP, StatusActive)
	return &IssueResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    c.ExpiresAt(),
		Auth:         auth.Clone(),
	}, nil
}

func (c *Component) ValidateAccessToken(ctx context.Context, rawToken string) (*AuthContext, error) {
	if rawToken == "" {
		return nil, ErrMissingToken
	}

	claims, err := c.parseAccessToken(rawToken)
	if err != nil {
		return nil, err
	}
	if claims.UID == 0 || claims.UA == "" || claims.SessionID == "" || claims.TokenID == "" {
		return nil, ErrInvalidToken
	}

	now := time.Now()
	access, err := c.getAccess(ctx, claims.TokenID)
	if err != nil {
		return nil, err
	}
	if access.Status != StatusActive {
		return nil, revokedStatusError(ErrTokenRevoked, access.Status, access.RevokeReason)
	}
	if !access.ExpiresAt.IsZero() && now.After(access.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if access.UserID != claims.UID || access.SessionID != claims.SessionID || access.UA != claims.UA {
		return nil, ErrInvalidToken
	}
	if !constantTimeEqual(access.TokenHash, tokenHash(rawToken)) {
		return nil, errTokenHashMismatch
	}

	session, err := c.getSession(ctx, claims.SessionID)
	if err != nil {
		return nil, err
	}
	if session.Status != StatusActive {
		return nil, revokedStatusError(ErrSessionRevoked, session.Status, session.RevokeReason)
	}
	if session.UserID != claims.UID || session.UA != claims.UA || session.CurrentAccessID != claims.TokenID {
		return nil, ErrInvalidToken
	}
	if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	auth := &AuthContext{
		UserID:      claims.UID,
		Username:    claims.Username,
		UserType:    claims.UserType,
		UA:          claims.UA,
		SessionID:   claims.SessionID,
		TokenID:     claims.TokenID,
		WorkspaceID: claims.WorkspaceID,
		Subject:     claims.Username,
		IssuedAt:    numericDateTime(claims.IssuedAt),
		ExpiresAt:   numericDateTime(claims.ExpiresAt),
	}
	if auth.WorkspaceID == 0 {
		auth.WorkspaceID = session.WorkspaceID
	}
	if err := c.validateContext(ctx, auth); err != nil {
		return nil, err
	}
	if now.Sub(session.LastActiveAt) >= c.config.TouchInterval {
		session.LastActiveAt = now
		if err := c.saveSession(ctx, session, time.Until(session.ExpiresAt)); err != nil {
			return nil, err
		}
	}

	return auth.Clone(), nil
}

func (c *Component) Logout(ctx context.Context, auth *AuthContext) error {
	if auth == nil || auth.SessionID == "" {
		return ErrMissingToken
	}
	return c.RevokeSession(ctx, auth.SessionID, StatusLogout)
}

func (c *Component) RevokeUser(ctx context.Context, userID uint64, reason Status) error {
	sids, err := c.store.ListUserSessions(ctx, c.keys.userSessions(userID))
	if err != nil {
		return err
	}
	for _, sid := range sids {
		if err := c.RevokeSession(ctx, sid, reason); err != nil && !errors.Is(err, errRecordNotFound) {
			return err
		}
	}
	return nil
}

func (c *Component) RevokeUserWorkspace(ctx context.Context, userID uint64, workspaceID uint64, reason Status) error {
	if userID == 0 || workspaceID == 0 {
		return nil
	}
	sids, err := c.store.ListUserSessions(ctx, c.keys.userSessions(userID))
	if err != nil {
		return err
	}
	for _, sid := range sids {
		session, er := c.getSession(ctx, sid)
		if er != nil {
			if errors.Is(er, errRecordNotFound) {
				_ = c.store.RemoveUserSession(ctx, c.keys.userSessions(userID), sid)
				continue
			}
			return er
		}
		if session.UserID != userID || session.WorkspaceID != workspaceID {
			continue
		}
		if er = c.RevokeSession(ctx, sid, reason); er != nil && !errors.Is(er, errRecordNotFound) {
			return er
		}
	}
	return nil
}

func (c *Component) RevokeSession(ctx context.Context, sessionID string, reason Status) error {
	if sessionID == "" {
		return nil
	}
	now := time.Now()
	session, err := c.getSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, errRecordNotFound) {
			return nil
		}
		return err
	}
	session.Status = reason
	session.RevokedAt = now
	session.RevokeReason = reason

	if session.CurrentAccessID != "" {
		if access, er := c.getAccess(ctx, session.CurrentAccessID); er == nil {
			access.Status = reason
			access.RevokedAt = now
			access.RevokeReason = reason
			_ = c.saveAccess(ctx, access, c.config.RevokedRecordTTL)
		}
	}
	if session.CurrentRefreshHash != "" {
		if refresh, er := c.getRefresh(ctx, session.CurrentRefreshHash); er == nil {
			refresh.Status = reason
			refresh.RevokedAt = now
			refresh.RevokeReason = reason
			_ = c.saveRefresh(ctx, refresh, c.config.RevokedRecordTTL)
		}
	}
	if err := c.saveSession(ctx, session, c.config.RevokedRecordTTL); err != nil {
		return err
	}
	_ = c.store.RemoveUserSession(ctx, c.keys.userSessions(session.UserID), session.ID)
	_ = c.store.DeleteDeviceSession(ctx, c.keys.deviceSession(session.UserID, session.DeviceHash))
	c.record(ctx, "revoke", authContextFromSession(session), session.IP, reason)
	return nil
}

func (c *Component) prepareSessionSlot(ctx context.Context, req IssueRequest, now time.Time) error {
	if !c.config.MultiLoginEnabled {
		return c.RevokeUser(ctx, req.UserID, StatusReplaced)
	}

	deviceKey := c.keys.deviceSession(req.UserID, deviceHash(req.UA))
	existingSID, err := c.store.GetDeviceSession(ctx, deviceKey)
	if err == nil && existingSID != "" {
		switch c.config.SameDeviceStrategy {
		case SameDeviceReject:
			return ErrSessionExists
		case SameDeviceReplace:
			if revokeErr := c.RevokeSession(ctx, existingSID, StatusReplaced); revokeErr != nil {
				return revokeErr
			}
		}
	} else if err != nil && !errors.Is(err, errRecordNotFound) {
		return err
	}

	if c.config.MaxSessions <= 0 {
		return nil
	}
	sids, err := c.store.ListUserSessions(ctx, c.keys.userSessions(req.UserID))
	if err != nil {
		return err
	}
	active := make([]string, 0, len(sids))
	for _, sid := range sids {
		session, er := c.getSession(ctx, sid)
		if er != nil || session.Status != StatusActive || (!session.ExpiresAt.IsZero() && now.After(session.ExpiresAt)) {
			_ = c.store.RemoveUserSession(ctx, c.keys.userSessions(req.UserID), sid)
			continue
		}
		active = append(active, sid)
	}
	if len(active) < c.config.MaxSessions {
		return nil
	}
	if c.config.OverflowStrategy == OverflowReject {
		return ErrTooManySessions
	}
	revokeCount := len(active) - c.config.MaxSessions + 1
	for i := 0; i < revokeCount; i++ {
		if err := c.RevokeSession(ctx, active[i], StatusKicked); err != nil {
			return err
		}
	}
	return nil
}

func (c *Component) signAccessToken(auth *AuthContext, now time.Time) (string, error) {
	claims := Claims{
		UID:         auth.UserID,
		Username:    auth.Username,
		UserType:    auth.UserType,
		UA:          auth.UA,
		SessionID:   auth.SessionID,
		TokenID:     auth.TokenID,
		WorkspaceID: auth.WorkspaceID,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ID:        auth.TokenID,
			Subject:   strconv.FormatUint(auth.UserID, 10),
			IssuedAt:  jwtv5.NewNumericDate(now),
			NotBefore: jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(c.config.AccessTokenTTL)),
		},
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, &claims)
	raw, err := token.SignedString(c.signKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return raw, nil
}

func (c *Component) parseAccessToken(rawToken string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwtv5.ParseWithClaims(rawToken, claims, func(token *jwtv5.Token) (any, error) {
		return c.signKey, nil
	}, jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwtv5.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}
	if token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (c *Component) validateContext(ctx context.Context, auth *AuthContext) error {
	if c.validator == nil {
		return nil
	}
	if err := c.validator.ValidateAuthContext(ctx, auth); err != nil {
		return fmt.Errorf("%w: %v", ErrSubjectInvalid, err)
	}
	return nil
}

func (c *Component) saveSession(ctx context.Context, session *SessionRecord, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.config.RevokedRecordTTL
	}
	return c.cache.Set(ctx, c.keys.session(session.ID), session, ttl)
}

func (c *Component) saveAccess(ctx context.Context, access *AccessRecord, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.config.RevokedRecordTTL
	}
	return c.cache.Set(ctx, c.keys.access(access.ID), access, ttl)
}

func (c *Component) saveRefresh(ctx context.Context, refresh *RefreshRecord, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.config.RevokedRecordTTL
	}
	return c.cache.Set(ctx, c.keys.refresh(refresh.Hash), refresh, ttl)
}

func (c *Component) getSession(ctx context.Context, sid string) (*SessionRecord, error) {
	var session SessionRecord
	if err := c.cache.Get(ctx, c.keys.session(sid), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *Component) getAccess(ctx context.Context, jti string) (*AccessRecord, error) {
	var access AccessRecord
	if err := c.cache.Get(ctx, c.keys.access(jti), &access); err != nil {
		return nil, err
	}
	return &access, nil
}

func (c *Component) getRefresh(ctx context.Context, refreshHash string) (*RefreshRecord, error) {
	var refresh RefreshRecord
	if err := c.cache.Get(ctx, c.keys.refresh(refreshHash), &refresh); err != nil {
		return nil, err
	}
	return &refresh, nil
}

func (c *Component) record(ctx context.Context, eventType string, auth *AuthContext, ip string, reason Status) {
	if c.recorder == nil || auth == nil {
		return
	}
	c.recorder.RecordAuthEvent(ctx, Event{
		Type:      eventType,
		UserID:    auth.UserID,
		Username:  auth.Username,
		SessionID: auth.SessionID,
		TokenID:   auth.TokenID,
		UA:        auth.UA,
		IP:        ip,
		Reason:    reason,
		At:        time.Now(),
	})
}

func authContextFromSession(session *SessionRecord) *AuthContext {
	if session == nil {
		return nil
	}
	return &AuthContext{
		UserID:      session.UserID,
		Username:    session.Username,
		UserType:    session.UserType,
		UA:          session.UA,
		SessionID:   session.ID,
		TokenID:     session.CurrentAccessID,
		WorkspaceID: session.WorkspaceID,
		Subject:     session.Username,
		IssuedAt:    session.LoginAt,
		ExpiresAt:   session.ExpiresAt,
	}
}

func deviceHash(ua string) string {
	return tokenHash(ua)
}

func numericDateTime(date *jwtv5.NumericDate) time.Time {
	if date == nil {
		return time.Time{}
	}
	return date.Time
}
