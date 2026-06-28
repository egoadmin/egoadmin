package authsession

import (
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewComponent,
)

func NewComponent(redis *eredis.Component, cache *jetcache.Component) *Component {
	return Load("component.authsession").Build(
		WithEredis(redis),
		WithJetCache(cache),
	)
}
