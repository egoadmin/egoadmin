package idgen

import (
	"context"
	"fmt"
	"sync"
)

// MemoryStore is a deterministic in-memory SegmentStore for tests and examples.
type MemoryStore struct {
	mu    sync.Mutex
	items map[string]*memorySegment
}

type memorySegment struct {
	nextID  int64
	step    int64
	minStep int64
	maxStep int64
	status  int
}

// NewMemoryStore returns an empty in-memory segment store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*memorySegment)}
}

// Seed creates or replaces a segment definition for tests and examples.
func (s *MemoryStore) Seed(namespace string, name string, nextID int64, step int64, minStep int64, maxStep int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = make(map[string]*memorySegment)
	}
	if minStep <= 0 {
		minStep = step
	}
	if maxStep <= 0 {
		maxStep = step
	}
	s.items[storeKey(namespace, name)] = &memorySegment{
		nextID:  nextID,
		step:    step,
		minStep: minStep,
		maxStep: maxStep,
		status:  SegmentStatusEnabled,
	}
}

func (s *MemoryStore) Ensure(ctx context.Context, namespace string, name string, cfg EnsureSegmentConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("%w: namespace and name are required", ErrInvalidConfig)
	}
	cfg = normalizeEnsureSegmentConfig(cfg)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = make(map[string]*memorySegment)
	}
	key := storeKey(namespace, name)
	if _, ok := s.items[key]; ok {
		return nil
	}
	s.items[key] = &memorySegment{
		nextID:  cfg.NextID,
		step:    cfg.Step,
		minStep: cfg.MinStep,
		maxStep: cfg.MaxStep,
		status:  cfg.Status,
	}
	return nil
}

func (s *MemoryStore) Fetch(ctx context.Context, namespace string, name string, step int64) (Range, SegmentConfig, error) {
	if err := ctx.Err(); err != nil {
		return Range{}, SegmentConfig{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[storeKey(namespace, name)]
	if !ok {
		return Range{}, SegmentConfig{}, ErrNameNotFound
	}
	if item.status != SegmentStatusEnabled {
		return Range{}, SegmentConfig{}, ErrNameDisabled
	}
	actualStep := step
	if actualStep <= 0 {
		actualStep = item.step
	}
	if actualStep < item.minStep {
		actualStep = item.minStep
	}
	if item.maxStep > 0 && actualStep > item.maxStep {
		actualStep = item.maxStep
	}
	end, err := checkedAdd(item.nextID, actualStep)
	if err != nil {
		return Range{}, SegmentConfig{}, err
	}
	r := Range{Start: item.nextID, End: end}
	item.nextID = end
	return r, SegmentConfig{
		Step:    item.step,
		MinStep: item.minStep,
		MaxStep: item.maxStep,
		Status:  item.status,
	}, nil
}

func (s *MemoryStore) Health(ctx context.Context) error {
	return ctx.Err()
}

func storeKey(namespace string, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
}

func normalizeEnsureSegmentConfig(cfg EnsureSegmentConfig) EnsureSegmentConfig {
	if cfg.NextID <= 0 {
		cfg.NextID = 1
	}
	if cfg.Step <= 0 {
		cfg.Step = DefaultConfig().Step
	}
	if cfg.MinStep <= 0 {
		cfg.MinStep = DefaultConfig().MinStep
	}
	if cfg.MaxStep <= 0 {
		cfg.MaxStep = DefaultConfig().MaxStep
	}
	if cfg.MinStep > cfg.Step {
		cfg.Step = cfg.MinStep
	}
	if cfg.MaxStep < cfg.Step {
		cfg.MaxStep = cfg.Step
	}
	if cfg.Status == 0 {
		cfg.Status = SegmentStatusEnabled
	}
	return cfg
}
