package authsession

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
	"github.com/gotomicro/ego/core/eerrors"
	jetcache "github.com/mgtv-tech/jetcache-go"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

type memoryRecordCache struct {
	mu    sync.Mutex
	items map[string]memoryRecord
}

type memoryRecord struct {
	value any
	exp   time.Time
}

func newMemoryRecordCache() *memoryRecordCache {
	return &memoryRecordCache{items: map[string]memoryRecord{}}
}

func (c *memoryRecordCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = memoryRecord{value: cloneValue(value), exp: exp}
	return nil
}

func (c *memoryRecordCache) Get(_ context.Context, key string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok || (!item.exp.IsZero() && time.Now().After(item.exp)) {
		return errRecordNotFound
	}
	switch dst := value.(type) {
	case *SessionRecord:
		v, ok := item.value.(*SessionRecord)
		if !ok {
			return fmt.Errorf("unexpected session value")
		}
		*dst = *v
	case *AccessRecord:
		v, ok := item.value.(*AccessRecord)
		if !ok {
			return fmt.Errorf("unexpected access value")
		}
		*dst = *v
	case *RefreshRecord:
		v, ok := item.value.(*RefreshRecord)
		if !ok {
			return fmt.Errorf("unexpected refresh value")
		}
		*dst = *v
	default:
		return fmt.Errorf("unsupported value type")
	}
	return nil
}

func (c *memoryRecordCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case *SessionRecord:
		cp := *v
		return &cp
	case *AccessRecord:
		cp := *v
		return &cp
	case *RefreshRecord:
		cp := *v
		return &cp
	default:
		return value
	}
}

type memoryIndexStore struct {
	mu      sync.Mutex
	devices map[string]string
	zsets   map[string]map[string]float64
}

func newMemoryIndexStore() *memoryIndexStore {
	return &memoryIndexStore{
		devices: map[string]string{},
		zsets:   map[string]map[string]float64{},
	}
}

func (s *memoryIndexStore) SetDeviceSession(_ context.Context, key string, sessionID string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[key] = sessionID
	return nil
}

func (s *memoryIndexStore) GetDeviceSession(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.devices[key]
	if !ok {
		return "", errRecordNotFound
	}
	return value, nil
}

func (s *memoryIndexStore) DeleteDeviceSession(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.devices, key)
	return nil
}

func (s *memoryIndexStore) AddUserSession(_ context.Context, key string, sessionID string, score float64, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.zsets[key] == nil {
		s.zsets[key] = map[string]float64{}
	}
	s.zsets[key][sessionID] = score
	return nil
}

func (s *memoryIndexStore) RemoveUserSession(_ context.Context, key string, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.zsets[key], sessionID)
	return nil
}

func (s *memoryIndexStore) ListUserSessions(_ context.Context, key string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type item struct {
		sid   string
		score float64
	}
	items := make([]item, 0, len(s.zsets[key]))
	for sid, score := range s.zsets[key] {
		items = append(items, item{sid: sid, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score < items[j].score
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.sid)
	}
	return out, nil
}

func (s *memoryIndexStore) Expire(context.Context, string, time.Duration) error {
	return nil
}

type sequenceIDGenerator struct {
	mu     sync.Mutex
	nextID int
}

func (g *sequenceIDGenerator) NewID() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nextID++
	return fmt.Sprintf("id-%d", g.nextID), nil
}

func (g *sequenceIDGenerator) NewOpaqueToken() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nextID++
	return fmt.Sprintf("refresh-%d", g.nextID), nil
}

func withRecordCache(cache recordCache) Option {
	return func(c *Container) {
		c.recordCache = cache
	}
}

func withIndexStore(store indexStore) Option {
	return func(c *Container) {
		c.indexStore = store
	}
}

func withIDGenerator(generator IDGenerator) Option {
	return func(c *Container) {
		c.idGenerator = generator
	}
}

func newTestComponent(t *testing.T, opts ...Option) *Component {
	t.Helper()

	base := []Option{
		WithConfig(&Config{
			Name:                   "test",
			KeyPrefix:              "test",
			JWTSignKey:             "test-secret",
			AccessTokenTTL:         time.Hour,
			AccessTokenDisplaySkew: time.Minute,
			RefreshTokenTTL:        24 * time.Hour,
			RevokedRecordTTL:       time.Hour,
			TouchInterval:          time.Hour,
			MultiLoginEnabled:      true,
			MaxSessions:            0,
			SameDeviceStrategy:     SameDeviceReplace,
			OverflowStrategy:       OverflowRevokeOldest,
		}),
		withRecordCache(newMemoryRecordCache()),
		withIndexStore(newMemoryIndexStore()),
		withIDGenerator(&sequenceIDGenerator{}),
	}
	base = append(base, opts...)
	return DefaultContainer().Build(base...)
}

type missingJetCache struct {
	err error
}

func (c missingJetCache) Set(context.Context, string, ...jetcache.ItemOption) error {
	return nil
}

func (c missingJetCache) Once(context.Context, string, ...jetcache.ItemOption) error {
	return nil
}

func (c missingJetCache) Delete(context.Context, string) error {
	return nil
}

func (c missingJetCache) DeleteFromLocalCache(string) {}

func (c missingJetCache) Exists(context.Context, string) bool {
	return false
}

func (c missingJetCache) Get(context.Context, string, any) error {
	return c.err
}

func (c missingJetCache) GetSkippingLocal(context.Context, string, any) error {
	return c.err
}

func (c missingJetCache) TaskSize() int {
	return 0
}

func (c missingJetCache) CacheType() string {
	return jetcache.TypeBoth
}

func (c missingJetCache) Close() {}

func TestJetRecordCache_GetMapsConfiguredNotFoundErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "jetcache miss", err: jetcache.ErrCacheMiss},
		{name: "configured gorm not found", err: gorm.ErrRecordNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newJetRecordCache(missingJetCache{err: tt.err})
			var session SessionRecord
			err := cache.Get(context.Background(), "auth:session:missing", &session)
			if !errors.Is(err, errRecordNotFound) {
				t.Fatalf("Get() error = %v, want %v", err, errRecordNotFound)
			}
		})
	}
}

func TestComponent_Issue_ReplacesSameDeviceAccessToken(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}
	second, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("second issue: %v", err)
	}

	if _, err = comp.ValidateAccessToken(ctx, first.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old access validation error = %v, want revoked or invalid", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, second.AccessToken); err != nil {
		t.Fatalf("new access validation: %v", err)
	}
}

func TestComponent_Issue_ReplacesSameDeviceWorkspaceAccessToken(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser", WorkspaceID: 10})
	if err != nil {
		t.Fatalf("first workspace issue: %v", err)
	}
	second, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser", WorkspaceID: 20})
	if err != nil {
		t.Fatalf("second workspace issue: %v", err)
	}

	if _, err = comp.ValidateAccessToken(ctx, first.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old workspace access validation error = %v, want revoked or invalid", err)
	}
	auth, err := comp.ValidateAccessToken(ctx, second.AccessToken)
	if err != nil {
		t.Fatalf("new workspace access validation: %v", err)
	}
	if auth.WorkspaceID != 20 {
		t.Fatalf("workspace id = %d, want 20", auth.WorkspaceID)
	}
}

func TestComponent_Refresh_RotatesRefreshTokenAndRejectsReuse(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	issued, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	refreshed, err := comp.Refresh(ctx, RefreshRequest{RefreshToken: issued.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if _, err = comp.ValidateAccessToken(ctx, issued.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old access validation error = %v, want revoked or invalid", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, refreshed.AccessToken); err != nil {
		t.Fatalf("refreshed access validation: %v", err)
	}
	if _, err = comp.Refresh(ctx, RefreshRequest{RefreshToken: issued.RefreshToken}); !errors.Is(err, ErrRefreshReused) {
		t.Fatalf("refresh reuse error = %v, want %v", err, ErrRefreshReused)
	}
	if _, err = comp.ValidateAccessToken(ctx, refreshed.AccessToken); !errors.Is(err, ErrSessionRevoked) && !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("access after reuse error = %v, want revoked session/token", err)
	}
}

func TestComponent_Issue_EnforcesMaxSessions(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t, WithConfig(&Config{
		Name:                   "test",
		KeyPrefix:              "test",
		JWTSignKey:             "test-secret",
		AccessTokenTTL:         time.Hour,
		AccessTokenDisplaySkew: time.Minute,
		RefreshTokenTTL:        24 * time.Hour,
		RevokedRecordTTL:       time.Hour,
		TouchInterval:          time.Hour,
		MultiLoginEnabled:      true,
		MaxSessions:            1,
		SameDeviceStrategy:     SameDeviceAllow,
		OverflowStrategy:       OverflowRevokeOldest,
	}))

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-1"})
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}
	second, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-2"})
	if err != nil {
		t.Fatalf("second issue: %v", err)
	}

	if _, err = comp.ValidateAccessToken(ctx, first.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("first access validation error = %v, want revoked or invalid", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, second.AccessToken); err != nil {
		t.Fatalf("second access validation: %v", err)
	}
}

func TestComponent_Issue_RejectsSameDeviceWhenConfigured(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t, WithConfig(&Config{
		Name:                   "test",
		KeyPrefix:              "test",
		JWTSignKey:             "test-secret",
		AccessTokenTTL:         time.Hour,
		AccessTokenDisplaySkew: time.Minute,
		RefreshTokenTTL:        24 * time.Hour,
		RevokedRecordTTL:       time.Hour,
		TouchInterval:          time.Hour,
		MultiLoginEnabled:      true,
		MaxSessions:            0,
		SameDeviceStrategy:     SameDeviceReject,
		OverflowStrategy:       OverflowRevokeOldest,
	}))

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}
	if _, err = comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"}); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("second issue error = %v, want %v", err, ErrSessionExists)
	}
	if _, err = comp.ValidateAccessToken(ctx, first.AccessToken); err != nil {
		t.Fatalf("first access should remain valid after rejected same-device login: %v", err)
	}
}

func TestComponent_Issue_RejectsOverflowWhenConfigured(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t, WithConfig(&Config{
		Name:                   "test",
		KeyPrefix:              "test",
		JWTSignKey:             "test-secret",
		AccessTokenTTL:         time.Hour,
		AccessTokenDisplaySkew: time.Minute,
		RefreshTokenTTL:        24 * time.Hour,
		RevokedRecordTTL:       time.Hour,
		TouchInterval:          time.Hour,
		MultiLoginEnabled:      true,
		MaxSessions:            1,
		SameDeviceStrategy:     SameDeviceAllow,
		OverflowStrategy:       OverflowReject,
	}))

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-1"})
	if err != nil {
		t.Fatalf("first issue: %v", err)
	}
	if _, err = comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-2"}); !errors.Is(err, ErrTooManySessions) {
		t.Fatalf("second issue error = %v, want %v", err, ErrTooManySessions)
	}
	if _, err = comp.ValidateAccessToken(ctx, first.AccessToken); err != nil {
		t.Fatalf("first access should remain valid after rejected overflow login: %v", err)
	}
}

func TestComponent_Logout_RevokesCurrentSession(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	issued, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	auth, err := comp.ValidateAccessToken(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := comp.Logout(ctx, auth); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, issued.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("validation after logout error = %v, want revoked", err)
	}
}

func TestComponent_RevokeSession_RemovesIndexesAndPreservesRevokedStatus(t *testing.T) {
	ctx := context.Background()
	store := newMemoryIndexStore()
	comp := newTestComponent(t, withIndexStore(store))

	issued, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	auth, err := comp.ValidateAccessToken(ctx, issued.AccessToken)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if err := comp.RevokeSession(ctx, auth.SessionID, StatusKicked); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	if sids, err := store.ListUserSessions(ctx, comp.keys.userSessions(auth.UserID)); err != nil || len(sids) != 0 {
		t.Fatalf("user session index after revoke = %#v, err = %v", sids, err)
	}
	if sid, err := store.GetDeviceSession(ctx, comp.keys.deviceSession(auth.UserID, deviceHash(auth.UA))); !errors.Is(err, errRecordNotFound) {
		t.Fatalf("device session index after revoke = %q, err = %v, want not found", sid, errRecordNotFound)
	}
	if _, err = comp.ValidateAccessToken(ctx, issued.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("validation after revoke error = %v, want revoked", err)
	} else if status, ok := statusFromError(err); !ok || status != StatusKicked {
		t.Fatalf("revoked status = %q, ok = %v, want %q", status, ok, StatusKicked)
	}
}

func TestComponent_RevokeUser_RevokesAllUserSessionsOnly(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	first, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-1"})
	if err != nil {
		t.Fatalf("issue first session: %v", err)
	}
	second, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-2"})
	if err != nil {
		t.Fatalf("issue second session: %v", err)
	}
	otherUser, err := comp.Issue(ctx, IssueRequest{UserID: 2, Username: "user2", UserType: 1, UA: "browser-3"})
	if err != nil {
		t.Fatalf("issue other user session: %v", err)
	}

	if err := comp.RevokeUser(ctx, 1, StatusKicked); err != nil {
		t.Fatalf("revoke user: %v", err)
	}

	for name, token := range map[string]string{"first": first.AccessToken, "second": second.AccessToken} {
		if _, err = comp.ValidateAccessToken(ctx, token); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrSessionRevoked) {
			t.Fatalf("%s user session validation error = %v, want revoked", name, err)
		}
	}
	if _, err = comp.ValidateAccessToken(ctx, otherUser.AccessToken); err != nil {
		t.Fatalf("other user should remain valid: %v", err)
	}
}

func TestToEcodeIncludesRevokedStatusMetadata(t *testing.T) {
	tests := []struct {
		name        string
		status      Status
		wantMessage string
	}{
		{
			name:        "kicked",
			status:      StatusKicked,
			wantMessage: "登录已被强制下线",
		},
		{
			name:        "replaced",
			status:      StatusReplaced,
			wantMessage: "登录已在其他设备生效",
		},
		{
			name:        "logout",
			status:      StatusLogout,
			wantMessage: "已退出登录",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := toEcode(context.Background(), revokedStatusError(ErrSessionRevoked, tt.status, ""))
			egoErr := eerrors.FromError(err)
			if egoErr.GetMessage() != tt.wantMessage {
				t.Fatalf("message = %q, want %q", egoErr.GetMessage(), tt.wantMessage)
			}
			if egoErr.GetMetadata()["auth_status"] != string(tt.status) {
				t.Fatalf("metadata auth_status = %q, want %q", egoErr.GetMetadata()["auth_status"], tt.status)
			}
		})
	}
}

func TestToEcodeMapsMissingSessionRecordToLoginExpired(t *testing.T) {
	err := toEcode(context.Background(), errRecordNotFound)
	egoErr := eerrors.FromError(err)
	wantReason := eerrors.FromError(ecode.ErrorLoginExpired()).GetReason()
	if egoErr.GetReason() != wantReason {
		t.Fatalf("reason = %q, want %q", egoErr.GetReason(), wantReason)
	}
	if egoErr.GetMessage() != "登录已过期" {
		t.Fatalf("message = %q, want 登录已过期", egoErr.GetMessage())
	}
}

func TestToEcodeUsesAcceptLanguage(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("accept-language", "en-US,en;q=0.9"))
	err := toEcode(ctx, errRecordNotFound)
	egoErr := eerrors.FromError(err)
	if egoErr.GetMessage() != "Login has expired" {
		t.Fatalf("message = %q, want Login has expired", egoErr.GetMessage())
	}
}

func TestIsSessionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "missing record", err: errRecordNotFound, want: true},
		{name: "refresh expired", err: ErrRefreshExpired, want: true},
		{name: "wrapped session revoked", err: fmt.Errorf("validate auth: %w", ErrSessionRevoked), want: true},
		{name: "other error", err: errors.New("other error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSessionError(tt.err); got != tt.want {
				t.Fatalf("IsSessionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComponent_RevokeUserWorkspace_RevokesOnlyTargetWorkspaceSessions(t *testing.T) {
	ctx := context.Background()
	comp := newTestComponent(t)

	target, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-1", WorkspaceID: 10})
	if err != nil {
		t.Fatalf("issue target workspace: %v", err)
	}
	otherWorkspace, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser-2", WorkspaceID: 20})
	if err != nil {
		t.Fatalf("issue other workspace: %v", err)
	}
	otherUser, err := comp.Issue(ctx, IssueRequest{UserID: 2, Username: "user2", UserType: 1, UA: "browser-3", WorkspaceID: 10})
	if err != nil {
		t.Fatalf("issue other user: %v", err)
	}

	if err := comp.RevokeUserWorkspace(ctx, 1, 10, StatusRevoked); err != nil {
		t.Fatalf("revoke user workspace: %v", err)
	}

	if _, err = comp.ValidateAccessToken(ctx, target.AccessToken); !errors.Is(err, ErrTokenRevoked) && !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("target workspace validation error = %v, want revoked", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, otherWorkspace.AccessToken); err != nil {
		t.Fatalf("other workspace should remain valid: %v", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, otherUser.AccessToken); err != nil {
		t.Fatalf("other user should remain valid: %v", err)
	}
}

func TestComponent_ValidateAccessToken_RejectsDisabledWorkspaceMember(t *testing.T) {
	ctx := context.Background()
	workspaceMemberEnabled := true
	comp := newTestComponent(t, WithContextValidatorFunc(func(ctx Context, auth *AuthContext) error {
		if auth.WorkspaceID != 0 && !workspaceMemberEnabled {
			return ErrSubjectDisabled
		}
		return nil
	}))

	issued, err := comp.Issue(ctx, IssueRequest{UserID: 1, Username: "admin", UserType: 1, UA: "browser", WorkspaceID: 10})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err = comp.ValidateAccessToken(ctx, issued.AccessToken); err != nil {
		t.Fatalf("validate enabled workspace member: %v", err)
	}

	workspaceMemberEnabled = false
	if _, err = comp.ValidateAccessToken(ctx, issued.AccessToken); !errors.Is(err, ErrSubjectInvalid) {
		t.Fatalf("validate disabled workspace member error = %v, want %v", err, ErrSubjectInvalid)
	}
}
