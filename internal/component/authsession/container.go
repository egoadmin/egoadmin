package authsession

import (
	"errors"

	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

type Container struct {
	config      *Config
	name        string
	logger      *elog.Component
	redis       redisClient
	jetcache    jetcache.Interface
	recordCache recordCache
	indexStore  indexStore
	validator   ContextValidator
	recorder    EventRecorder
	idGenerator IDGenerator
}

func DefaultContainer() *Container {
	return &Container{
		config: DefaultConfig(),
		name:   "component.authsession.default",
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
	if c.config.JWTSignKey == "" {
		c.logger.Panic("authsession jwt sign key is empty", elog.FieldKey(c.name))
	}

	if c.recordCache == nil {
		if c.jetcache == nil {
			c.logger.Panic("authsession jetcache is nil", elog.FieldKey(c.name))
		}
		c.recordCache = newJetRecordCache(c.jetcache.Cache())
	}
	if c.indexStore == nil {
		if c.redis == nil {
			c.logger.Panic("authsession redis is nil", elog.FieldKey(c.name))
		}
		c.indexStore = newRedisIndexStore(c.redis.Client())
	}
	if c.idGenerator == nil {
		c.idGenerator = randomIDGenerator{}
	}

	comp, err := newComponent(c.name, c.config, c.logger, c.recordCache, c.indexStore, c.validator, c.recorder, c.idGenerator)
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			c.logger.Panic("authsession build failed", elog.FieldErr(err), elog.FieldKey(c.name))
		}
		c.logger.Panic("authsession build failed", elog.FieldErr(err), elog.FieldKey(c.name))
	}

	return comp
}
