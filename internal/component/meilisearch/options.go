package meilisearch

import "time"

// Option 选项
type Option func(c *Container)

// WithHost 设置服务地址
func WithHost(host string) Option {
	return func(c *Container) { c.config.Host = host }
}

// WithAPIKey 设置API密钥
func WithAPIKey(key string) Option {
	return func(c *Container) { c.config.APIKey = key }
}

// WithTimeout 设置超时
func WithTimeout(timeout time.Duration) Option {
	return func(c *Container) { c.config.Timeout = timeout }
}

// WithIndexes 设置索引集合
func WithIndexes(indexes []IndexConf) Option {
	return func(c *Container) { c.config.Indexes = indexes }
}

// WithEnsureOnBuild 设置构建时是否确保索引
func WithEnsureOnBuild(enable bool) Option {
	return func(c *Container) { c.config.EnsureOnBuild = enable }
}

// WithEnableHealth 设置健康检查
func WithEnableHealth(enable bool) Option {
	return func(c *Container) { c.config.EnableHealth = enable }
}

// WithAccessLog 设置访问日志
func WithAccessLog(enable bool) Option {
	return func(c *Container) { c.config.EnableAccessLog = enable }
}

// WithSlowLogThreshold 设置慢日志阈值
func WithSlowLogThreshold(d time.Duration) Option {
	return func(c *Container) { c.config.SlowLog = d }
}
