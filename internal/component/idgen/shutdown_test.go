package idgen

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStopMachineLeaseBestEffort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		manager MachineLeaseManager
		wantErr error
	}{
		{name: "nil manager"},
		{
			name:    "store unavailable is expected during shutdown",
			manager: &fakeMachineLeaseManager{stopErr: ErrStoreUnavailable},
		},
		{
			name:    "lease lost is expected during shutdown",
			manager: &fakeMachineLeaseManager{stopErr: ErrMachineLeaseLost},
		},
		{
			name:    "deadline is expected during shutdown",
			manager: &fakeMachineLeaseManager{stopErr: context.DeadlineExceeded},
		},
		{
			name:    "unexpected error is returned",
			manager: &fakeMachineLeaseManager{stopErr: errUnexpectedMachineLeaseStop},
			wantErr: errUnexpectedMachineLeaseStop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := StopMachineLeaseBestEffort(context.Background(), tt.manager, time.Millisecond)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("StopMachineLeaseBestEffort() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStopMachineLeaseBestEffortUsesStopWithoutRelease(t *testing.T) {
	t.Parallel()

	manager := &fakeMachineLeaseManagerWithStopWithoutRelease{}
	if err := StopMachineLeaseBestEffort(context.Background(), manager, time.Millisecond); err != nil {
		t.Fatalf("StopMachineLeaseBestEffort() error = %v", err)
	}
	if manager.stopCalls != 0 {
		t.Fatalf("Stop calls = %d, want 0", manager.stopCalls)
	}
	if manager.stopWithoutReleaseCalls != 1 {
		t.Fatalf("StopWithoutRelease calls = %d, want 1", manager.stopWithoutReleaseCalls)
	}
}

var errUnexpectedMachineLeaseStop = errors.New("unexpected stop error")

type fakeMachineLeaseManager struct {
	stopErr error
}

func (m *fakeMachineLeaseManager) Start(context.Context) error {
	return nil
}

func (m *fakeMachineLeaseManager) Stop(context.Context) error {
	return m.stopErr
}

func (m *fakeMachineLeaseManager) Renew(context.Context) error {
	return nil
}

func (m *fakeMachineLeaseManager) Lease() (MachineLease, bool) {
	return MachineLease{}, false
}

func (m *fakeMachineLeaseManager) Health(context.Context) error {
	return nil
}

type fakeMachineLeaseManagerWithStopWithoutRelease struct {
	fakeMachineLeaseManager
	stopCalls               int
	stopWithoutReleaseCalls int
}

func (m *fakeMachineLeaseManagerWithStopWithoutRelease) Stop(context.Context) error {
	m.stopCalls++
	return nil
}

func (m *fakeMachineLeaseManagerWithStopWithoutRelease) StopWithoutRelease(context.Context) error {
	m.stopWithoutReleaseCalls++
	return nil
}
