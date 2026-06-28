package logincrypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"sync"
	"testing"
	"time"
)

type memoryKeyStore struct {
	active *KeyRecord
}

func (s *memoryKeyStore) GetActive(ctx context.Context) (*KeyRecord, bool, error) {
	if s.active == nil {
		return nil, false, nil
	}
	return s.active, true, nil
}

func (s *memoryKeyStore) GetByKeyID(ctx context.Context, keyID string) (*KeyRecord, bool, error) {
	if s.active == nil || s.active.KeyID != keyID {
		return nil, false, nil
	}
	return s.active, true, nil
}

func (s *memoryKeyStore) Create(ctx context.Context, key *KeyRecord) error {
	s.active = key
	return nil
}

type memoryChallengeStore struct {
	mu    sync.Mutex
	items map[string]ChallengeRecord
}

func (s *memoryChallengeStore) Set(ctx context.Context, key string, value ChallengeRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = make(map[string]ChallengeRecord)
	}
	s.items[key] = value
	return nil
}

func (s *memoryChallengeStore) Consume(ctx context.Context, key string) (ChallengeRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.items[key]
	delete(s.items, key)
	return record, ok, nil
}

func TestComponent_DecryptPasswordConsumesChallenge(t *testing.T) {
	ctx := context.Background()
	store := &memoryChallengeStore{}
	keyStore := &memoryKeyStore{}
	comp, err := newComponent("test", DefaultConfig(), nil, store, keyStore)
	if err != nil {
		t.Fatal(err)
	}

	challenge, err := comp.Challenge(ctx, "admin", "browser")
	if err != nil {
		t.Fatal(err)
	}
	cipherText := encryptForTest(t, challenge.PublicKeyPEM, LoginPayload{
		Username:    "admin",
		Password:    "123456",
		ChallengeID: challenge.ChallengeID,
		Nonce:       challenge.Nonce,
		Timestamp:   time.Now().UnixMilli(),
		UA:          "browser",
	})
	password, err := comp.DecryptPassword(ctx, DecryptRequest{
		KeyID:          challenge.KeyID,
		ChallengeID:    challenge.ChallengeID,
		Username:       "admin",
		UA:             "browser",
		PasswordCipher: cipherText,
	})
	if err != nil {
		t.Fatal(err)
	}
	if password != "123456" {
		t.Fatalf("password = %q, want 123456", password)
	}
	_, err = comp.DecryptPassword(ctx, DecryptRequest{
		KeyID:          challenge.KeyID,
		ChallengeID:    challenge.ChallengeID,
		Username:       "admin",
		UA:             "browser",
		PasswordCipher: cipherText,
	})
	if err == nil {
		t.Fatal("expected consumed challenge to fail")
	}
}

func TestComponent_DecryptPayloadValidatesAction(t *testing.T) {
	ctx := context.Background()
	store := &memoryChallengeStore{}
	keyStore := &memoryKeyStore{}
	comp, err := newComponent("test", DefaultConfig(), nil, store, keyStore)
	if err != nil {
		t.Fatal(err)
	}

	challenge, err := comp.ChallengeFor(ctx, "admin", "browser", ActionCenterEditPassword)
	if err != nil {
		t.Fatal(err)
	}
	cipherText := encryptForTest(t, challenge.PublicKeyPEM, LoginPayload{
		Username:    "admin",
		OldPassword: "oldpass",
		NewPassword: "newpass",
		ChallengeID: challenge.ChallengeID,
		Nonce:       challenge.Nonce,
		Timestamp:   time.Now().UnixMilli(),
		UA:          "browser",
		Action:      ActionCenterEditPassword,
	})

	_, err = comp.DecryptPayload(ctx, DecryptRequest{
		KeyID:          challenge.KeyID,
		ChallengeID:    challenge.ChallengeID,
		Username:       "admin",
		UA:             "browser",
		PasswordCipher: cipherText,
		Action:         ActionLogin,
	})
	if err == nil {
		t.Fatal("expected action mismatch to fail")
	}
}

func encryptForTest(t *testing.T, publicPEM string, payload LoginPayload) string {
	t.Helper()
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil {
		t.Fatal("missing public pem")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		t.Fatal("not rsa public key")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	cipherText, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(cipherText)
}
