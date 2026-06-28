package asyncq

import (
	"github.com/google/wire"
)

// ProviderSet 用于依赖注入
var ProviderSet = wire.NewSet(
	NewAsyncqComponent,
)

// NewAsyncqComponent 供Wire使用的构造函数
// 默认从配置键 client.asyncq 加载
func NewAsyncqComponent() *Component {
	return Load("client.asyncq").Build()
}
