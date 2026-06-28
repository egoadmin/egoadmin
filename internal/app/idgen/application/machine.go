package application

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/idgen/domain/machine"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

type MachineLeaseUseCase struct {
	repo machine.Repository
}

func NewMachineLeaseUseCase(repo machine.Repository) *MachineLeaseUseCase {
	return &MachineLeaseUseCase{repo: repo}
}

func (u *MachineLeaseUseCase) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
	return u.repo.Allocate(ctx, req)
}

func (u *MachineLeaseUseCase) Renew(ctx context.Context, lease idgen.MachineLease) error {
	return u.repo.Renew(ctx, lease)
}

func (u *MachineLeaseUseCase) Release(ctx context.Context, lease idgen.MachineLease) error {
	return u.repo.Release(ctx, lease)
}

func (u *MachineLeaseUseCase) CleanupExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	return u.repo.CleanupExpired(ctx, before, limit)
}
