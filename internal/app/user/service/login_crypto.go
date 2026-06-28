package service

import (
	"context"
	"fmt"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const loginFailedMessage = "用户名或密码错误"

type LoginCryptoInterface interface {
	ChallengeFor(ctx context.Context, username string, ua string, action string) (logincrypto.Challenge, error)
	DecryptPayload(ctx context.Context, req logincrypto.DecryptRequest) (logincrypto.LoginPayload, error)
	Health(ctx context.Context) error
}

type authCryptoKeyStore struct {
	keys store.AuthCryptoKeyInterface
}

func (s authCryptoKeyStore) GetActive(ctx context.Context) (*logincrypto.KeyRecord, bool, error) {
	key, ok, err := s.keys.GetActive(ctx)
	if err != nil || !ok {
		return nil, ok, err
	}
	return authCryptoKeyToRecord(key), true, nil
}

func (s authCryptoKeyStore) GetByKeyID(ctx context.Context, keyID string) (*logincrypto.KeyRecord, bool, error) {
	key, ok, err := s.keys.GetByKeyID(ctx, keyID)
	if err != nil || !ok {
		return nil, ok, err
	}
	return authCryptoKeyToRecord(key), true, nil
}

func (s authCryptoKeyStore) Create(ctx context.Context, key *logincrypto.KeyRecord) error {
	return s.keys.Add(ctx, &store.AuthCryptoKeyModel{
		KeyID:         key.KeyID,
		Algorithm:     key.Algorithm,
		PublicKeyPEM:  key.PublicKeyPEM,
		PrivateKeyPEM: key.PrivateKeyPEM,
		Status:        store.AuthCryptoKeyModelStatusActive,
		Remark:        "系统自动生成登录加密密钥",
	})
}

func authCryptoKeyToRecord(key *store.AuthCryptoKeyModel) *logincrypto.KeyRecord {
	if key == nil {
		return nil
	}
	return &logincrypto.KeyRecord{
		KeyID:         key.KeyID,
		Algorithm:     key.Algorithm,
		PublicKeyPEM:  key.PublicKeyPEM,
		PrivateKeyPEM: key.PrivateKeyPEM,
	}
}

func NewLoginCrypto(cache *jetcache.Component, keys store.AuthCryptoKeyInterface) *logincrypto.Component {
	return logincrypto.Load("component.logincrypto").Build(
		logincrypto.WithJetCache(cache),
		logincrypto.WithKeyStore(authCryptoKeyStore{keys: keys}),
		logincrypto.WithKeyPrefix(defaults.RedisKeyPrefix),
	)
}

type LoginCryptoChallenge struct {
	KeyID       string
	PublicKey   string
	ChallengeID string
	Nonce       string
	Algorithm   string
	ExpiresAt   *timestamppb.Timestamp
}

func (s *UserService) GetLoginCrypto(ctx context.Context, username string, ua string, action string) (LoginCryptoChallenge, error) {
	challenge, err := s.LoginCrypto.ChallengeFor(ctx, username, ua, action)
	if err != nil {
		return LoginCryptoChallenge{}, err
	}
	return LoginCryptoChallenge{
		KeyID:       challenge.KeyID,
		PublicKey:   challenge.PublicKeyPEM,
		ChallengeID: challenge.ChallengeID,
		Nonce:       challenge.Nonce,
		Algorithm:   challenge.Algorithm,
		ExpiresAt:   timestamppb.New(challenge.ExpiresAt),
	}, nil
}

// WarmupLoginCrypto ensures the login transport key exists before the service
// is marked ready. Generating a 4096-bit RSA key on the first user request can
// exceed short upstream gRPC deadlines and cancel the initial database write.
func (s *UserService) WarmupLoginCrypto(ctx context.Context) error {
	if s.LoginCrypto == nil {
		return fmt.Errorf("warmup login crypto: component is nil")
	}
	if err := s.LoginCrypto.Health(ctx); err != nil {
		return fmt.Errorf("warmup login crypto: %w", err)
	}
	return nil
}

func (s *UserService) DecryptLoginPayload(ctx context.Context, req logincrypto.DecryptRequest) (logincrypto.LoginPayload, error) {
	payload, err := s.LoginCrypto.DecryptPayload(ctx, req)
	if err != nil {
		return logincrypto.LoginPayload{}, platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)
	}
	return payload, nil
}
