package segment

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

// Repository allocates non-overlapping ID ranges.
type Repository interface {
	Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error
	Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error)
	Health(ctx context.Context) error
}
