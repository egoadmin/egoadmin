package mysql

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/app/idgen/domain/segment"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"github.com/egoadmin/egoadmin/internal/component/idgen/store/gormstore"
	"github.com/gotomicro/ego-component/egorm"
)

type SegmentRepository struct {
	store *gormstore.Store
}

var _ segment.Repository = (*SegmentRepository)(nil)

func NewSegmentRepository(db *egorm.Component) *SegmentRepository {
	return &SegmentRepository{store: gormstore.New(db)}
}

func (r *SegmentRepository) Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error {
	return r.store.Ensure(ctx, namespace, name, cfg)
}

func (r *SegmentRepository) Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error) {
	return r.store.Fetch(ctx, namespace, name, requestedStep)
}

func (r *SegmentRepository) Health(ctx context.Context) error {
	return r.store.Health(ctx)
}
