package job

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/google/wire"
	"github.com/gotomicro/ego/task/ecron"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(Options), "*"),
	New,
)

type Options struct {
	Upload *upload.Component
}

type Cron struct {
	tss []ecron.Ecron
}

func New(opts *Options) *Cron {
	cr := &Cron{
		tss: []ecron.Ecron{
			uploadCleanup(opts),
		},
	}
	return cr
}

func (c *Cron) Tasks() []ecron.Ecron {
	return c.tss
}

func uploadCleanup(opts *Options) ecron.Ecron {
	return ecron.Load("cron.gateway.upload.cleanup").Build(
		ecron.WithJob(func(ctx context.Context) error {
			_, err := opts.Upload.CleanupExpired(ctx, time.Now(), 100)
			return err
		}),
	)
}
