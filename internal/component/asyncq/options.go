package asyncq

import (
	"time"
)

// Option 选项
type Option func(c *Container)

// WithRedisAddr 设置Redis地址
func WithRedisAddr(addr string) Option {
	return func(c *Container) {
		c.config.RedisAddr = addr
	}
}

// WithRedisPassword 设置Redis密码
func WithRedisPassword(password string) Option {
	return func(c *Container) {
		c.config.RedisPassword = password
	}
}

// WithRedisDB 设置Redis数据库
func WithRedisDB(db int) Option {
	return func(c *Container) {
		c.config.RedisDB = db
	}
}

// WithConcurrency 设置并发数
func WithConcurrency(concurrency int) Option {
	return func(c *Container) {
		c.config.Concurrency = concurrency
	}
}

// WithQueues 设置队列配置
func WithQueues(queues map[string]int) Option {
	return func(c *Container) {
		c.config.Queues = queues
	}
}

// WithMaxRetry 设置最大重试次数
func WithMaxRetry(maxRetry int) Option {
	return func(c *Container) {
		c.config.MaxRetry = maxRetry
	}
}

// WithTaskTimeout 设置任务超时时间
func WithTaskTimeout(timeout time.Duration) Option {
	return func(c *Container) {
		c.config.TaskTimeout = timeout
	}
}

// WithSlowLogThreshold 设置慢日志阈值
func WithSlowLogThreshold(threshold time.Duration) Option {
	return func(c *Container) {
		c.config.SlowLogThreshold = threshold
	}
}

// WithEnableClient 设置是否启用客户端
func WithEnableClient(enable bool) Option {
	return func(c *Container) {
		c.config.EnableClient = enable
	}
}

// WithEnableServer 设置是否启用服务端
func WithEnableServer(enable bool) Option {
	return func(c *Container) {
		c.config.EnableServer = enable
	}
}

// WithEnableAccessLog 设置是否开启访问日志
func WithEnableAccessLog(enable bool) Option {
	return func(c *Container) {
		c.config.EnableAccessLog = enable
	}
}

// WithEnableHealthCheck 设置是否开启健康检查
func WithEnableHealthCheck(enable bool) Option {
	return func(c *Container) {
		c.config.EnableHealthCheck = enable
	}
}
