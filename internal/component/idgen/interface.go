package idgen

import (
	"context"
	"time"
)

// Interface is the business-facing ID generator contract.
type Interface interface {
	Next(ctx context.Context, name string) (int64, error)
	Reserve(ctx context.Context, name string, n int64) (Range, error)
	Generator(name string) (Generator, error)
	Stats(name string) (Stats, bool)
	Health(ctx context.Context) error
}

// Generator is a cached handle for high-frequency ID generation.
type Generator interface {
	Next(ctx context.Context) (int64, error)
	Reserve(ctx context.Context, n int64) (Range, error)
	Stats() Stats
}

// SegmentStore allocates non-overlapping ID ranges for a namespace/name pair.
type SegmentStore interface {
	Ensure(ctx context.Context, namespace string, name string, cfg EnsureSegmentConfig) error
	Fetch(ctx context.Context, namespace string, name string, step int64) (Range, SegmentConfig, error)
	Health(ctx context.Context) error
}

// MachineAllocator manages the process-level machine lease.
type MachineAllocator interface {
	Allocate(ctx context.Context, req MachineRequest) (MachineLease, error)
	Renew(ctx context.Context, lease MachineLease) error
	Release(ctx context.Context, lease MachineLease) error
}

// MachineLeaseManager owns the process-level machine lease lifecycle.
type MachineLeaseManager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Renew(ctx context.Context) error
	Lease() (MachineLease, bool)
	Health(ctx context.Context) error
}

// Range is a half-open ID interval: [Start, End).
type Range struct {
	Start int64
	End   int64
}

func (r Range) Len() int64 {
	if r.End <= r.Start {
		return 0
	}
	return r.End - r.Start
}

func (r Range) Empty() bool {
	return r.Len() == 0
}

// SegmentConfig is returned by the store with each allocated segment.
type SegmentConfig struct {
	Step    int64
	MinStep int64
	MaxStep int64
	Status  int
}

// EnsureSegmentConfig is used to create a missing segment definition.
type EnsureSegmentConfig struct {
	NextID      int64
	Step        int64
	MinStep     int64
	MaxStep     int64
	Status      int
	Description string
}

// MachineRequest describes the desired machine lease.
type MachineRequest struct {
	Namespace        string
	InstanceID       string
	MaxMachineID     int
	TTL              time.Duration
	RenewInterval    time.Duration
	StableInstanceID string
}

// MachineLease is the current process machine lease.
type MachineLease struct {
	Namespace     string
	InstanceID    string
	SessionID     string
	MachineID     int
	TTL           time.Duration
	RenewInterval time.Duration
	ExpiresAt     time.Time
}

// Stats is a point-in-time generator snapshot.
type Stats struct {
	Name              string
	Current           Range
	Next              Range
	CurrentRemaining  int64
	NextReady         bool
	Initialized       bool
	ThreadRunning     bool
	Step              int64
	MinStep           int64
	MaxStep           int64
	LastFetchAt       time.Time
	LastError         string
	Generated         uint64
	Prefetches        uint64
	SegmentFetches    uint64
	SegmentFetchFails uint64
}

const (
	SegmentStatusEnabled  = 1
	SegmentStatusDisabled = 2
)
