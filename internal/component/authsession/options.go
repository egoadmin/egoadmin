package authsession

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	compjetcache "github.com/egoadmin/egoadmin/internal/component/jetcache"
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

func WithEredis(redis *eredis.Component) Option {
	return func(c *Container) {
		c.redis = redis
	}
}

func WithJetCache(cache compjetcache.Interface) Option {
	return func(c *Container) {
		c.jetcache = cache
	}
}

func WithContextValidator(validator ContextValidator) Option {
	return func(c *Container) {
		c.validator = validator
	}
}

func WithContextValidatorFunc(fn func(ctx Context, auth *AuthContext) error) Option {
	return func(c *Container) {
		if fn != nil {
			c.validator = ContextValidatorFunc(func(ctx context.Context, auth *AuthContext) error {
				return fn(ctx, auth)
			})
		}
	}
}

func WithEventRecorder(recorder EventRecorder) Option {
	return func(c *Container) {
		c.recorder = recorder
	}
}

func WithLogger(logger *elog.Component) Option {
	return func(c *Container) {
		if logger != nil {
			c.logger = logger
		}
	}
}

type Context = context.Context
