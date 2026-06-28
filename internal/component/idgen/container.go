package idgen

import (
	"errors"

	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

// Container builds an idgen Component from EGO config plus options.
type Container struct {
	name           string
	config         *Config
	logger         *elog.Component
	store          SegmentStore
	machineManager MachineLeaseManager
}

func DefaultContainer() *Container {
	return &Container{
		name:   defaultComponentName,
		config: DefaultConfig(),
		logger: elog.EgoLogger.With(elog.FieldComponent(PackageName)),
	}
}

func Load(key string) *Container {
	c := DefaultContainer()
	c.name = key
	c.logger = c.logger.With(elog.FieldComponentName(key))
	if err := econf.UnmarshalKey(key, c.config); err != nil {
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
	comp, err := newComponent(c.name, c.config, c.logger, c.store, c.machineManager)
	if err != nil {
		if errors.Is(err, ErrInvalidConfig) {
			c.logger.Panic("idgen build failed", elog.FieldErr(err), elog.FieldKey(c.name))
		}
		c.logger.Panic("idgen build failed", elog.FieldErr(err), elog.FieldKey(c.name))
	}
	return comp
}
