package machine

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

// Repository coordinates process-level machine leases.
type Repository interface {
	Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error)
	Renew(ctx context.Context, lease idgen.MachineLease) error
	Release(ctx context.Context, lease idgen.MachineLease) error
	CleanupExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}
