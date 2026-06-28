package idgen

import (
	"context"

	"github.com/gotomicro/ego/task/ecron"
)

const machineRenewCronName = "cron.idgen.machine.renew"

func NewMachineLeaseRenewCron(manager MachineLeaseManager) ecron.Ecron {
	return ecron.Load(machineRenewCronName).Build(
		ecron.WithJob(func(ctx context.Context) error {
			if manager == nil {
				return nil
			}
			return manager.Renew(ctx)
		}),
	)
}
