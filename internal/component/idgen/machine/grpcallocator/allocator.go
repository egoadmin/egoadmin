package grpcallocator

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/client/idgenclient"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

// Allocator coordinates machine leases through the idgen service.
type Allocator struct {
	client idgenclient.MachineLeaseService
}

func New(client idgenclient.MachineLeaseService) *Allocator {
	return &Allocator{client: client}
}

func (a *Allocator) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
	if a == nil || a.client == nil {
		return idgen.MachineLease{}, idgen.ErrStoreUnavailable
	}
	return a.client.Allocate(ctx, req)
}

func (a *Allocator) Renew(ctx context.Context, lease idgen.MachineLease) error {
	if a == nil || a.client == nil {
		return idgen.ErrStoreUnavailable
	}
	return a.client.Renew(ctx, lease)
}

func (a *Allocator) Release(ctx context.Context, lease idgen.MachineLease) error {
	if a == nil || a.client == nil {
		return idgen.ErrStoreUnavailable
	}
	return a.client.Release(ctx, lease)
}
