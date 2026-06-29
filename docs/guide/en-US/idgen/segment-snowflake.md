# Segment and Snowflake Algorithms

EgoAdmin's IDGen service provides two globally unique ID generation strategies -- Segment and Snowflake -- both exposed through a unified `internal/component/idgen` component.

## Overview

EgoAdmin uses Segment mode as the primary ID generation strategy, achieving high throughput through database-preallocated ID ranges. Snowflake mode supplements it for scenarios requiring time-ordered IDs. Both strategies share the same component framework and machine lease management.

| Strategy | Bit Width | Generation Method | Use Case |
|----------|-----------|-------------------|----------|
| Segment | `int64` | Database-preallocated range + in-process atomic increment | Database primary keys, batch imports |
| Snowflake | `uint64` | Timestamp + machine ID + sequence | Time-ordered distributed IDs |

::: tip Selection Guidance
Most business scenarios only need Segment mode. Snowflake mode requires system clock synchronization via NTP. EgoAdmin's `IDGetter` adapter wraps the segment generator into the Snowflake `xflake.Geter` interface shape.
:::

## Core Usage

### Segment Mode

Segment mode is EgoAdmin's default ID generation strategy. On startup, the component pre-allocates an ID range `[Start, End)` from the database. Within the process, IDs are distributed via atomic operations, and the next segment is asynchronously prefetched when remaining capacity drops below the threshold.

```go
// Injected via Wire as idgen.Component
type UserRepository struct {
    idgen *idgen.Component
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    id, err := r.idgen.Next(ctx, "user_id")
    if err != nil {
        return fmt.Errorf("generate user id: %w", err)
    }
    user.ID = id
    return r.db.WithContext(ctx).Create(user).Error
}
```

Common methods:

| Method | Description |
|--------|-------------|
| `Next(ctx, name)` | Get the next ID (`int64`) |
| `NextDefault(ctx)` | Get the next ID using the configured default name |
| `Reserve(ctx, name, n)` | Reserve n IDs at once, returns `Range{Start, End}` |
| `ReserveDefault(ctx, n)` | Batch reserve IDs using the default name |
| `Generator(name)` | Get a named generator handle for high-frequency reuse |

### Batch ID Reservation

For scenarios that need to allocate many IDs at once (e.g., batch imports), use `Reserve` to avoid per-call overhead:

```go
func (r *OrderRepository) BatchCreate(ctx context.Context, orders []*Order) error {
    rng, err := r.idgen.Reserve(ctx, "order_id", int64(len(orders)))
    if err != nil {
        return fmt.Errorf("reserve order ids: %w", err)
    }
    for i, order := range orders {
        order.ID = rng.Start + int64(i)
    }
    return r.db.WithContext(ctx).CreateInBatches(orders, 100).Error
}
```

`Range` is a half-open interval `[Start, End)` -- the caller distributes IDs as needed.

### Snowflake Mode

When machine leases are configured, IDGen supports the Snowflake algorithm through the `IDGetter` adapter. The 64-bit ID structure:

```text
| 1 bit sign | 41-bit timestamp | 10-bit machine ID | 12-bit sequence |
| ---------- | ---------------- | ----------------- | --------------- |
|     0      |     41 bits      |     10 bits       |    12 bits      |
```

- **Sign bit**: Fixed at 0, ensuring the ID is positive.
- **Timestamp**: Millisecond precision, relative to a custom epoch (January 1, 2020).
- **Machine ID**: Allocated by machine lease, range `[0, 1023]`.
- **Sequence**: Auto-incrementing within the same millisecond, up to 4096 per millisecond.

Snowflake mode is used through the `IDGetter` adapter:

```go
// Built in platform/idgen/provider.go
gen, err := component.GeneratorDefault()
getter := idgen.NewIDGetter(gen)

// Get an ID (uint64)
id, err := getter.Get()
```

::: warning Clock Sensitive
The Snowflake algorithm depends on system clock. Clock rollbacks may produce duplicate IDs. Segment mode is unaffected by clock drift.
:::

## Configuration Examples

### Component Configuration

```toml
# Segment generator instance
[component.idgen.default]
namespace = "egoadmin-local"       # Namespace isolation; use different values per environment
name = "default"                   # Instance name, corresponds to the segment name
step = 100000                      # IDs pre-allocated per fetch
minStep = 10000                    # Minimum dynamic step
maxStep = 100000000                # Maximum dynamic step
autoEnsure = true                  # Auto-create missing segment definitions on startup
warmup = true                      # Pre-load the first segment on startup
fetchTimeout = "2s"                # Timeout for fetching segments from the database
waitTimeout = "200ms"              # Timeout when waiting for prefetch on segment exhaustion
prefetchRemainingRatio = 0.2       # Trigger async prefetch when remaining ratio drops below this
dynamicStep = true                 # Enable dynamic step adjustment
targetDuration = "15m"             # Target consumption window for dynamic step
maxPrefetchWorkers = 8             # Max concurrent prefetch goroutines
enableMetrics = true               # Enable Prometheus metrics
```

### Minimal Configuration

```toml
[component.idgen.default]
namespace = "egoadmin-local"
name = "default"
```

::: tip Defaults
Unconfigured fields use the conservative defaults from `DefaultConfig()`: `step = 100000`, `dynamicStep = true`, `prefetchRemainingRatio = 0.2`, `targetDuration = 15m`.
:::

### Default Values Quick Reference

| Config Key | Default | Description |
|------------|---------|-------------|
| `namespace` | `"default"` | Namespace |
| `name` | `"default"` | Instance name |
| `step` | `100000` | Pre-allocation size per fetch |
| `minStep` | `10000` | Minimum dynamic step |
| `maxStep` | `100000000` | Maximum dynamic step |
| `autoEnsure` | `true` | Auto-create segment |
| `warmup` | `true` | Startup warmup |
| `fetchTimeout` | `2s` | Fetch timeout |
| `waitTimeout` | `200ms` | Wait-for-prefetch timeout |
| `prefetchRemainingRatio` | `0.2` | Prefetch threshold ratio |
| `dynamicStep` | `true` | Dynamic step sizing |
| `targetDuration` | `15m` | Step target interval |
| `maxPrefetchWorkers` | `8` | Max concurrent prefetch workers |

## Real-World Examples

### Complete User Creation Flow

```go
// Domain layer: User entity
type User struct {
    ID        int64
    Name      string
    Email     string
    CreatedAt time.Time
}

// Application layer: create use case
type CreateUserUseCase struct {
    idgen    idgen.Interface
    userRepo UserRepository
}

func (uc *CreateUserUseCase) Execute(ctx context.Context, cmd CreateUserCommand) (*User, error) {
    id, err := uc.idgen.Next(ctx, "user_id")
    if err != nil {
        return nil, fmt.Errorf("generate user id: %w", err)
    }
    user := &User{
        ID:    id,
        Name:  cmd.Name,
        Email: cmd.Email,
    }
    if err := uc.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }
    return user, nil
}
```

### Generator Cached Handle

For high-frequency ID generation, cache a Generator handle to avoid repeated lookups:

```go
gen, err := comp.Generator("order")

// Use gen directly for subsequent generation
id, err := gen.Next(ctx)

// Batch reservation
rng, err := gen.Reserve(ctx, 100)

// View statistics
stats := gen.Stats()
fmt.Printf("Generated: %d, Remaining: %d\n", stats.Generated, stats.CurrentRemaining)
```

### IDGetter Adapter

Adapts idgen to the `xflake.Geter` interface shape, returning `uint64`:

```go
gen, err := comp.Generator("snowflake")
getter := idgen.NewIDGetter(gen)

id, err := getter.Get() // returns uint64
```

## How It Works

### Segment Mode Architecture

```text
                  Application calls Next("user_id")
                         |
                         v
            segmentGenerator (in-process cache)
            Current segment: [1000, 2000), cursor = 1500
                         |
                    Atomic cursor increment
                         |
              Remaining < 20% -> async prefetch next segment
                         |
                    Return int64 ID
```

Core design:

- **Double buffering**: Holds both the current and prefetched segments (`current` + `next`), enabling seamless switchover when the current segment is exhausted.
- **Dynamic step**: Automatically adjusts the next prefetch size based on historical consumption rate (`stepPolicy`), targeting `targetDuration` (default 15 minutes) per segment.
- **Concurrency safe**: The segment uses `atomic.Pointer[segmentState]` and `atomic.Int64` for lock-free allocation.
- **Auto Ensure**: When `autoEnsure = true`, missing segment definitions are created in the database during startup.

### Segment Data Structure

```go
type segment struct {
    ptr atomic.Pointer[segmentState]  // Atomic pointer for lock-free state switching
}

type segmentState struct {
    start  int64       // Range start
    end    int64       // Range end (exclusive)
    cursor atomic.Int64 // Current cursor
}
```

Each `next()` call atomically increments the cursor via `cursor.Add(1) - 1`. When `cursor >= end`, the segment is exhausted, triggering switchover or prefetch.

### Double Buffering Switchover Flow

```text
Current segment exhausted
    |
    v
Is next ready? --yes--> current = next, next cleared, currentVersion++
    |
    no
    |
    v
fetchSegmentLocked() fetches new segment from database
    |
    v
current = new segment, next cleared
```

The `currentVersion` atomic counter ensures prefetch threads do not overwrite already-switched state.

### Dynamic Step Algorithm

When `dynamicStep = true`, the step size adjusts based on segment consumption speed:

```go
// internal/component/idgen/step.go
func (p stepPolicy) next(current, minStep, maxStep int64, elapsed time.Duration) int64 {
    if elapsed < p.targetDuration {
        return min(current*2, maxStep)   // Fast consumption -> double step
    }
    if elapsed >= 2*p.targetDuration {
        return max(current/2, minStep)   // Slow consumption -> halve step
    }
    return current                        // Moderate -> unchanged
}
```

The step is always constrained within `[minStep, maxStep]`. The server can dynamically control step policy via `SegmentConfig`.

### Prefetch Mechanism

Prefetch is triggered when the current segment's remaining capacity drops to or below `prefetchRemainingRatio` (default 20%):

1. A background goroutine loads the next segment from the database into `next`.
2. When `current` is exhausted, if `next` is ready, it switches immediately (zero latency).
3. If `next` is not ready, it waits up to `waitTimeout` (200ms), then falls back to synchronous fetch.
4. Prefetch is limited by `maxPrefetchWorkers` to avoid excessive concurrent database requests.

```go
func (g *segmentGenerator) maybePrefetch() {
    // Double-check: initialized and no prefetch thread running
    // Check remaining <= currentLen * prefetchRemainingRatio
    // Acquire prefetch thread slot via CompareAndSwap
    // Execute prefetch() asynchronously
}
```

### Snowflake Bit Allocation

```text
|  0  |  41-bit timestamp  |  10-bit machine ID  |  12-bit sequence  |
| --- | ------------------ | ------------------- | ----------------- |
|  1b |        41b         |         10b         |        12b        |
```

- Timestamp: Millisecond precision, epoch from January 1, 2020. 41 bits provides approximately 69 years.
- Machine ID: Allocated by machine lease, maximum 1023, supporting up to 1024 workers.
- Sequence: Auto-incrementing within the same millisecond, up to 4096 IDs per millisecond.

### Performance Characteristics

| Environment | Typical QPS | Notes |
|-------------|-------------|-------|
| Development | ~50K | Single process, local database |
| Production | ~200K | Multi-process, optimized DB connections |
| High-load | ~800K | Dynamic step + large step configuration |

The bottleneck for Segment mode is database segment allocation, not in-process generation. By tuning `step` and `dynamicStep`, database interaction frequency can be reduced to a minimum.

### Prometheus Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `idgen_generated_total` | Counter | component, name, operation, status | ID generation count |
| `idgen_segment_fetch_total` | Counter | component, name, status | Segment fetch count |
| `idgen_segment_fetch_seconds` | Histogram | component, name | Segment fetch latency |
| `idgen_segment_remaining` | Gauge | component, name | Current segment remaining |
| `idgen_segment_step` | Gauge | component, name | Current effective step size |
| `idgen_prefetch_total` | Counter | component, name, status | Prefetch attempt count |

## Common Issues

### Clock Drift Causing ID Conflicts

::: danger
The Snowflake algorithm is sensitive to system clock. Clock rollback may produce duplicate IDs.
:::

**Symptoms**: Logs show `idgen machine lease lost` or duplicate IDs.

**Diagnosis**:

```bash
# Check NTP synchronization status
timedatectl status
ntpq -p
```

**Mitigation**: Segment mode (EgoAdmin's default) is unaffected by clock drift, as it does not rely on the system clock to generate IDs.

### Segment Exhaustion or Prefetch Timeout

**Symptoms**: `ErrSegmentExhausted` or `ErrStoreUnavailable`.

**Diagnosis**:

- Is the database connection healthy?
- Is `fetchTimeout` too short (default 2 seconds)?
- Is the segment's step size too small (consider increasing for high-concurrency scenarios)?
- Check `Stats()` for the `SegmentFetchFails` counter.

```go
stats, ok := comp.Stats("user_id")
if ok && stats.SegmentFetchFails > 0 {
    log.Warn("segment fetch failures detected",
        zap.Uint64("failures", stats.SegmentFetchFails),
        zap.String("lastError", stats.LastError))
}
```

### Dynamic Step Adjustment Not Working

**Cause**: `dynamicStep = false` or unreasonable `targetDuration` configuration.

Dynamic step behavior:

- Consumption faster than target -> next step doubles (capped at `maxStep`).
- Consumption slower than target (over 2x) -> next step halves (floored at `minStep`).
- Consumption within target range -> step unchanged.

### JavaScript Precision Loss

::: warning
Go `int64` / `uint64` values exceeding JavaScript's `Number.MAX_SAFE_INTEGER` (2^53 - 1) will lose precision.
:::

**Solutions**:

1. Use idcodec to encode the ID as a string before sending to the frontend.
2. Define ID fields as `string` in proto/JSON serialization.
3. Always handle IDs as strings in the frontend; never perform numeric operations.

```protobuf
message UserResponse {
    string id = 1;    // Use string instead of uint64
    string name = 2;
}
```

### ID Uniqueness After Lease Loss

In `LostPolicy = "degraded"` mode, segment ID generation continues after lease loss (segments do not depend on machine ID), but Snowflake mode cannot continue generating IDs.

### Component Shutdown

```go
// Graceful shutdown: stop prefetch threads, wait for background goroutines
err := comp.Stop()
```

## Reference Links

- `internal/component/idgen/component.go` -- Component definition and lifecycle
- `internal/component/idgen/config.go` -- Config struct and defaults
- `internal/component/idgen/generator.go` -- segmentGenerator implementation (Next, Reserve, prefetch)
- `internal/component/idgen/segment.go` -- Lock-free segment data structure (atomic.Pointer + atomic.Int64)
- `internal/component/idgen/step.go` -- Dynamic step policy (stepPolicy)
- `internal/component/idgen/interface.go` -- Interface, Generator, SegmentStore interface definitions
- `internal/component/idgen/idgetter.go` -- IDGetter adapter (xflake.Geter shape)
- `internal/component/idgen/store/gormstore/store.go` -- GORM segment store implementation
- `internal/component/idgen/memory_store.go` -- In-memory segment store (for testing)
