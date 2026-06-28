package idgen

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type segmentGenerator struct {
	component *Component
	name      string
	policy    stepPolicy

	mu             sync.Mutex
	current        segment
	next           segment
	nextReady      bool
	currentVersion atomic.Uint64
	initialized    atomic.Bool
	threadRunning  atomic.Bool

	step        int64
	minStep     int64
	maxStep     int64
	lastFetchAt time.Time
	lastErr     string

	generated         atomic.Uint64
	prefetches        atomic.Uint64
	segmentFetches    atomic.Uint64
	segmentFetchFails atomic.Uint64
}

func newSegmentGenerator(component *Component, name string) *segmentGenerator {
	return &segmentGenerator{
		component: component,
		name:      name,
		policy: stepPolicy{
			dynamic:        component.config.DynamicStep,
			targetDuration: component.config.TargetDuration,
		},
	}
}

func (g *segmentGenerator) Next(ctx context.Context) (int64, error) {
	var err error
	defer func() { g.component.observeGenerate(g.name, "next", err) }()

	if g.component.failClosed() {
		err = ErrMachineLeaseLost
		return 0, err
	}
	if er := g.ensureReady(ctx); er != nil {
		err = er
		return 0, err
	}
	for {
		if g.component.failClosed() {
			err = ErrMachineLeaseLost
			return 0, err
		}
		g.maybePrefetch()
		if id, ok := g.current.next(); ok {
			g.generated.Add(1)
			g.component.observeRemaining(g.name, g.current.remaining())
			return id, nil
		}
		if er := g.switchOrFetch(ctx, 1); er != nil {
			err = er
			return 0, err
		}
	}
}

func (g *segmentGenerator) Reserve(ctx context.Context, n int64) (Range, error) {
	var err error
	defer func() { g.component.observeGenerate(g.name, "reserve", err) }()

	if n <= 0 {
		err = fmt.Errorf("%w: reserve size must be positive", ErrInvalidConfig)
		return Range{}, err
	}
	if g.component.failClosed() {
		err = ErrMachineLeaseLost
		return Range{}, err
	}
	if er := g.ensureReady(ctx); er != nil {
		err = er
		return Range{}, err
	}
	for {
		if g.component.failClosed() {
			err = ErrMachineLeaseLost
			return Range{}, err
		}
		g.maybePrefetch()
		if r, ok := g.current.reserve(n); ok {
			g.generated.Add(uint64(n))
			g.component.observeRemaining(g.name, g.current.remaining())
			return r, nil
		}
		if er := g.switchOrFetch(ctx, n); er != nil {
			err = er
			return Range{}, err
		}
	}
}

func (g *segmentGenerator) Stats() Stats {
	g.mu.Lock()
	defer g.mu.Unlock()
	return Stats{
		Name:              g.name,
		Current:           g.current.snapshot(),
		Next:              g.next.snapshot(),
		CurrentRemaining:  g.current.remaining(),
		NextReady:         g.nextReady,
		Initialized:       g.initialized.Load(),
		ThreadRunning:     g.threadRunning.Load(),
		Step:              g.step,
		MinStep:           g.minStep,
		MaxStep:           g.maxStep,
		LastFetchAt:       g.lastFetchAt,
		LastError:         g.lastErr,
		Generated:         g.generated.Load(),
		Prefetches:        g.prefetches.Load(),
		SegmentFetches:    g.segmentFetches.Load(),
		SegmentFetchFails: g.segmentFetchFails.Load(),
	}
}

func (g *segmentGenerator) ensureReady(ctx context.Context) error {
	if g.initialized.Load() {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.initialized.Load() {
		return nil
	}
	r, cfg, err := g.fetchSegmentLocked(ctx, 1)
	if err != nil {
		g.setErrorLocked(err)
		return err
	}
	g.current.reset(r)
	g.applyConfigLocked(cfg, r.Len())
	g.currentVersion.Add(1)
	g.initialized.Store(true)
	g.lastErr = ""
	return nil
}

func (g *segmentGenerator) maybePrefetch() {
	if !g.initialized.Load() || g.threadRunning.Load() {
		return
	}
	g.mu.Lock()
	if g.nextReady {
		g.mu.Unlock()
		return
	}
	currentLen := g.current.len()
	remaining := g.current.remaining()
	threshold := int64(float64(currentLen) * g.component.config.PrefetchRemainingRatio)
	shouldPrefetch := currentLen > 0 && remaining <= threshold
	if !shouldPrefetch || !g.threadRunning.CompareAndSwap(false, true) {
		g.mu.Unlock()
		return
	}
	version := g.currentVersion.Load()
	g.mu.Unlock()

	started := g.component.startWorker(func(ctx context.Context) {
		defer g.threadRunning.Store(false)
		if !g.component.tryAcquirePrefetch(ctx) {
			g.component.observePrefetch(g.name, metricStatusSkipped)
			return
		}
		defer g.component.releasePrefetch()
		g.prefetches.Add(1)
		g.component.observePrefetch(g.name, g.prefetch(ctx, version))
	})
	if !started {
		g.threadRunning.Store(false)
	}
}

func (g *segmentGenerator) prefetch(ctx context.Context, version uint64) string {
	g.mu.Lock()
	if g.currentVersion.Load() != version || g.nextReady {
		g.mu.Unlock()
		return metricStatusStale
	}
	requested := g.nextStepLocked()
	if requested < 1 {
		requested = 1
	}
	g.mu.Unlock()

	r, cfg, err := g.fetchSegmentWithStep(ctx, requested, 1)
	g.mu.Lock()
	defer g.mu.Unlock()
	if err != nil {
		g.setErrorLocked(err)
		return metricStatusError
	}
	if g.currentVersion.Load() != version {
		return metricStatusStale
	}
	g.next.reset(r)
	g.nextReady = true
	g.applyConfigLocked(cfg, r.Len())
	g.lastErr = ""
	return metricStatusOK
}

func (g *segmentGenerator) switchOrFetch(ctx context.Context, need int64) error {
	deadline := time.Now().Add(g.component.config.WaitTimeout)
	for {
		g.mu.Lock()
		if g.nextReady && g.next.len() >= need {
			g.current.reset(g.next.snapshot())
			g.next.reset(Range{})
			g.nextReady = false
			g.currentVersion.Add(1)
			g.mu.Unlock()
			return nil
		}
		if !g.threadRunning.Load() {
			r, cfg, err := g.fetchSegmentLocked(ctx, need)
			if err != nil {
				g.setErrorLocked(err)
				g.mu.Unlock()
				return err
			}
			g.current.reset(r)
			g.next = segment{}
			g.nextReady = false
			g.applyConfigLocked(cfg, r.Len())
			g.currentVersion.Add(1)
			g.lastErr = ""
			g.mu.Unlock()
			return nil
		}
		g.mu.Unlock()

		if time.Now().After(deadline) {
			g.mu.Lock()
			if g.nextReady && g.next.len() >= need {
				g.current.reset(g.next.snapshot())
				g.next.reset(Range{})
				g.nextReady = false
				g.currentVersion.Add(1)
				g.mu.Unlock()
				return nil
			}
			r, cfg, err := g.fetchSegmentLocked(ctx, need)
			if err != nil {
				g.setErrorLocked(err)
				g.mu.Unlock()
				return err
			}
			g.current.reset(r)
			g.next = segment{}
			g.nextReady = false
			g.applyConfigLocked(cfg, r.Len())
			g.currentVersion.Add(1)
			g.lastErr = ""
			g.mu.Unlock()
			return nil
		}

		timer := time.NewTimer(200 * time.Microsecond)
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return ctx.Err()
		case <-g.component.rootCtx.Done():
			stopTimer(timer)
			return ErrComponentClosed
		case <-timer.C:
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}

func (g *segmentGenerator) fetchSegmentLocked(ctx context.Context, need int64) (Range, SegmentConfig, error) {
	requested := g.nextStepLocked()
	if requested < need {
		requested = need
	}
	return g.fetchSegmentWithStep(ctx, requested, need)
}

func (g *segmentGenerator) fetchSegmentWithStep(ctx context.Context, requested int64, need int64) (Range, SegmentConfig, error) {
	if g.component.stopped.Load() {
		return Range{}, SegmentConfig{}, ErrComponentClosed
	}
	if requested < need {
		requested = need
	}
	deadlineCtx, cancel := g.component.fetchContext(ctx)
	defer cancel()
	begin := time.Now()
	r, cfg, err := g.component.store.Fetch(deadlineCtx, g.component.config.Namespace, g.name, requested)
	g.component.observeSegmentFetch(g.name, begin, err)
	if err != nil {
		g.segmentFetchFails.Add(1)
		return Range{}, SegmentConfig{}, err
	}
	if r.Empty() {
		g.segmentFetchFails.Add(1)
		return Range{}, SegmentConfig{}, ErrSegmentExhausted
	}
	if r.Len() < need {
		g.segmentFetchFails.Add(1)
		return Range{}, SegmentConfig{}, ErrSegmentExhausted
	}
	if cfg.Status != 0 && cfg.Status != SegmentStatusEnabled {
		g.segmentFetchFails.Add(1)
		return Range{}, SegmentConfig{}, ErrNameDisabled
	}
	g.segmentFetches.Add(1)
	return r, cfg, nil
}

func (g *segmentGenerator) nextStepLocked() int64 {
	elapsed := time.Duration(0)
	if !g.lastFetchAt.IsZero() {
		elapsed = time.Since(g.lastFetchAt)
	}
	return g.policy.next(g.step, g.minStep, g.maxStep, elapsed)
}

func (g *segmentGenerator) applyConfigLocked(cfg SegmentConfig, actualStep int64) {
	if cfg.MinStep > 0 {
		g.minStep = cfg.MinStep
	} else if cfg.Step > 0 {
		g.minStep = cfg.Step
	}
	if cfg.MaxStep > 0 {
		g.maxStep = cfg.MaxStep
	} else if g.maxStep <= 0 && cfg.Step > 0 {
		g.maxStep = cfg.Step
	}
	if actualStep > 0 {
		g.step = actualStep
	} else if cfg.Step > 0 {
		g.step = cfg.Step
	}
	if g.minStep <= 0 {
		g.minStep = g.step
	}
	if g.maxStep <= 0 {
		g.maxStep = g.step
	}
	g.lastFetchAt = time.Now()
	g.component.observeStep(g.name, g.step)
}

func (g *segmentGenerator) setErrorLocked(err error) {
	if err == nil {
		g.lastErr = ""
		return
	}
	g.lastErr = err.Error()
}
