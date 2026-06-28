package logincrypto

import (
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewComponent,
)

func NewComponent(cache jetcache.Interface, keyStore KeyStore) *Component {
	return Load("component.logincrypto").Build(
		WithJetCache(cache),
		WithKeyStore(keyStore),
	)
}
