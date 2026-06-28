package jetcache

import (
	"time"
)

// Config jetcache配置
type Config struct {
	Name             string        `json:"name" toml:"name"`                         // 缓存名称
	RemoteExpiry     time.Duration `json:"remoteExpiry" toml:"remoteExpiry"`         // 远程缓存过期时间
	LocalSize        int           `json:"localSize" toml:"localSize"`               // 本地缓存大小（MB）
	LocalExpiry      time.Duration `json:"localExpiry" toml:"localExpiry"`           // 本地缓存过期时间
	RefreshDuration  time.Duration `json:"refreshDuration" toml:"refreshDuration"`   // 自动刷新间隔
	StopRefreshAfter time.Duration `json:"stopRefreshAfter" toml:"stopRefreshAfter"` // 停止刷新时间
	NotFoundExpiry   time.Duration `json:"notFoundExpiry" toml:"notFoundExpiry"`     // 未找到缓存过期时间
	EnableMetrics    bool          `json:"enableMetrics" toml:"enableMetrics"`       // 是否开启指标采集
	EnableSyncLocal  bool          `json:"enableSyncLocal" toml:"enableSyncLocal"`   // 是否开启本地缓存同步
	SourceId         string        `json:"sourceId" toml:"sourceId"`                 // 来源id
	Codec            string        `json:"codec" toml:"codec"`                       // 序列化方式：msgpack, json, sonic
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Name:             "default",
		RemoteExpiry:     time.Hour,
		LocalSize:        256,
		LocalExpiry:      time.Minute,
		RefreshDuration:  time.Minute * 2,
		StopRefreshAfter: time.Hour,
		NotFoundExpiry:   time.Minute,
		EnableMetrics:    true,
		EnableSyncLocal:  false,
		SourceId:         "",
		Codec:            "msgpack",
	}
}
