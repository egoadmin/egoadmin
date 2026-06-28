package idgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gotomicro/ego/core/elog"
)

// ProcessMachineLeaseManager manages one machine lease for the current process.
type ProcessMachineLeaseManager struct {
	name      string
	config    *MachineConfig
	logger    *elog.Component
	allocator MachineAllocator

	cancel context.CancelFunc

	lifecycleMu sync.Mutex
	renewMu     sync.Mutex
	started     atomic.Bool
	stopped     atomic.Bool
	leaseLost   atomic.Bool
	leaseValid  atomic.Bool
	renewFails  atomic.Int64

	leaseMu sync.RWMutex
	lease   MachineLease

	nextAllocateAt atomic.Int64
}

func NewMachineLeaseManager(name string, config *MachineConfig, allocator MachineAllocator, logger *elog.Component) (*ProcessMachineLeaseManager, error) {
	if name == "" {
		name = "component.idgen.machine"
	}
	if config == nil {
		config = DefaultMachineConfig()
	}
	config.normalize()
	if err := config.validate(); err != nil {
		return nil, err
	}
	if allocator == nil {
		return nil, fmt.Errorf("%w: machine allocator is nil", ErrInvalidConfig)
	}
	_, cancel := context.WithCancel(context.Background())
	return &ProcessMachineLeaseManager{
		name:      name,
		config:    config,
		logger:    logger,
		allocator: allocator,
		cancel:    cancel,
	}, nil
}

func (m *ProcessMachineLeaseManager) Start(ctx context.Context) error {
	if m == nil || m.config == nil {
		return fmt.Errorf("%w: machine manager is nil", ErrInvalidConfig)
	}
	if !m.started.CompareAndSwap(false, true) {
		return nil
	}
	if err := m.allocate(ctx); err != nil {
		m.started.Store(false)
		return err
	}
	return nil
}

func (m *ProcessMachineLeaseManager) Stop(ctx context.Context) error {
	return m.stop(ctx, true)
}

// StopWithoutRelease stops local lease renewal without calling the allocator
// release endpoint. It is intended for process shutdown paths where the remote
// lease service may already be stopping and the lease TTL is the fallback.
func (m *ProcessMachineLeaseManager) StopWithoutRelease(ctx context.Context) error {
	return m.stop(ctx, false)
}

func (m *ProcessMachineLeaseManager) stop(ctx context.Context, release bool) error {
	if m == nil {
		return nil
	}
	m.lifecycleMu.Lock()
	if m.stopped.Load() {
		m.lifecycleMu.Unlock()
		return nil
	}
	m.stopped.Store(true)
	m.cancel()
	m.lifecycleMu.Unlock()

	m.leaseMu.RLock()
	lease := m.lease
	m.leaseMu.RUnlock()
	if lease.InstanceID == "" {
		m.leaseValid.Store(false)
		return nil
	}
	defer m.leaseValid.Store(false)
	if !release {
		return nil
	}
	releaseCtx := ctx
	if releaseCtx == nil {
		releaseCtx = context.Background()
	}
	if _, ok := releaseCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		releaseCtx, cancel = context.WithTimeout(releaseCtx, m.config.TTL)
		defer cancel()
	}
	if err := m.allocator.Release(releaseCtx, lease); err != nil {
		return fmt.Errorf("release idgen machine lease: %w", err)
	}
	return nil
}

func (m *ProcessMachineLeaseManager) Lease() (MachineLease, bool) {
	if m == nil || m.config == nil {
		return MachineLease{}, false
	}
	m.leaseMu.RLock()
	defer m.leaseMu.RUnlock()
	if m.lease.InstanceID == "" || !m.leaseValid.Load() || m.leaseLost.Load() {
		return MachineLease{}, false
	}
	if !m.lease.ExpiresAt.IsZero() && !m.lease.ExpiresAt.After(time.Now()) {
		return MachineLease{}, false
	}
	return m.lease, true
}

func (m *ProcessMachineLeaseManager) Health(ctx context.Context) error {
	if m == nil || m.config == nil {
		return fmt.Errorf("%w: machine manager is nil", ErrInvalidConfig)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.stopped.Load() {
		return ErrComponentClosed
	}
	if m.leaseLost.Load() || !m.leaseValid.Load() {
		return ErrMachineLeaseLost
	}
	m.leaseMu.RLock()
	lease := m.lease
	m.leaseMu.RUnlock()
	if lease.InstanceID == "" {
		return ErrMachineLeaseLost
	}
	if !lease.ExpiresAt.IsZero() && !lease.ExpiresAt.After(time.Now()) {
		return ErrMachineLeaseLost
	}
	return nil
}

func (m *ProcessMachineLeaseManager) LostPolicy() LostPolicy {
	if m == nil || m.config == nil {
		return LostPolicyDegraded
	}
	return m.config.LostPolicy
}

func (m *ProcessMachineLeaseManager) LeaseLost() bool {
	if m == nil || m.config == nil {
		return false
	}
	return m.leaseLost.Load() || !m.leaseValid.Load()
}

func (m *ProcessMachineLeaseManager) allocate(ctx context.Context) error {
	req := MachineRequest{
		Namespace:        m.config.Group,
		InstanceID:       m.instanceID(),
		MaxMachineID:     m.config.MaxMachineID,
		TTL:              m.config.TTL,
		RenewInterval:    m.config.RenewInterval,
		StableInstanceID: m.config.StableInstanceID,
	}
	deadlineCtx, cancel := m.withTimeout(ctx, m.config.RenewTimeout)
	defer cancel()
	lease, err := m.allocator.Allocate(deadlineCtx, req)
	if err != nil {
		return fmt.Errorf("allocate idgen machine lease: %w", err)
	}
	m.leaseMu.Lock()
	m.lease = lease
	m.leaseMu.Unlock()
	m.leaseLost.Store(false)
	m.leaseValid.Store(true)
	m.renewFails.Store(0)
	m.nextAllocateAt.Store(0)
	observeMachineLeaseStatus(m.name, true)
	return nil
}

func (m *ProcessMachineLeaseManager) Renew(ctx context.Context) error {
	if m == nil || m.config == nil {
		return fmt.Errorf("%w: machine manager is nil", ErrInvalidConfig)
	}
	if m.stopped.Load() {
		return ErrComponentClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.renewMu.Lock()
	defer m.renewMu.Unlock()

	lease := m.currentLease()
	if lease.InstanceID == "" || m.shouldReallocate(lease) {
		return m.reallocate(ctx)
	}

	deadlineCtx, cancel := m.withTimeout(ctx, m.config.RenewTimeout)
	err := m.allocator.Renew(deadlineCtx, lease)
	cancel()
	observeMachineRenew(m.name, err)
	if err != nil {
		return m.handleRenewError(ctx, lease, err)
	}

	lease.ExpiresAt = time.Now().Add(lease.TTL)
	m.leaseMu.Lock()
	m.lease = lease
	m.leaseMu.Unlock()
	m.leaseLost.Store(false)
	m.leaseValid.Store(true)
	m.renewFails.Store(0)
	m.nextAllocateAt.Store(0)
	observeMachineLeaseStatus(m.name, true)
	return nil
}

func (m *ProcessMachineLeaseManager) currentLease() MachineLease {
	m.leaseMu.RLock()
	defer m.leaseMu.RUnlock()
	return m.lease
}

func (m *ProcessMachineLeaseManager) handleRenewError(ctx context.Context, lease MachineLease, err error) error {
	fails := m.renewFails.Add(1)
	if m.shouldKeepLeaseOnRenewError(lease, err) {
		if m.logger != nil {
			m.logger.Warn("idgen machine lease renew failed, keeping current lease",
				elog.FieldCustomKeyValue("failures", fmt.Sprint(fails)),
				elog.FieldCustomKeyValue("expiresAt", lease.ExpiresAt.Format(time.RFC3339Nano)),
				elog.FieldErr(err),
			)
		}
		m.leaseLost.Store(false)
		m.leaseValid.Store(true)
		observeMachineLeaseStatus(m.name, true)
		return nil
	}
	m.markLeaseLost(err)
	return m.reallocate(ctx)
}

func (m *ProcessMachineLeaseManager) shouldKeepLeaseOnRenewError(lease MachineLease, err error) bool {
	if errors.Is(err, ErrMachineLeaseLost) {
		return false
	}
	return lease.InstanceID != "" && lease.ExpiresAt.After(time.Now())
}

func (m *ProcessMachineLeaseManager) shouldReallocate(lease MachineLease) bool {
	if lease.InstanceID == "" {
		return true
	}
	return !lease.ExpiresAt.IsZero() && !lease.ExpiresAt.After(time.Now())
}

func (m *ProcessMachineLeaseManager) markLeaseLost(err error) {
	m.leaseLost.Store(true)
	m.leaseValid.Store(false)
	observeMachineLeaseStatus(m.name, false)
	if m.logger != nil {
		m.logger.Error("idgen machine lease lost", elog.FieldErr(err))
	}
}

func (m *ProcessMachineLeaseManager) reallocate(ctx context.Context) error {
	now := time.Now()
	if next := m.nextAllocateAt.Load(); next > 0 && now.Before(time.Unix(0, next)) {
		return ErrMachineLeaseLost
	}
	if err := m.allocate(ctx); err != nil {
		m.nextAllocateAt.Store(now.Add(m.config.ReallocateBackoff).UnixNano())
		m.leaseLost.Store(true)
		m.leaseValid.Store(false)
		observeMachineLeaseStatus(m.name, false)
		if m.logger != nil {
			m.logger.Error("idgen machine lease reallocate failed", elog.FieldErr(err))
		}
		return err
	}
	return nil
}

func (m *ProcessMachineLeaseManager) withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = DefaultMachineConfig().RenewTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func (m *ProcessMachineLeaseManager) instanceID() string {
	stable := m.config.StableInstanceID
	if stable != "" {
		return stable
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
