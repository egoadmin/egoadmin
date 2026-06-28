package logincrypto

import (
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/gotomicro/ego/core/elog"
)

type Option func(*Container)

func WithConfig(config *Config) Option {
	return func(c *Container) {
		if config != nil {
			cp := *config
			c.config = &cp
		}
	}
}

func WithKeyPrefix(prefix string) Option {
	return func(c *Container) {
		c.config.KeyPrefix = prefix
	}
}

func WithJetCache(cache jetcache.Interface) Option {
	return func(c *Container) {
		c.jetcache = cache
	}
}

func WithKeyStore(store KeyStore) Option {
	return func(c *Container) {
		c.keyStore = store
	}
}

func WithChallengeStore(store challengeStore) Option {
	return func(c *Container) {
		c.store = store
	}
}

func WithLogger(logger *elog.Component) Option {
	return func(c *Container) {
		if logger != nil {
			c.logger = logger
		}
	}
}
