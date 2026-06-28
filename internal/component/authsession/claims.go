package authsession

import (
	"context"
	"strconv"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Claims is the access-token JWT payload. It intentionally stores identifiers
// only; roles, menus, and permissions stay server-side.
type Claims struct {
	UID         uint64 `json:"uid"`
	Username    string `json:"username"`
	UserType    int32  `json:"typ"`
	UA          string `json:"ua"`
	SessionID   string `json:"sid"`
	TokenID     string `json:"jti"`
	WorkspaceID uint64 `json:"workspace_id,omitempty"`
	jwtv5.RegisteredClaims
}

type AuthContext struct {
	UserID            uint64
	Username          string
	UserType          int32
	DeptID            uint64
	UA                string
	SessionID         string
	TokenID           string
	WorkspaceID       uint64
	WorkspaceMemberID uint64
	IsBuiltinAdmin    bool
	Subject           string
	IssuedAt          time.Time
	ExpiresAt         time.Time
}

func (a *AuthContext) Clone() *AuthContext {
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

func (a *AuthContext) UserIDString() string {
	if a == nil {
		return ""
	}
	return strconv.FormatUint(a.UserID, 10)
}

func (a *AuthContext) DeptIDString() string {
	if a == nil {
		return ""
	}
	return strconv.FormatUint(a.DeptID, 10)
}

type authContextKey struct{}

func NewContext(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

func FromContext(ctx context.Context) (*AuthContext, bool) {
	auth, ok := ctx.Value(authContextKey{}).(*AuthContext)
	return auth, ok && auth != nil
}

type ContextValidator interface {
	ValidateAuthContext(ctx context.Context, auth *AuthContext) error
}

type ContextValidatorFunc func(ctx context.Context, auth *AuthContext) error

func (fn ContextValidatorFunc) ValidateAuthContext(ctx context.Context, auth *AuthContext) error {
	return fn(ctx, auth)
}

type EventRecorder interface {
	RecordAuthEvent(ctx context.Context, event Event)
}

type EventRecorderFunc func(ctx context.Context, event Event)

func (fn EventRecorderFunc) RecordAuthEvent(ctx context.Context, event Event) {
	fn(ctx, event)
}

type Event struct {
	Type      string
	UserID    uint64
	Username  string
	SessionID string
	TokenID   string
	UA        string
	IP        string
	Reason    Status
	At        time.Time
}

type IssueRequest struct {
	UserID      uint64
	Username    string
	UserType    int32
	UA          string
	IP          string
	WorkspaceID uint64
}

type RefreshRequest struct {
	RefreshToken string
	IP           string
}

type IssueResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Auth         *AuthContext
}
