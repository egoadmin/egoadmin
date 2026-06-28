package asyncq

import (
	"time"

	"github.com/hibiken/asynq"
)

// Config asyncq配置
type Config struct {
	// Redis连接配置
	RedisAddr     string `json:"redisAddr" toml:"redisAddr"`         // Redis地址
	RedisPassword string `json:"redisPassword" toml:"redisPassword"` // Redis密码
	RedisDB       int    `json:"redisDB" toml:"redisDB"`             // Redis数据库

	// 客户端配置
	EnableClient bool `json:"enableClient" toml:"enableClient"` // 是否启用客户端

	// 服务端配置
	EnableServer bool `json:"enableServer" toml:"enableServer"` // 是否启用服务端
	Concurrency  int  `json:"concurrency" toml:"concurrency"`   // 并发数

	// 队列配置
	Queues map[string]int `json:"queues" toml:"queues"` // 队列优先级配置

	// 重试配置
	MaxRetry       int    `json:"maxRetry" toml:"maxRetry"`             // 最大重试次数
	RetryDelayFunc string `json:"retryDelayFunc" toml:"retryDelayFunc"` // 重试延迟函数类型

	// 超时配置
	TaskTimeout time.Duration `json:"taskTimeout" toml:"taskTimeout"` // 任务超时时间

	// 日志配置
	EnableAccessLog  bool          `json:"enableAccessLog" toml:"enableAccessLog"`   // 是否开启访问日志
	SlowLogThreshold time.Duration `json:"slowLogThreshold" toml:"slowLogThreshold"` // 慢日志阈值

	// 健康检查配置
	EnableHealthCheck bool `json:"enableHealthCheck" toml:"enableHealthCheck"` // 是否开启健康检查
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		RedisAddr:     "127.0.0.1:6379",
		RedisPassword: "",
		RedisDB:       0,

		EnableClient: true,
		EnableServer: false,
		Concurrency:  10,

		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},

		MaxRetry:       3,
		RetryDelayFunc: "exponential",
		TaskTimeout:    30 * time.Second,

		EnableAccessLog:   false,
		SlowLogThreshold:  time.Second,
		EnableHealthCheck: true,
	}
}

// RedisConnOpt 获取Redis连接选项
func (c *Config) RedisConnOpt() asynq.RedisConnOpt {
	return asynq.RedisClientOpt{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	}
}
