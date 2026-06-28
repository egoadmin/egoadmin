package authsession

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type IDGenerator interface {
	NewID() (string, error)
	NewOpaqueToken() (string, error)
}

type randomIDGenerator struct{}

func (randomIDGenerator) NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (randomIDGenerator) NewOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate opaque token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
