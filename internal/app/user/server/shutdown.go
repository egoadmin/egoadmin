package server

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

func configureShutdown(opts Options) {
	if opts.shutdown == nil {
		return
	}
	opts.shutdown.RegisterCloser("config", opts.conf)
	opts.shutdown.RegisterRegistry(opts.registry)
	opts.shutdown.RegisterDB("mysql", opts.db)
	if opts.redis != nil {
		opts.shutdown.RegisterCloser("redis", opts.redis)
	}
	if opts.jetcache != nil {
		opts.shutdown.RegisterCloser("jetcache", opts.jetcache)
	}
	if opts.idgenClient != nil {
		opts.shutdown.RegisterCloser("idgen-grpc-client", opts.idgenClient)
	}
	if opts.idm != nil {
		opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
			return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
		})
	}
	if opts.idgen != nil {
		opts.shutdown.RegisterCloser("idgen", opts.idgen)
	}
	opts.shutdown.Bind(opts.app)
}
