package meilisearch

import "github.com/google/wire"

// ProviderSet 用于依赖注入
var ProviderSet = wire.NewSet(
	NewMeiliComponent,
)

// NewMeiliComponent 供Wire使用的构造函数
// 默认从配置键 client.meili 加载
func NewMeiliComponent() *Component {
	return Load("client.meili").Build()
}
