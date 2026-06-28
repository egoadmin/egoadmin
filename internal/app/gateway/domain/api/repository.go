package api

import "context"

// Repository persists gateway API catalog entries.
type Repository interface {
	CreateBatch(ctx context.Context, apis []*API) error
	Update(ctx context.Context, id uint64, api *API) error
	DeleteByIDs(ctx context.Context, ids []uint64) error
	FindAll(ctx context.Context) ([]*API, error)
	FindAllBySign(ctx context.Context, signs []string) ([]*API, error)
}
