package store

import "context"

// ConfigInterface 配置管理接口
type ConfigInterface interface {
	// Add 新增配置
	Add(ctx context.Context, conf *ConfigModel) (err error)
	// Update 修改配置
	Update(ctx context.Context, key string, value string) (err error)
	// Get 查询配置
	Get(ctx context.Context, key string) (config *ConfigModel, err error)
	// GetAll 获取所有配置
	GetAll(ctx context.Context) (configs []*ConfigModel, err error)
}
