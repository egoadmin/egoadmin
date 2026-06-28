package logincrypto

import (
	"errors"

	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

type Container struct {
	name     string
	config   *Config
	logger   *elog.Component
	jetcache jetcache.Interface
	store    challengeStore
	keyStore KeyStore
}

func DefaultContainer() *Container {
	return &Container{
		name:   "component.logincrypto.default",
		config: DefaultConfig(),
		logger: elog.EgoLogger.With(elog.FieldComponent(PackageName)),
	}
}

func Load(key string) *Container {
	c := DefaultContainer()
	c.name = key
	c.logger = c.logger.With(elog.FieldComponentName(key))
	if err := econf.UnmarshalKey(key, &c.config); err != nil {
		c.logger.Panic("parse config error", elog.FieldErr(err), elog.FieldKey(key))
		return c
	}
	return c
}

func (c *Container) Build(options ...Option) *Component {
	for _, option := range options {
		option(c)
	}
	c.config.normalize()
	if c.keyStore == nil {
		c.logger.Panic("logincrypto key store is nil", elog.FieldKey(c.name))
	}
	if c.store == nil {
		if c.jetcache == nil {
			c.logger.Panic("logincrypto jetcache is nil", elog.FieldKey(c.name))
		}
		c.store = newJetChallengeStore(c.jetcache)
	}

	comp, err := newComponent(c.name, c.config, c.logger, c.store, c.keyStore)
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			c.logger.Panic("logincrypto build failed", elog.FieldErr(err), elog.FieldKey(c.name))
		}
		c.logger.Panic("logincrypto build failed", elog.FieldErr(err), elog.FieldKey(c.name))
	}
	return comp
}
