package idgen

import (
	"context"
	"errors"
	"time"

	"github.com/gotomicro/ego/core/elog"
)

// StopMachineLeaseBestEffort stops the process machine lease during shutdown.
// Current managers stop renewal without a remote release call; the lease TTL
// bounds reuse if the idgen service is already stopping.
func StopMachineLeaseBestEffort(ctx context.Context, manager MachineLeaseManager, fallbackTimeout time.Duration) error {
	if manager == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok && fallbackTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, fallbackTimeout)
		defer cancel()
	}
	var err error
	if stopper, ok := manager.(interface{ StopWithoutRelease(context.Context) error }); ok {
		err = stopper.StopWithoutRelease(ctx)
	} else {
		err = manager.Stop(ctx)
	}
	if err != nil {
		if isExpectedMachineLeaseShutdownError(err) {
			elog.Info("idgen machine lease stopped best-effort during shutdown", elog.FieldComponent("shutdown"), elog.FieldErr(err))
			return nil
		}
		return err
	}
	return nil
}

func isExpectedMachineLeaseShutdownError(err error) bool {
	return errors.Is(err, ErrStoreUnavailable) ||
		errors.Is(err, ErrMachineLeaseLost) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
