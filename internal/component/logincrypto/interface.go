package logincrypto

import (
	"context"
	"time"
)

type KeyRecord struct {
	KeyID         string
	Algorithm     string
	PublicKeyPEM  string
	PrivateKeyPEM string
}

type KeyStore interface {
	GetActive(ctx context.Context) (*KeyRecord, bool, error)
	GetByKeyID(ctx context.Context, keyID string) (*KeyRecord, bool, error)
	Create(ctx context.Context, key *KeyRecord) error
}

type ChallengeRecord struct {
	KeyID       string    `json:"keyId"`
	Username    string    `json:"username"`
	UA          string    `json:"ua"`
	Action      string    `json:"action"`
	Nonce       string    `json:"nonce"`
	ChallengeID string    `json:"challengeId"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type Challenge struct {
	KeyID        string
	PublicKeyPEM string
	ChallengeID  string
	Nonce        string
	Algorithm    string
	ExpiresAt    time.Time
}

type LoginPayload struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
	ChallengeID string `json:"challengeId"`
	Nonce       string `json:"nonce"`
	Timestamp   int64  `json:"timestamp"`
	UA          string `json:"ua"`
	Action      string `json:"action"`
}

type DecryptRequest struct {
	KeyID          string
	ChallengeID    string
	Username       string
	UA             string
	PasswordCipher string
	Action         string
}
