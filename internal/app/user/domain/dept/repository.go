package dept

import "context"

// Repository persists department aggregates.
type Repository interface {
	NextID(ctx context.Context) (uint64, error)
	Create(ctx context.Context, aggregate *Dept) error
	UpdateName(ctx context.Context, id uint64, name string) error
	UpdatePriorities(ctx context.Context, updates []PriorityUpdate) error
	DeleteByIDs(ctx context.Context, ids []uint64) error
	FindByID(ctx context.Context, id uint64) (*Dept, error)
	FindSubtree(ctx context.Context, id uint64) ([]*Dept, error)
	CountByParent(ctx context.Context, parentID uint64) (int64, error)
	FindByParentAndName(ctx context.Context, parentID uint64, name string) (*Dept, error)
}
