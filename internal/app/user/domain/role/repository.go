package role

import "context"

// Repository persists role aggregates.
type Repository interface {
	Create(ctx context.Context, aggregate *Role) error
	Update(ctx context.Context, id uint64, aggregate *Role) error
	Delete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*Role, error)
	FindByName(ctx context.Context, name string) (*Role, error)
}
