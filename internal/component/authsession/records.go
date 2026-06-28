package authsession

import "time"

type Status string

const (
	StatusActive        Status = "active"
	StatusLogout        Status = "logout"
	StatusExpired       Status = "expired"
	StatusKicked        Status = "kicked"
	StatusReplaced      Status = "replaced"
	StatusRevoked       Status = "revoked"
	StatusRotated       Status = "rotated"
	StatusRefreshReused Status = "refresh_reused"
)

type SessionRecord struct {
	ID                 string    `json:"id"`
	UserID             uint64    `json:"user_id"`
	Username           string    `json:"username"`
	UserType           int32     `json:"user_type"`
	UA                 string    `json:"ua"`
	DeviceHash         string    `json:"device_hash"`
	IP                 string    `json:"ip"`
	WorkspaceID        uint64    `json:"workspace_id"`
	CurrentAccessID    string    `json:"current_access_id"`
	CurrentRefreshHash string    `json:"current_refresh_hash"`
	Status             Status    `json:"status"`
	LoginAt            time.Time `json:"login_at"`
	LastActiveAt       time.Time `json:"last_active_at"`
	ExpiresAt          time.Time `json:"expires_at"`
	RevokedAt          time.Time `json:"revoked_at,omitempty"`
	RevokeReason       Status    `json:"revoke_reason,omitempty"`
}

type AccessRecord struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id"`
	UserID       uint64    `json:"user_id"`
	UA           string    `json:"ua"`
	TokenHash    string    `json:"token_hash"`
	Status       Status    `json:"status"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	RevokedAt    time.Time `json:"revoked_at,omitempty"`
	RevokeReason Status    `json:"revoke_reason,omitempty"`
}

type RefreshRecord struct {
	Hash          string    `json:"hash"`
	SessionID     string    `json:"session_id"`
	AccessID      string    `json:"access_id"`
	UserID        uint64    `json:"user_id"`
	Status        Status    `json:"status"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	RotatedAt     time.Time `json:"rotated_at,omitempty"`
	RevokedAt     time.Time `json:"revoked_at,omitempty"`
	RevokeReason  Status    `json:"revoke_reason,omitempty"`
	NextTokenHash string    `json:"next_token_hash,omitempty"`
}
