package idgen

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestComponent(t *testing.T, step int64) *Component {
	t.Helper()
	store := NewMemoryStore()
	store.Seed("test", "order", 1, step, step, step*100)
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            false,
		MaxPrefetchWorkers:     2,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})
	return comp
}

func TestComponent_NextSequential(t *testing.T) {
	comp := newTestComponent(t, 10)
	ctx := context.Background()
	for want := int64(1); want <= 25; want++ {
		got, err := comp.Next(ctx, "order")
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if got != want {
			t.Fatalf("id = %d, want %d", got, want)
		}
	}
}

func TestComponent_NextConcurrentUnique(t *testing.T) {
	comp := newTestComponent(t, 64)
	ctx := context.Background()
	const workers = 16
	const perWorker = 1000

	ids := make(chan int64, workers*perWorker)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				id, err := comp.Next(ctx, "order")
				if err != nil {
					t.Errorf("next: %v", err)
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int64]struct{}, workers*perWorker)
	for id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %d", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("ids = %d, want %d", len(seen), workers*perWorker)
	}
}

func TestComponent_MultipleNamesConcurrent(t *testing.T) {
	store := NewMemoryStore()
	store.Seed("test", "order", 1, 16, 16, 16)
	store.Seed("test", "invoice", 1_000_001, 16, 16, 16)
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            false,
		MaxPrefetchWorkers:     2,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err = comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})

	ctx := context.Background()
	names := []string{"order", "invoice"}
	const perName = 512
	type result struct {
		name string
		id   int64
	}
	results := make(chan result, len(names)*perName)
	var wg sync.WaitGroup
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perName; i++ {
				id, er := comp.Next(ctx, name)
				if er != nil {
					t.Errorf("%s next: %v", name, er)
					return
				}
				results <- result{name: name, id: id}
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := map[string]map[int64]struct{}{
		"order":   {},
		"invoice": {},
	}
	for item := range results {
		if _, ok := seen[item.name][item.id]; ok {
			t.Fatalf("%s duplicate id %d", item.name, item.id)
		}
		seen[item.name][item.id] = struct{}{}
	}
	for _, name := range names {
		if got := len(seen[name]); got != perName {
			t.Fatalf("%s ids = %d, want %d", name, got, perName)
		}
	}
}

func TestComponent_UsesPrefetchedSegmentAfterWaitTimeout(t *testing.T) {
	store := newBlockingStore([]Range{
		{Start: 1, End: 3},
		{Start: 3, End: 5},
		{Start: 5, End: 7},
	})
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		FetchTimeout:           time.Second,
		WaitTimeout:            50 * time.Millisecond,
		PrefetchRemainingRatio: 0.5,
		DynamicStep:            false,
		MaxPrefetchWorkers:     1,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err = comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})

	ctx := context.Background()
	if id, er := comp.Next(ctx, "order"); er != nil || id != 1 {
		t.Fatalf("first id = %d, err = %v, want id 1", id, er)
	}
	if id, er := comp.Next(ctx, "order"); er != nil || id != 2 {
		t.Fatalf("second id = %d, err = %v, want id 2", id, er)
	}
	store.waitFetchCount(t, 2)

	done := make(chan struct{})
	go func() {
		defer close(done)
		id, er := comp.Next(ctx, "order")
		if er != nil {
			t.Errorf("third next: %v", er)
			return
		}
		if id != 3 {
			t.Errorf("third id = %d, want 3", id)
		}
	}()

	time.Sleep(5 * time.Millisecond)
	store.releaseFetch()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("third next did not return")
	}
	if got := store.fetches.Load(); got != 2 {
		t.Fatalf("fetches = %d, want 2; synchronous fallback fetched an extra segment", got)
	}
}

func TestComponent_Reserve(t *testing.T) {
	comp := newTestComponent(t, 10)
	ctx := context.Background()
	r, err := comp.Reserve(ctx, "order", 4)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if r.Start != 1 || r.End != 5 {
		t.Fatalf("range = [%d,%d), want [1,5)", r.Start, r.End)
	}
	id, err := comp.Next(ctx, "order")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if id != 5 {
		t.Fatalf("id = %d, want 5", id)
	}
}

func TestComponent_NameNotFound(t *testing.T) {
	comp := newTestComponent(t, 10)
	_, err := comp.Next(context.Background(), "missing")
	if !errors.Is(err, ErrNameNotFound) {
		t.Fatalf("err = %v, want ErrNameNotFound", err)
	}
}

func TestComponent_StartWarmsConfiguredNames(t *testing.T) {
	store := NewMemoryStore()
	store.Seed("test", "order", 1, 10, 10, 100)
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		Name:                   "order",
		Step:                   10,
		MinStep:                10,
		MaxStep:                100,
		AutoEnsure:             true,
		Warmup:                 true,
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		MaxPrefetchWorkers:     1,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err = comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})
	if err = comp.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	stats, ok := comp.Stats("order")
	if !ok {
		t.Fatal("missing warmed generator")
	}
	if !stats.Initialized || stats.Current.Len() != 10 {
		t.Fatalf("stats = %+v, want initialized first segment", stats)
	}
}

func TestComponent_StartAutoEnsuresDefaultSegment(t *testing.T) {
	store := NewMemoryStore()
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		Name:                   "order",
		Step:                   10,
		MinStep:                10,
		MaxStep:                100,
		AutoEnsure:             true,
		Warmup:                 true,
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		MaxPrefetchWorkers:     1,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err = comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})
	if err = comp.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	id, err := comp.NextDefault(context.Background())
	if err != nil {
		t.Fatalf("next default: %v", err)
	}
	if id != 1 {
		t.Fatalf("id = %d, want 1", id)
	}
}

func TestContainerDefaultConfigKeepsAutoEnsureAndWarmup(t *testing.T) {
	store := NewMemoryStore()
	cfg := DefaultConfig()
	cfg.Namespace = "test"
	cfg.Name = "order"
	cfg.EnableMetrics = false
	comp := DefaultContainer().Build(
		WithConfig(cfg),
		WithSegmentStore(store),
		WithMachineLeaseManager(healthyMachineManager{}),
	)
	t.Cleanup(func() {
		if err := comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})
	if !comp.config.AutoEnsure || !comp.config.Warmup {
		t.Fatalf("autoEnsure/warmup = %v/%v, want defaults true/true", comp.config.AutoEnsure, comp.config.Warmup)
	}
	if err := comp.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	id, err := comp.NextDefault(context.Background())
	if err != nil {
		t.Fatalf("next default: %v", err)
	}
	if id != 1 {
		t.Fatalf("id = %d, want 1", id)
	}
}

func TestComponent_FailClosedLeaseLostRejectsNext(t *testing.T) {
	manager := &fakeMachineManager{err: ErrMachineLeaseLost, policy: LostPolicyFailClosed}
	store := NewMemoryStore()
	store.Seed("test", "order", 1, 10, 10, 100)
	comp, err := newComponent("test", &Config{
		Namespace:              "test",
		Name:                   "order",
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            false,
		MaxPrefetchWorkers:     2,
		EnableMetrics:          false,
	}, nil, store, manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err = comp.Stop(); err != nil {
			t.Fatalf("stop component: %v", err)
		}
	})

	_, err = comp.Next(context.Background(), "order")
	if !errors.Is(err, ErrMachineLeaseLost) {
		t.Fatalf("next err = %v, want ErrMachineLeaseLost", err)
	}
}

func TestComponent_MultipleComponentsShareMachineManager(t *testing.T) {
	manager := &fakeMachineManager{policy: LostPolicyFailClosed}
	store := NewMemoryStore()
	store.Seed("test", "default", 1, 10, 10, 100)
	store.Seed("test", "order", 1_000, 10, 10, 100)
	cfg := Config{
		Namespace:              "test",
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            false,
		MaxPrefetchWorkers:     2,
		EnableMetrics:          false,
	}
	defaultCfg := cfg
	defaultCfg.Name = "default"
	orderCfg := cfg
	orderCfg.Name = "order"
	defaultComp, err := newComponent("default", &defaultCfg, nil, store, manager)
	if err != nil {
		t.Fatal(err)
	}
	orderComp, err := newComponent("order", &orderCfg, nil, store, manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = defaultComp.Stop()
		_ = orderComp.Stop()
	})
	if _, err = defaultComp.NextDefault(context.Background()); err != nil {
		t.Fatalf("default next: %v", err)
	}
	if _, err = orderComp.NextDefault(context.Background()); err != nil {
		t.Fatalf("order next: %v", err)
	}
	if got := manager.lostCalls.Load(); got == 0 {
		t.Fatal("manager lease state was not checked")
	}
}

func TestComponent_InitValidatesConfig(t *testing.T) {
	comp := newTestComponent(t, 10)
	if err := comp.Init(); err != nil {
		t.Fatalf("init valid component: %v", err)
	}

	comp.config = nil
	if err := comp.Init(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("init err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewComponentRequiresStore(t *testing.T) {
	_, err := newComponent("test", DefaultConfig(), nil, nil, healthyMachineManager{})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func TestNewComponentRequiresMachineManager(t *testing.T) {
	_, err := newComponent("test", DefaultConfig(), nil, NewMemoryStore(), nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("err = %v, want ErrInvalidConfig", err)
	}
}

func BenchmarkNext(b *testing.B) {
	comp := newBenchComponent(b, 1_000_000)
	g, err := comp.Generator("order")
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err = g.Next(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNextParallel(b *testing.B) {
	comp := newBenchComponent(b, 10_000_000)
	g, err := comp.Generator("order")
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := g.Next(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkReserve(b *testing.B) {
	comp := newBenchComponent(b, 10_000_000)
	g, err := comp.Generator("order")
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err = g.Reserve(ctx, 100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSegmentSwitch(b *testing.B) {
	comp := newBenchComponent(b, 32)
	g, err := comp.Generator("order")
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err = g.Next(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNextLatencyPercentiles(b *testing.B) {
	comp := newBenchComponent(b, 1_000_000)
	g, err := comp.Generator("order")
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	samples := make([]int64, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		begin := time.Now()
		if _, err = g.Next(ctx); err != nil {
			b.Fatal(err)
		}
		samples = append(samples, time.Since(begin).Nanoseconds())
	}
	b.StopTimer()

	slices.Sort(samples)
	b.ReportMetric(float64(percentile(samples, 0.50)), "p50-ns")
	b.ReportMetric(float64(percentile(samples, 0.90)), "p90-ns")
	b.ReportMetric(float64(percentile(samples, 0.99)), "p99-ns")
}

func percentile(samples []int64, rank float64) int64 {
	if len(samples) == 0 {
		return 0
	}
	if rank <= 0 {
		return samples[0]
	}
	if rank >= 1 {
		return samples[len(samples)-1]
	}
	index := int(float64(len(samples)-1) * rank)
	return samples[index]
}

func newBenchComponent(b *testing.B, step int64) *Component {
	b.Helper()
	store := NewMemoryStore()
	store.Seed("bench", "order", 1, step, step, step)
	comp, err := newComponent("bench", &Config{
		Namespace:              "bench",
		FetchTimeout:           time.Second,
		WaitTimeout:            time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            false,
		MaxPrefetchWorkers:     4,
		EnableMetrics:          false,
	}, nil, store, healthyMachineManager{})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := comp.Stop(); err != nil {
			b.Fatalf("stop component: %v", err)
		}
	})
	return comp
}

type fakeMachineManager struct {
	err         error
	policy      LostPolicy
	healthCalls atomic.Int64
	lostCalls   atomic.Int64
}

func (m *fakeMachineManager) Start(context.Context) error {
	return nil
}

func (m *fakeMachineManager) Stop(context.Context) error {
	return nil
}

func (m *fakeMachineManager) Renew(context.Context) error {
	return m.err
}

func (m *fakeMachineManager) Lease() (MachineLease, bool) {
	if m.err != nil {
		return MachineLease{}, false
	}
	return MachineLease{
		Namespace:  "test",
		InstanceID: "instance",
		SessionID:  "session",
		MachineID:  1,
		TTL:        time.Minute,
		ExpiresAt:  time.Now().Add(time.Minute),
	}, true
}

func (m *fakeMachineManager) Health(context.Context) error {
	m.healthCalls.Add(1)
	return m.err
}

func (m *fakeMachineManager) LostPolicy() LostPolicy {
	if m.policy == "" {
		return LostPolicyDegraded
	}
	return m.policy
}

func (m *fakeMachineManager) LeaseLost() bool {
	m.lostCalls.Add(1)
	return m.err != nil
}

type healthyMachineManager struct{}

func (healthyMachineManager) Start(context.Context) error {
	return nil
}

func (healthyMachineManager) Stop(context.Context) error {
	return nil
}

func (healthyMachineManager) Renew(context.Context) error {
	return nil
}

func (healthyMachineManager) Lease() (MachineLease, bool) {
	return MachineLease{
		Namespace:  "test",
		InstanceID: "instance",
		SessionID:  "session",
		MachineID:  1,
		TTL:        time.Minute,
		ExpiresAt:  time.Now().Add(time.Minute),
	}, true
}

func (healthyMachineManager) Health(context.Context) error {
	return nil
}

func (healthyMachineManager) LostPolicy() LostPolicy {
	return LostPolicyFailClosed
}

func (healthyMachineManager) LeaseLost() bool {
	return false
}

type blockingStore struct {
	ranges []Range

	mu      sync.Mutex
	fetches atomic.Int64
	blockCh chan struct{}
}

func newBlockingStore(ranges []Range) *blockingStore {
	return &blockingStore{
		ranges:  append([]Range(nil), ranges...),
		blockCh: make(chan struct{}),
	}
}

func (s *blockingStore) Fetch(ctx context.Context, _ string, _ string, _ int64) (Range, SegmentConfig, error) {
	fetchNo := s.fetches.Add(1)
	if fetchNo == 2 {
		select {
		case <-s.blockCh:
		case <-ctx.Done():
			return Range{}, SegmentConfig{}, ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ranges) == 0 {
		return Range{}, SegmentConfig{}, ErrSegmentExhausted
	}
	r := s.ranges[0]
	s.ranges = s.ranges[1:]
	return r, SegmentConfig{
		Step:    r.Len(),
		MinStep: r.Len(),
		MaxStep: r.Len(),
		Status:  SegmentStatusEnabled,
	}, nil
}

func (s *blockingStore) Ensure(ctx context.Context, _ string, _ string, _ EnsureSegmentConfig) error {
	return ctx.Err()
}

func (s *blockingStore) Health(ctx context.Context) error {
	return ctx.Err()
}

func (s *blockingStore) releaseFetch() {
	close(s.blockCh)
}

func (s *blockingStore) waitFetchCount(t *testing.T, want int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.fetches.Load() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("fetches = %d, want at least %d", s.fetches.Load(), want)
}
