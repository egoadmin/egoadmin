package redis

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/eredis/ecronlock"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	"github.com/google/wire"
)

// 本文件负责redis下面各种ProviderSet的注册
var CoreProviderSet = wire.NewSet(
	NewRedisComponent,
	NewLock,
	NewRedis,
)

var ProviderSet = wire.NewSet(
	CoreProviderSet,
)

// NewRedisComponent 初始化 redis 组件。
// 依赖 config.EgoReady 以保证在 ego 配置加载（econf 就绪）之后构造。
func NewRedisComponent(_ config.EgoReady) *eredis.Component {
	return eredis.Load("client.redis").Build()
}

// NewLock 初始化redis lock
func NewLock(cc *eredis.Component) *ecronlock.Component {
	return ecronlock.DefaultContainer().Build(
		ecronlock.WithClient(cc),
		ecronlock.WithPrefix(defaults.RedisKeyPrefix+"ecronlock:"),
	)
}

type Redis struct {
	cc *eredis.Component
}

func NewRedis(cc *eredis.Component) RedisInterface {
	return &Redis{
		cc: cc,
	}
}

func (s *Redis) Info(ctx context.Context) (string, error) {
	return s.cc.Client().Info(ctx).Result()
}
