package jetcache

import (
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/google/wire"
)

// ProviderSet 组件集合
var ProviderSet = wire.NewSet(
	NewComponent,
)

// NewComponent 创建jetcache组件
func NewComponent(eredis *eredis.Component) *Component {
	return Load("client.jetcache").Build(WithEredis(eredis))
}
