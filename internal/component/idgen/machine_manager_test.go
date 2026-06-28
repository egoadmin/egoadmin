package idgen

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMachineLeaseManager_StartLeaseAndStop(t *testing.T) {
	allocator := &fakeMachineAllocator{}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	lease, ok := manager.Lease()
	if !ok {
		t.Fatal("lease missing")
	}
	if lease.Namespace != "test" || lease.MachineID != 1 {
		t.Fatalf("lease = %+v, want namespace test machine 1", lease)
	}
	if err = manager.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	if err = manager.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if allocator.released != 1 {
		t.Fatalf("released = %d, want 1", allocator.released)
	}
}

func TestMachineLeaseManager_StopWithoutReleaseKeepsRemoteLease(t *testing.T) {
	allocator := &fakeMachineAllocator{}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err = manager.StopWithoutRelease(context.Background()); err != nil {
		t.Fatalf("stop without release: %v", err)
	}
	if allocator.released != 0 {
		t.Fatalf("released = %d, want 0", allocator.released)
	}
}

func TestMachineLeaseManager_StartFailureCanRetry(t *testing.T) {
	boom := errors.New("redis unavailable")
	allocator := &fakeMachineAllocator{allocateErr: boom}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("start err = %v, want boom", err)
	}
	allocator.allocateErr = nil
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("retry start: %v", err)
	}
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })
}

func TestMachineLeaseManager_RenewTransientFailureKeepsLease(t *testing.T) {
	boom := errors.New("temporary scheduler stall")
	allocator := &fakeMachineAllocator{renewErr: boom}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		RenewTimeout:  10 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })

	if err = manager.Renew(context.Background()); err != nil {
		t.Fatalf("renew transient failure: %v", err)
	}
	if _, ok := manager.Lease(); !ok {
		t.Fatal("lease should remain valid before local expiry")
	}
	if err = manager.Health(context.Background()); err != nil {
		t.Fatalf("health should remain healthy before local expiry: %v", err)
	}
}

func TestMachineLeaseManager_RenewLeaseLostReallocates(t *testing.T) {
	allocator := &fakeMachineAllocator{renewErr: ErrMachineLeaseLost}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		RenewTimeout:  10 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })

	first, ok := manager.Lease()
	if !ok {
		t.Fatal("lease missing")
	}
	if err = manager.Renew(context.Background()); err != nil {
		t.Fatalf("renew lease lost: %v", err)
	}
	second, ok := manager.Lease()
	if !ok {
		t.Fatal("lease missing after reallocate")
	}
	if second.SessionID == first.SessionID {
		t.Fatalf("session id = %q, want reallocated session", second.SessionID)
	}
	if allocator.allocations != 2 {
		t.Fatalf("allocations = %d, want 2", allocator.allocations)
	}
}

func TestMachineLeaseManager_RenewExpiredLeaseReallocates(t *testing.T) {
	allocator := &fakeMachineAllocator{leaseTTL: 20 * time.Millisecond}
	manager, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		RenewTimeout:  10 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, allocator, nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err = manager.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })

	time.Sleep(30 * time.Millisecond)
	if err = manager.Renew(context.Background()); err != nil {
		t.Fatalf("renew expired lease: %v", err)
	}
	if allocator.allocations != 2 {
		t.Fatalf("allocations = %d, want 2", allocator.allocations)
	}
}

func TestMachineLeaseManager_RequiresAllocator(t *testing.T) {
	_, err := NewMachineLeaseManager("machine", &MachineConfig{
		Group:         "test",
		MaxMachineID:  3,
		TTL:           time.Second,
		RenewInterval: 100 * time.Millisecond,
		LostPolicy:    LostPolicyFailClosed,
	}, nil, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("new manager err = %v, want ErrInvalidConfig", err)
	}
}

type fakeMachineAllocator struct {
	mu          sync.Mutex
	allocateErr error
	renewErr    error
	released    int
	allocations int
	leaseTTL    time.Duration
}

func (a *fakeMachineAllocator) Allocate(_ context.Context, req MachineRequest) (MachineLease, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.allocateErr != nil {
		return MachineLease{}, a.allocateErr
	}
	a.allocations++
	sessionID := "session-" + string(rune('0'+a.allocations))
	ttl := a.leaseTTL
	if ttl <= 0 {
		ttl = req.TTL
	}
	return MachineLease{
		Namespace:     req.Namespace,
		InstanceID:    req.InstanceID,
		SessionID:     sessionID,
		MachineID:     1,
		TTL:           req.TTL,
		RenewInterval: req.RenewInterval,
		ExpiresAt:     time.Now().Add(ttl),
	}, nil
}

func (a *fakeMachineAllocator) Renew(context.Context, MachineLease) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.renewErr
}

func (a *fakeMachineAllocator) Release(context.Context, MachineLease) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.released++
	return nil
}
