package grpcstore

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/client/idgenclient"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

// Store allocates ID segments through the idgen service.
type Store struct {
	client idgenclient.SegmentService
}

func New(client idgenclient.SegmentService) *Store {
	return &Store{client: client}
}

func (s *Store) Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error {
	if s == nil || s.client == nil {
		return idgen.ErrStoreUnavailable
	}
	return s.client.Ensure(ctx, namespace, name, cfg)
}

func (s *Store) Fetch(ctx context.Context, namespace string, name string, step int64) (idgen.Range, idgen.SegmentConfig, error) {
	if s == nil || s.client == nil {
		return idgen.Range{}, idgen.SegmentConfig{}, idgen.ErrStoreUnavailable
	}
	return s.client.Allocate(ctx, namespace, name, step)
}

func (s *Store) Health(ctx context.Context) error {
	if s == nil || s.client == nil {
		return idgen.ErrStoreUnavailable
	}
	return s.client.Health(ctx)
}
