package meilisearch

import (
	"time"
)

// Config meilisearch配置
type Config struct {
	Host            string        `json:"host" toml:"host"`                       // http(s)://localhost:7700
	APIKey          string        `json:"apiKey" toml:"apiKey"`                   // 管理或只读密钥
	Timeout         time.Duration `json:"timeout" toml:"timeout"`                 // 请求超时
	EnableHealth    bool          `json:"enableHealth" toml:"enableHealth"`       // 是否开启健康检查
	EnsureOnBuild   bool          `json:"ensureOnBuild" toml:"ensureOnBuild"`     // Build 时是否确保索引存在
	Indexes         []IndexConf   `json:"indexes" toml:"indexes"`                 // 索引配置（名称与主键）
	EnableAccessLog bool          `json:"enableAccessLog" toml:"enableAccessLog"` // 是否记录访问日志
	SlowLog         time.Duration `json:"slowLog" toml:"slowLog"`                 // 慢日志阈值
}

// IndexConf 索引配置
type IndexConf struct {
	Name       string `json:"name" toml:"name"`
	PrimaryKey string `json:"primaryKey" toml:"primaryKey"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:            "http://127.0.0.1:7700",
		APIKey:          "",
		Timeout:         5 * time.Second,
		EnableHealth:    true,
		EnsureOnBuild:   false,
		Indexes:         []IndexConf{},
		EnableAccessLog: false,
		SlowLog:         time.Second,
	}
}
