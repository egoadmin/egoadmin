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
	"fmt"
	"time"

	"github.com/gotomicro/ego/core/elog"
)

type Component struct {
	name     string
	config   *Config
	logger   *elog.Component
	store    challengeStore
	keyStore KeyStore
	keys     keyBuilder
}

func newComponent(name string, config *Config, logger *elog.Component, store challengeStore, keyStore KeyStore) (*Component, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	config.normalize()
	if store == nil {
		return nil, fmt.Errorf("%w: challenge store is nil", ErrInvalidConfig)
	}
	if keyStore == nil {
		return nil, fmt.Errorf("%w: key store is nil", ErrInvalidConfig)
	}
	return &Component{
		name:     name,
		config:   config,
		logger:   logger,
		store:    store,
		keyStore: keyStore,
		keys:     newKeyBuilder(config.KeyPrefix),
	}, nil
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) PackageName() string {
	return PackageName
}

func (c *Component) Init() error {
	return nil
}

func (c *Component) Start() error {
	return nil
}

func (c *Component) Stop() error {
	return nil
}

func (c *Component) Close() error {
	return c.Stop()
}

func (c *Component) Health(ctx context.Context) error {
	_, err := c.activeKey(ctx)
	return err
}

func (c *Component) Challenge(ctx context.Context, username string, ua string) (Challenge, error) {
	return c.ChallengeFor(ctx, username, ua, ActionLogin)
}

func (c *Component) ChallengeFor(ctx context.Context, username string, ua string, action string) (challenge Challenge, err error) {
	start := time.Now()
	defer func() { c.observe("challenge", start, err) }()

	action = normalizeAction(action)
	key, err := c.activeKey(ctx)
	if err != nil {
		return Challenge{}, err
	}
	challengeID, err := randomID(18)
	if err != nil {
		return Challenge{}, err
	}
	nonce, err := randomID(18)
	if err != nil {
		return Challenge{}, err
	}
	expiresAt := time.Now().Add(c.config.ChallengeTTL)
	record := ChallengeRecord{
		KeyID:       key.KeyID,
		Username:    username,
		UA:          ua,
		Action:      action,
		Nonce:       nonce,
		ChallengeID: challengeID,
		ExpiresAt:   expiresAt,
	}
	if err = c.store.Set(ctx, c.keys.challenge(challengeID), record, c.config.ChallengeTTL); err != nil {
		return Challenge{}, err
	}
	return Challenge{
		KeyID:        key.KeyID,
		PublicKeyPEM: key.PublicKeyPEM,
		ChallengeID:  challengeID,
		Nonce:        nonce,
		Algorithm:    AlgorithmRSAOAEP256,
		ExpiresAt:    expiresAt,
	}, nil
}

func (c *Component) DecryptPassword(ctx context.Context, req DecryptRequest) (string, error) {
	payload, err := c.DecryptPayload(ctx, req)
	if err != nil {
		return "", err
	}
	if payload.Password == "" {
		return "", ErrChallengeInvalid
	}
	return payload.Password, nil
}

func (c *Component) DecryptPayload(ctx context.Context, req DecryptRequest) (payload LoginPayload, err error) {
	start := time.Now()
	defer func() { c.observe("decrypt", start, err) }()

	if req.KeyID == "" || req.ChallengeID == "" || req.Username == "" || req.UA == "" || req.PasswordCipher == "" {
		return LoginPayload{}, ErrChallengeInvalid
	}
	req.Action = normalizeAction(req.Action)
	record, ok, err := c.store.Consume(ctx, c.keys.challenge(req.ChallengeID))
	if err != nil {
		return LoginPayload{}, err
	}
	if !ok ||
		record.KeyID != req.KeyID ||
		record.Username != req.Username ||
		record.UA != req.UA ||
		record.Action != req.Action ||
		record.ChallengeID != req.ChallengeID {
		return LoginPayload{}, ErrChallengeInvalid
	}
	now := time.Now()
	if record.ExpiresAt.IsZero() || now.After(record.ExpiresAt) {
		return LoginPayload{}, ErrChallengeInvalid
	}

	key, ok, err := c.keyStore.GetByKeyID(ctx, req.KeyID)
	if err != nil {
		return LoginPayload{}, err
	}
	if !ok || key.Algorithm != AlgorithmRSAOAEP256 {
		return LoginPayload{}, ErrKeyNotFound
	}
	privateKey, err := parsePrivateKey(key.PrivateKeyPEM)
	if err != nil {
		return LoginPayload{}, err
	}
	cipherText, err := base64.StdEncoding.DecodeString(req.PasswordCipher)
	if err != nil {
		if cipherText, err = base64.RawStdEncoding.DecodeString(req.PasswordCipher); err != nil {
			return LoginPayload{}, ErrCipherInvalid
		}
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, cipherText, nil)
	if err != nil {
		return LoginPayload{}, ErrCipherInvalid
	}
	if err = json.Unmarshal(plain, &payload); err != nil {
		return LoginPayload{}, ErrCipherInvalid
	}
	if payload.Username != req.Username ||
		payload.ChallengeID != req.ChallengeID ||
		payload.Nonce != record.Nonce ||
		payload.UA != req.UA ||
		normalizeAction(payload.Action) != req.Action {
		return LoginPayload{}, ErrChallengeInvalid
	}
	if payload.Timestamp <= 0 {
		return LoginPayload{}, ErrChallengeInvalid
	}
	sentAt := time.UnixMilli(payload.Timestamp)
	if sentAt.IsZero() || now.Sub(sentAt) > c.config.TimestampSkew || sentAt.Sub(now) > c.config.TimestampSkew {
		return LoginPayload{}, ErrChallengeInvalid
	}
	return payload, nil
}

func (c *Component) activeKey(ctx context.Context) (*KeyRecord, error) {
	key, ok, err := c.keyStore.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	if ok && key.Algorithm == AlgorithmRSAOAEP256 {
		return key, nil
	}
	generated, err := generateKey(c.config.RSAKeyBits)
	if err != nil {
		return nil, err
	}
	if err = c.keyStore.Create(ctx, generated); err != nil {
		return nil, err
	}
	if c.logger != nil {
		c.logger.Info("logincrypto generated active RSA key", elog.String("keyId", generated.KeyID))
	}
	return generated, nil
}

func generateKey(bits int) (*KeyRecord, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	keyID, err := randomID(16)
	if err != nil {
		return nil, err
	}
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER})
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal rsa public key: %w", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return &KeyRecord{
		KeyID:         keyID,
		Algorithm:     AlgorithmRSAOAEP256,
		PublicKeyPEM:  string(publicPEM),
		PrivateKeyPEM: string(privatePEM),
	}, nil
}

func parsePrivateKey(privatePEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, ErrKeyNotFound
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrKeyNotFound
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrKeyNotFound
	}
	return key, nil
}

func randomID(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeAction(action string) string {
	if action == "" {
		return ActionLogin
	}
	return action
}
