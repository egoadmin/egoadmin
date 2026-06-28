package grpcstore

import (
	"context"
	"errors"
	"testing"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

func TestStoreDelegatesSegmentOperations(t *testing.T) {
	t.Parallel()

	client := &fakeSegmentClient{
		allocated: idgen.Range{Start: 10, End: 20},
		cfg:       idgen.SegmentConfig{Step: 10, MinStep: 10, MaxStep: 100, Status: idgen.SegmentStatusEnabled},
	}
	store := New(client)

	if err := store.Ensure(context.Background(), "test", "order", idgen.EnsureSegmentConfig{Step: 10}); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	r, cfg, err := store.Fetch(context.Background(), "test", "order", 10)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if r != client.allocated || cfg != client.cfg {
		t.Fatalf("fetch = %+v %+v, want %+v %+v", r, cfg, client.allocated, client.cfg)
	}
	if err = store.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	if client.ensureCalls != 1 || client.allocateCalls != 1 || client.healthCalls != 1 {
		t.Fatalf("calls ensure=%d allocate=%d health=%d", client.ensureCalls, client.allocateCalls, client.healthCalls)
	}
}

func TestStoreUnavailableWithoutClient(t *testing.T) {
	t.Parallel()

	store := New(nil)
	if err := store.Ensure(context.Background(), "test", "order", idgen.EnsureSegmentConfig{}); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("ensure err = %v, want ErrStoreUnavailable", err)
	}
	if _, _, err := store.Fetch(context.Background(), "test", "order", 1); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("fetch err = %v, want ErrStoreUnavailable", err)
	}
	if err := store.Health(context.Background()); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("health err = %v, want ErrStoreUnavailable", err)
	}
}

type fakeSegmentClient struct {
	allocated     idgen.Range
	cfg           idgen.SegmentConfig
	ensureCalls   int
	allocateCalls int
	healthCalls   int
}

func (c *fakeSegmentClient) Ensure(context.Context, string, string, idgen.EnsureSegmentConfig) error {
	c.ensureCalls++
	return nil
}

func (c *fakeSegmentClient) Allocate(context.Context, string, string, int64) (idgen.Range, idgen.SegmentConfig, error) {
	c.allocateCalls++
	return c.allocated, c.cfg, nil
}

func (c *fakeSegmentClient) Health(context.Context) error {
	c.healthCalls++
	return nil
}
