package authsession

import (
	"context"
	"time"
)

type Interface interface {
	Issue(ctx context.Context, req IssueRequest) (*IssueResult, error)
	Refresh(ctx context.Context, req RefreshRequest) (*IssueResult, error)
	ValidateAccessToken(ctx context.Context, rawToken string) (*AuthContext, error)
	Logout(ctx context.Context, auth *AuthContext) error
	RevokeSession(ctx context.Context, sessionID string, reason Status) error
	RevokeUser(ctx context.Context, userID uint64, reason Status) error
	RevokeUserWorkspace(ctx context.Context, userID uint64, workspaceID uint64, reason Status) error
	ExpiresAt() time.Time
}
