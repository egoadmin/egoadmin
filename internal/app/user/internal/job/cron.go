package job

import (
	"context"

	"github.com/google/wire"
	"github.com/gotomicro/ego/task/ecron"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(Options), "*"),
	New,
)

// Options cron配置
type Options struct {
	User UserOfflineService
}

type UserOfflineService interface {
	OfflineUser(context.Context) error
}

// Cron 定时任务功能
type Cron struct {
	tss []ecron.Ecron
}

// New 实例化
func New(opts *Options) *Cron {
	cr := &Cron{
		tss: []ecron.Ecron{},
	}

	cr.tss = append(cr.tss, userCrons(opts)...) // 用户相关定时任务

	return cr
}

// Tasks 返回定时任务列表
func (c *Cron) Tasks() []ecron.Ecron {
	return c.tss
}
