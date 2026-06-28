package jetcache

import (
	"context"

	jetcache "github.com/mgtv-tech/jetcache-go"
)

// Interface jetcache组件接口
type Interface interface {
	// Cache 返回jetcache实例
	Cache() jetcache.Cache
	// GetDelBytes 原子获取并删除远端缓存值.
	//
	// 该方法用于一次性令牌、challenge 等必须防重放的场景。返回 ok=false 表示键不存在。
	GetDelBytes(ctx context.Context, key string) (value []byte, ok bool, err error)
	// Health 健康检查
	Health(ctx context.Context) error
	// Close 关闭组件
	Close() error
}
