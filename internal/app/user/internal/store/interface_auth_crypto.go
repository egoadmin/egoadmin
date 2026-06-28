package store

import "context"

// AuthCryptoKeyInterface 登录加密密钥管理.
type AuthCryptoKeyInterface interface {
	// Add 新增登录加密密钥.
	Add(ctx context.Context, key *AuthCryptoKeyModel) error
	// GetActive 获取当前启用的登录加密密钥.
	GetActive(ctx context.Context) (*AuthCryptoKeyModel, bool, error)
	// GetByKeyID 通过密钥标识查询登录加密密钥.
	GetByKeyID(ctx context.Context, keyID string) (*AuthCryptoKeyModel, bool, error)
}
