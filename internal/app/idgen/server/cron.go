package server

import (
	"context"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/idgen/application"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/task/ecron"
)

const machineCleanupCronName = "cron.idgen.machine.cleanup"
const defaultMachineCleanupLimit = 1000
const defaultMachineCleanupRetention = 7 * 24 * time.Hour

func newMachineLeaseCleanupCron(conf *config.Config, usecase *application.MachineLeaseUseCase) ecron.Ecron {
	return ecron.Load(machineCleanupCronName).Build(
		ecron.WithJob(func(ctx context.Context) error {
			if conf == nil || usecase == nil {
				return nil
			}
			retention, limit, err := machineCleanupOptions(conf)
			if err != nil {
				return err
			}
			deleted, err := usecase.CleanupExpired(ctx, time.Now().Add(-retention), limit)
			if err != nil {
				return err
			}
			if deleted > 0 {
				elog.Info("idgen expired machine leases cleaned",
					elog.FieldComponent("task.ecron"),
					elog.FieldComponentName(machineCleanupCronName),
					elog.FieldCustomKeyValue("deleted", fmt.Sprint(deleted)),
				)
			}
			return nil
		}),
	)
}

func machineCleanupOptions(conf *config.Config) (time.Duration, int, error) {
	cfg := conf.IDGen()
	retention := defaultMachineCleanupRetention
	if cfg.MachineLeaseCleanupRetention != "" {
		parsed, err := time.ParseDuration(cfg.MachineLeaseCleanupRetention)
		if err != nil || parsed <= 0 {
			return 0, 0, fmt.Errorf("parse idgen machine lease cleanup retention: %w", err)
		}
		retention = parsed
	}
	limit := cfg.MachineLeaseCleanupLimit
	if limit <= 0 {
		limit = defaultMachineCleanupLimit
	}
	return retention, limit, nil
}
