package meilisearch

import (
	"context"

	ms "github.com/meilisearch/meilisearch-go"
)

// Interface meilisearch组件接口
type Interface interface {
	// Health 调用 Meilisearch 健康接口
	Health(ctx context.Context) error
	// EnsureIndexes 确保索引存在（存在则跳过）
	EnsureIndexes(ctx context.Context, indexes []IndexConf) error
	// Client 返回客户端（meilisearch.ServiceManager）
	Client() ms.ServiceManager
	// Close 关闭客户端
	Close()
}
