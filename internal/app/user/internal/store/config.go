package store

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
)

const (
	// ConfigModelKeyUpload 文件上传配置key
	ConfigModelKeyUpload = "config_model_key_upload"
	// ConfigModelKeyUser 用户配置key
	ConfigModelKeyUser = "config_model_key_user"
)

// ConfigModel 系统配置
type ConfigModel struct {
	// Ckey 配置key
	Ckey string `gorm:"primarykey;type:varchar(255);not null;comment:键"`
	// Value 配置value
	Value string `gorm:"type:text;comment:值"`
}

// TableName 表名.
func (ConfigModel) TableName() string {
	return "config"
}

// Config 配置操作
type Config struct {
	cc *egorm.Component
}

// NewConfig 配置管理
func NewConfig(db *egorm.Component, id xorm.IDSetter) ConfigInterface {
	return &Config{
		cc: db,
	}
}

// ConfigUpload 文件上传配置
type ConfigUpload struct {
	// UploadSize 文件上传大小
	UploadSize uint64 `json:"uploadSize"`
	// UploadType 文件上传类型 文件后缀[.pdf|.txt|.md]
	UploadType string `json:"uploadType"`
}

// ConfigUser 用户配置
type ConfigUser struct {
	// UserLoginRetryTime 用户登录重试时间秒
	UserLoginRetryTime uint64 `json:"userLoginRetryTime"`
	// UserLoginRetryNum 用户登录重试次数
	UserLoginRetryNum int32 `json:"userLoginRetryNum"`
	// UserLoginLockTime 用户登录锁定时间秒
	UserLoginLockTime uint64 `json:"userLoginLockTime"`
}

// Add 新增配置
func (m *Config) Add(ctx context.Context, conf *ConfigModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Create(&conf).Error

	return
}

// Update 修改配置
func (m *Config) Update(ctx context.Context, key string, value string) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&ConfigModel{}).Select("value").Where("ckey = ?", key).Update("value", value).Error

	return
}

// Get 查询配置
func (m *Config) Get(ctx context.Context, key string) (config *ConfigModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("ckey = ?", key).First(&config).Error

	return
}

// GetAll 获取所有配置
func (m *Config) GetAll(ctx context.Context) (configs []*ConfigModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Find(&configs).Error

	return
}
