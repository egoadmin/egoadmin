package application

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/app/idgen/domain/segment"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

type SegmentUseCase struct {
	repo segment.Repository
}

func NewSegmentUseCase(repo segment.Repository) *SegmentUseCase {
	return &SegmentUseCase{repo: repo}
}

func (u *SegmentUseCase) Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error {
	return u.repo.Ensure(ctx, namespace, name, cfg)
}

func (u *SegmentUseCase) Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error) {
	return u.repo.Allocate(ctx, namespace, name, requestedStep)
}

func (u *SegmentUseCase) Health(ctx context.Context) error {
	return u.repo.Health(ctx)
}
