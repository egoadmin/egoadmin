package grpcallocator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

func TestAllocatorDelegatesLeaseOperations(t *testing.T) {
	t.Parallel()

	client := &fakeMachineLeaseClient{
		lease: idgen.MachineLease{
			Namespace:     "test",
			InstanceID:    "instance-a",
			SessionID:     "session",
			MachineID:     1,
			TTL:           time.Second,
			RenewInterval: time.Second / 3,
		},
	}
	allocator := New(client)
	lease, err := allocator.Allocate(context.Background(), idgen.MachineRequest{
		Namespace:     "test",
		InstanceID:    "instance-a",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: time.Second / 3,
	})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if lease != client.lease {
		t.Fatalf("lease = %+v, want %+v", lease, client.lease)
	}
	if err = allocator.Renew(context.Background(), lease); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err = allocator.Release(context.Background(), lease); err != nil {
		t.Fatalf("release: %v", err)
	}
	if client.allocateCalls != 1 || client.renewCalls != 1 || client.releaseCalls != 1 {
		t.Fatalf("calls allocate=%d renew=%d release=%d", client.allocateCalls, client.renewCalls, client.releaseCalls)
	}
}

func TestAllocatorUnavailableWithoutClient(t *testing.T) {
	t.Parallel()

	allocator := New(nil)
	if _, err := allocator.Allocate(context.Background(), idgen.MachineRequest{}); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("allocate err = %v, want ErrStoreUnavailable", err)
	}
	if err := allocator.Renew(context.Background(), idgen.MachineLease{}); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("renew err = %v, want ErrStoreUnavailable", err)
	}
	if err := allocator.Release(context.Background(), idgen.MachineLease{}); !errors.Is(err, idgen.ErrStoreUnavailable) {
		t.Fatalf("release err = %v, want ErrStoreUnavailable", err)
	}
}

type fakeMachineLeaseClient struct {
	lease         idgen.MachineLease
	allocateCalls int
	renewCalls    int
	releaseCalls  int
}

func (c *fakeMachineLeaseClient) Allocate(context.Context, idgen.MachineRequest) (idgen.MachineLease, error) {
	c.allocateCalls++
	return c.lease, nil
}

func (c *fakeMachineLeaseClient) Renew(context.Context, idgen.MachineLease) error {
	c.renewCalls++
	return nil
}

func (c *fakeMachineLeaseClient) Release(context.Context, idgen.MachineLease) error {
	c.releaseCalls++
	return nil
}
