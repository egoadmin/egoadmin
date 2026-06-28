package etusupload

import (
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

// Load 从配置加载组件
func Load(key string) *Component {
	cfg := DefaultConfig()
	if err := econf.UnmarshalKey(key, cfg); err != nil {
		elog.Warn("unmarshal config failed", elog.String("key", key), elog.FieldErr(err))
	}
	return newComponent("etusupload", cfg, elog.EgoLogger)
}

// ProviderSet wire提供者集合
var ProviderSet = wire.NewSet(
	NewComponentProvider,
)

// NewComponentProvider wire提供者。
func NewComponentProvider() *Component {
	return Load(PackageName).Build()
}
