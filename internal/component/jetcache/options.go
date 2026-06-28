package jetcache

import (
	"time"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
)

// Option 选项
type Option func(c *Container)

// WithName 设置缓存名称
func WithName(name string) Option {
	return func(c *Container) { c.config.Name = name }
}

// WithRemoteExpiry 设置远程缓存过期时间
func WithRemoteExpiry(expiry time.Duration) Option {
	return func(c *Container) { c.config.RemoteExpiry = expiry }
}

// WithLocalSize 设置本地缓存大小（MB）
func WithLocalSize(size int) Option {
	return func(c *Container) { c.config.LocalSize = size }
}

// WithLocalExpiry 设置本地缓存过期时间
func WithLocalExpiry(expiry time.Duration) Option {
	return func(c *Container) { c.config.LocalExpiry = expiry }
}

// WithRefreshDuration 设置自动刷新间隔
func WithRefreshDuration(duration time.Duration) Option {
	return func(c *Container) { c.config.RefreshDuration = duration }
}

// WithStopRefreshAfter 设置停止刷新时间
func WithStopRefreshAfter(duration time.Duration) Option {
	return func(c *Container) { c.config.StopRefreshAfter = duration }
}

// WithNotFoundExpiry 设置未找到缓存过期时间
func WithNotFoundExpiry(expiry time.Duration) Option {
	return func(c *Container) { c.config.NotFoundExpiry = expiry }
}

// WithEnableMetrics 设置是否开启指标采集
func WithEnableMetrics(enable bool) Option {
	return func(c *Container) { c.config.EnableMetrics = enable }
}

// WithEnableSyncLocal 设置是否开启本地缓存同步
func WithEnableSyncLocal(enable bool) Option {
	return func(c *Container) { c.config.EnableSyncLocal = enable }
}

// WithCodec 设置序列化方式
func WithCodec(codec string) Option {
	return func(c *Container) { c.config.Codec = codec }
}

// WithSourceId 设置来源id
func WithSourceId(sourceId string) Option {
	return func(c *Container) { c.config.SourceId = sourceId }
}

// WithEredis 设置eredis组件
func WithEredis(eredis *eredis.Component) Option {
	return func(c *Container) { c.eredis = eredis }
}
