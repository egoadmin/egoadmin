package idgen

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gotomicro/ego/core/elog"
)

// Component is an EGO-style high-throughput segment ID generator.
type Component struct {
	name           string
	config         *Config
	logger         *elog.Component
	store          SegmentStore
	machineManager MachineLeaseManager

	rootCtx    context.Context
	cancelRoot context.CancelFunc

	mu             sync.RWMutex
	generators     map[string]*segmentGenerator
	prefetchTokens chan struct{}

	lifecycleMu sync.Mutex
	wg          sync.WaitGroup
	started     atomic.Bool
	stopped     atomic.Bool
}

func newComponent(name string, config *Config, logger *elog.Component, store SegmentStore, manager MachineLeaseManager) (*Component, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	config.normalize()
	if err := config.validate(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("%w: segment store is nil", ErrInvalidConfig)
	}
	if manager == nil {
		return nil, fmt.Errorf("%w: machine lease manager is nil", ErrInvalidConfig)
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	return &Component{
		name:           name,
		config:         config,
		logger:         logger,
		store:          store,
		machineManager: manager,
		rootCtx:        rootCtx,
		cancelRoot:     cancel,
		generators:     make(map[string]*segmentGenerator),
		prefetchTokens: make(chan struct{}, config.MaxPrefetchWorkers),
	}, nil
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) PackageName() string {
	return PackageName
}

func (c *Component) Init() error {
	if c == nil || c.config == nil {
		return fmt.Errorf("%w: component is nil", ErrInvalidConfig)
	}
	c.config.normalize()
	return c.config.validate()
}

func (c *Component) Start() error {
	if !c.started.CompareAndSwap(false, true) {
		return nil
	}
	started := false
	defer func() {
		if !started {
			c.started.Store(false)
		}
	}()
	ctx := context.Background()
	if c.machineManager != nil {
		if err := c.machineManager.Health(ctx); err != nil {
			return fmt.Errorf("idgen machine health: %w", err)
		}
	}
	if c.config.AutoEnsure {
		if err := c.store.Ensure(ctx, c.config.Namespace, c.config.Name, EnsureSegmentConfig{
			NextID:      1,
			Step:        c.config.Step,
			MinStep:     c.config.MinStep,
			MaxStep:     c.config.MaxStep,
			Status:      SegmentStatusEnabled,
			Description: fmt.Sprintf("auto ensured by %s", c.name),
		}); err != nil {
			return fmt.Errorf("ensure idgen segment %s/%s: %w", c.config.Namespace, c.config.Name, err)
		}
	}
	if c.config.Warmup {
		generator, err := c.Generator(c.config.Name)
		if err != nil {
			return err
		}
		if segmentGen, ok := generator.(*segmentGenerator); ok {
			if err = segmentGen.ensureReady(context.Background()); err != nil {
				return fmt.Errorf("warmup idgen name %s: %w", c.config.Name, err)
			}
		}
	}
	started = true
	return nil
}

func (c *Component) Stop() error {
	c.lifecycleMu.Lock()
	if c.stopped.Load() {
		c.lifecycleMu.Unlock()
		return nil
	}
	c.stopped.Store(true)
	c.cancelRoot()
	c.lifecycleMu.Unlock()

	c.wg.Wait()
	return nil
}

func (c *Component) Close() error {
	return c.Stop()
}

func (c *Component) Health(ctx context.Context) error {
	var err error
	defer func() { c.observeHealth(err) }()

	if c.stopped.Load() {
		err = ErrComponentClosed
		return err
	}
	if c.machineManager != nil {
		if er := c.machineManager.Health(ctx); er != nil {
			err = er
			return err
		}
	}
	if er := c.store.Health(ctx); er != nil {
		err = fmt.Errorf("%w: %v", ErrStoreUnavailable, er)
		return err
	}
	return nil
}

func (c *Component) Next(ctx context.Context, name string) (int64, error) {
	g, err := c.Generator(name)
	if err != nil {
		c.observeGenerate(name, "next", err)
		return 0, err
	}
	return g.Next(ctx)
}

func (c *Component) NextDefault(ctx context.Context) (int64, error) {
	return c.Next(ctx, c.config.Name)
}

func (c *Component) Reserve(ctx context.Context, name string, n int64) (Range, error) {
	g, err := c.Generator(name)
	if err != nil {
		c.observeGenerate(name, "reserve", err)
		return Range{}, err
	}
	return g.Reserve(ctx, n)
}

func (c *Component) ReserveDefault(ctx context.Context, n int64) (Range, error) {
	return c.Reserve(ctx, c.config.Name, n)
}

func (c *Component) Generator(name string) (Generator, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name is empty", ErrInvalidConfig)
	}
	if c.stopped.Load() {
		return nil, ErrComponentClosed
	}
	c.mu.RLock()
	g, ok := c.generators[name]
	c.mu.RUnlock()
	if ok {
		return g, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if g, ok = c.generators[name]; ok {
		return g, nil
	}
	g = newSegmentGenerator(c, name)
	c.generators[name] = g
	return g, nil
}

func (c *Component) GeneratorDefault() (Generator, error) {
	return c.Generator(c.config.Name)
}

func (c *Component) Stats(name string) (Stats, bool) {
	c.mu.RLock()
	g, ok := c.generators[name]
	c.mu.RUnlock()
	if !ok {
		return Stats{}, false
	}
	return g.Stats(), true
}

func (c *Component) startWorker(fn func(context.Context)) bool {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.stopped.Load() {
		return false
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		fn(c.rootCtx)
	}()
	return true
}

func (c *Component) tryAcquirePrefetch(ctx context.Context) bool {
	select {
	case c.prefetchTokens <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func (c *Component) releasePrefetch() {
	select {
	case <-c.prefetchTokens:
	default:
	}
}

func (c *Component) fetchContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	timeout := c.config.FetchTimeout
	if timeout <= 0 {
		timeout = DefaultConfig().FetchTimeout
	}
	return context.WithTimeout(parent, timeout)
}

func (c *Component) failClosed() bool {
	if c.machineManager == nil {
		return false
	}
	policy := LostPolicyDegraded
	if manager, ok := c.machineManager.(interface{ LostPolicy() LostPolicy }); ok {
		policy = manager.LostPolicy()
	}
	if policy != LostPolicyFailClosed {
		return false
	}
	if manager, ok := c.machineManager.(interface{ LeaseLost() bool }); ok {
		return manager.LeaseLost()
	}
	return errors.Is(c.machineManager.Health(context.Background()), ErrMachineLeaseLost)
}
