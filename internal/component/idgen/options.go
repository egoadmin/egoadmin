package idgen

import "github.com/gotomicro/ego/core/elog"

type Option func(*Container)

// WithName sets the configured component instance name when building without
// Load, such as in executable examples or focused tests.
func WithName(name string) Option {
	return func(c *Container) {
		if name != "" {
			c.name = name
			if c.logger != nil {
				c.logger = c.logger.With(elog.FieldComponentName(name))
			}
		}
	}
}

// WithConfig replaces the loaded config. Start from DefaultConfig when only
// overriding a few fields.
func WithConfig(config *Config) Option {
	return func(c *Container) {
		if config != nil {
			cp := *config
			c.config = &cp
		}
	}
}

func WithSegmentStore(store SegmentStore) Option {
	return func(c *Container) {
		c.store = store
	}
}

func WithMachineLeaseManager(manager MachineLeaseManager) Option {
	return func(c *Container) {
		c.machineManager = manager
	}
}

func WithLogger(logger *elog.Component) Option {
	return func(c *Container) {
		if logger != nil {
			c.logger = logger
		}
	}
}
