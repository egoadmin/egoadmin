package redis

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/eredis/ecronlock"
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

func NewRedisComponent() *eredis.Component {
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
