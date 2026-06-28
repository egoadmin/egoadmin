package job

import (
	"context"

	"github.com/gotomicro/ego/task/ecron"
)

// userCrons 用户定时任务
func userCrons(opts *Options) (ecs []ecron.Ecron) {
	ecs = append(ecs, userOffline(opts))

	return
}

// userOffline 用户离线
func userOffline(opts *Options) ecron.Ecron {
	return ecron.Load("cron.user.login.offline").Build(
		ecron.WithJob(func(ctx context.Context) error {
			return opts.User.OfflineUser(ctx)
		}),
	)
}
