package store

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
	"gorm.io/gorm"
)

const (
	// AuthCryptoKeyModelStatusActive 启用.
	AuthCryptoKeyModelStatusActive int32 = iota + 1
	// AuthCryptoKeyModelStatusRetired 已退役.
	AuthCryptoKeyModelStatusRetired
)

const (
	// AuthCryptoKeyAlgorithmRSAOAEP256 RSA-OAEP-SHA256.
	AuthCryptoKeyAlgorithmRSAOAEP256 = "RSA-OAEP-SHA256"
)

// AuthCryptoKeyModel 登录加密密钥.
type AuthCryptoKeyModel struct {
	xorm.Model
	KeyID         string `gorm:"uniqueIndex;type:varchar(64);not null;default:'';comment:密钥标识"`
	Algorithm     string `gorm:"type:varchar(64);not null;default:'';comment:算法"`
	PublicKeyPEM  string `gorm:"type:text;not null;comment:公钥PEM"`
	PrivateKeyPEM string `gorm:"type:text;not null;comment:私钥PEM"`
	Status        int32  `gorm:"index;type:int(10);not null;default:1;comment:状态,1启用,2退役"`
	Remark        string `gorm:"type:varchar(255);not null;default:'';comment:备注"`
}

// TableName 表名.
func (AuthCryptoKeyModel) TableName() string {
	return "auth_crypto_key"
}

// SetID id设置接口.
func (m *AuthCryptoKeyModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

// BeforeCreate 创建执行前钩子函数.
func (m *AuthCryptoKeyModel) BeforeCreate(tx *gorm.DB) error {
	return mysql.SetID(m)
}

// AuthCryptoKey 登录加密密钥管理.
type AuthCryptoKey struct {
	cc *egorm.Component
}

// NewAuthCryptoKey 实例化登录加密密钥管理.
func NewAuthCryptoKey(db *egorm.Component, id xorm.IDSetter) AuthCryptoKeyInterface {
	return &AuthCryptoKey{cc: db}
}

// Add 新增登录加密密钥.
func (m *AuthCryptoKey) Add(ctx context.Context, key *AuthCryptoKeyModel) error {
	db := mysql.DBWithContext(ctx, m.cc)
	return db.Create(key).Error
}

// GetActive 获取当前启用的登录加密密钥.
func (m *AuthCryptoKey) GetActive(ctx context.Context) (*AuthCryptoKeyModel, bool, error) {
	var key AuthCryptoKeyModel
	db := mysql.DBWithContext(ctx, m.cc)
	err := db.Where("status = ?", AuthCryptoKeyModelStatusActive).
		Order("created_at DESC").
		First(&key).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &key, true, nil
}

// GetByKeyID 通过密钥标识查询登录加密密钥.
func (m *AuthCryptoKey) GetByKeyID(ctx context.Context, keyID string) (*AuthCryptoKeyModel, bool, error) {
	var key AuthCryptoKeyModel
	db := mysql.DBWithContext(ctx, m.cc)
	err := db.Where("key_id = ? AND status = ?", keyID, AuthCryptoKeyModelStatusActive).
		First(&key).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &key, true, nil
}
