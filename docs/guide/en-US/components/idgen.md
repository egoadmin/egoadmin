# IDGen ID Generation

IDGen provides globally unique ID generation for EgoAdmin, supporting both Snowflake and Segment strategies, along with idcodec stable ID encoding/decoding and machine lease management.

## Overview

EgoAdmin's IDGen component lives in `internal/component/idgen` and exposes core capabilities through the `Interface` interface. It consists of three sub-modules:

| Sub-module | Package Path | Responsibility |
|------------|--------------|----------------|
| Segment Generator | `internal/component/idgen` | Segment mode: pre-allocates ID ranges from the database to reduce round trips |
| idcodec Encoder | `internal/component/idgen/idcodec` | Reversibly encodes internal numeric IDs into stable public IDs |
| Machine Lease | `internal/component/idgen` (`MachineLeaseManager`) | Distributed worker ID coordination with process-level machine leases |

::: tip When to Use IDGen
- **Database primary keys**: Use the default segment generator `Next()` / `NextDefault()`.
- **High-throughput batch allocation**: Use `Reserve()` to obtain an ID range in one call.
- **Public ID display**: Use idcodec to encode internal IDs, avoiding exposure of auto-increment sequences.
- **External system references**: idcodec-encoded IDs can be used in URLs, invite codes, and other user-facing surfaces.
:::

## Core Usage

### Segment ID Generation

Segment mode is EgoAdmin's default ID generation strategy. On startup, the component pre-allocates an ID range (`[Start, End)`) from the database. Within the process, IDs are distributed via atomic operations, and the next segment is asynchronously prefetched when remaining capacity drops below the threshold.

```go
// Injected via Wire as idgen.Component
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
| `Stats(name)` | Query the current generator snapshot (remaining, prefetch status, etc.) |
| `Health(ctx)` | Health check (store availability + machine lease status) |

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

### Stable ID Encoding/Decoding (idcodec)

idcodec encodes internal numeric IDs into prefixed stable public IDs. Encoding uses a Feistel network + HMAC-SHA256 + Base62, making it reversible but unpredictable.

```go
codec := idcodec.Load("component.idgen.codec").Build()

// Encode: internal ID -> public ID
publicID, err := codec.Encode("order", 12345)
// Result: "order-07uQlcBmL6d0"

// Decode: public ID -> internal ID
prefix, id, err := codec.Decode(publicID)
// prefix = "order", id = 12345

// Decode with prefix verification
id, err := codec.DecodeWithPrefix("order", publicID)
// id = 12345
```

idcodec core interface:

| Method | Description |
|--------|-------------|
| `Encode(prefix, id)` | Encode a positive int64 ID into `prefix-separator-Base62Body` format |
| `Decode(value)` | Decode a public ID, returns `(prefix, id, err)` |
| `DecodeWithPrefix(prefix, value)` | Decode and verify the prefix matches |

::: warning idcodec Is Not an Authorization Mechanism
idcodec uses HMAC-SHA256 + Feistel network for reversible encoding. It is obfuscation, not encryption, and provides no security guarantees. Do not use idcodec for security-sensitive scenarios (invite codes, password reset tokens) -- use cryptographically secure random tokens instead.
:::

### Public IDs at the API Layer

In a typical layered architecture, idcodec handles encoding/decoding at the adapter/schema layer while the domain layer always uses `int64`:

```go
// Adapter layer: encode
func toPublicUser(u *domain.User) *schema.UserPublic {
    publicID, _ := idcodec.Encode("usr", u.ID)
    return &schema.UserPublic{
        ID:        publicID,
        Name:      u.Name,
        CreatedAt: u.CreatedAt,
    }
}

// Adapter layer: decode
func parseUserID(publicID string) (int64, error) {
    return idcodec.DecodeWithPrefix("usr", publicID)
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

## Configuration Examples

### Component Configuration (config.toml)

```toml
# Segment generator instance
[component.idgen.default]
namespace = "egoadmin-local"       # Namespace isolation; use different values per environment
name = "default"                   # Instance name, corresponds to the Segment name
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

# Machine lease configuration
[component.idgen.machine]
group = "egoadmin-local"           # Lease group name; processes in the same group share the machine ID space
maxMachineID = 1023                # Maximum machine ID
ttl = "60s"                        # Lease TTL
renewInterval = "10s"              # Renewal interval (must be less than TTL)
renewTimeout = "5s"                # Renewal timeout
minRenewWindows = 5                # Minimum renewal windows; TTL covers at least (minRenewWindows+1) * renewInterval
reallocateBackoff = "2s"           # Backoff after allocation failure
stableInstanceID = ""              # Stable instance ID (e.g., Kubernetes Pod name); defaults to hostname:pid
lostPolicy = "fail_closed"         # Lease loss policy: degraded or fail_closed

# idcodec configuration
[component.idgen.codec]
secret = "local-stable-idcodec-secret"  # HMAC key (at least 16 bytes)
algorithm = "feistel-base62"             # Encoding algorithm
alphabet = "base62"                      # Alphabet: base62 or a custom 62-character string
minLength = 12                           # Minimum body length after encoding
separator = "-"                          # Separator between prefix and body
enableMetrics = true                     # Enable Prometheus metrics
```

### gRPC Client Configuration

Non-idgen services access the IDGen service via a gRPC client:

```toml
[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"    # Service discovery address
debug = true                       # Enable debug logging
readTimeout = "3s"                 # Read timeout
dialTimeout = "5s"                 # Dial timeout
```

The client is injected via Wire:

```go
// idgenclient.Client is automatically built via egrpc.Load("client.grpc.idgen")
type Client struct {
    Segment      SegmentService       // Segment operations
    MachineLease MachineLeaseService  // Lease operations
}
```

### Default Values Quick Reference

| Config Key | Default | Description |
|------------|---------|-------------|
| `step` | `100000` | Pre-allocation size per fetch |
| `dynamicStep` | `true` | Adjust step size based on consumption rate |
| `targetDuration` | `15m` | Dynamic step target interval |
| `prefetchRemainingRatio` | `0.2` | Trigger prefetch at 20% remaining |
| `maxPrefetchWorkers` | `8` | Max concurrent prefetch workers |
| `maxMachineID` | `1023` | Maximum machine ID |
| `lostPolicy` | `fail_closed` | Refuse ID generation when lease is lost |
| `secret` (codec) | none | Required, at least 16 bytes |
| `minLength` (codec) | `12` | Minimum encoded body length |

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
    idgen idgen.Interface
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

// Adapter layer: return public ID
func toPublicUser(u *User) *UserPublicResponse {
    publicID, _ := idcodec.Encode("usr", u.ID)
    return &UserPublicResponse{
        ID:    publicID,  // "usr-Ab3kL9xQm2Nf"
        Name:  u.Name,
        Email: u.Email,
    }
}
```

### Batch Order Import

```go
func (uc *ImportOrdersUseCase) Execute(ctx context.Context, rawOrders []RawOrder) error {
    rng, err := uc.idgen.Reserve(ctx, "order_id", int64(len(rawOrders)))
    if err != nil {
        return fmt.Errorf("reserve order ids: %w", err)
    }

    orders := make([]*Order, len(rawOrders))
    for i, raw := range rawOrders {
        orders[i] = &Order{
            ID:     rng.Start + int64(i),
            UserID: raw.UserID,
            Amount: raw.Amount,
        }
    }
    return uc.orderRepo.BatchCreate(ctx, orders)
}
```

### Prefixed Public ID Route Parsing

```go
// Parse a public ID from the URL
func ParseResourceID(prefix string, publicID string) (int64, error) {
    id, err := idcodec.DecodeWithPrefix(prefix, publicID)
    if err != nil {
        return 0, fmt.Errorf("invalid %s id: %w", prefix, err)
    }
    return id, nil
}

// Example route: GET /orders/:id
// URL: /orders/order-07uQlcBmL6d0
// Parsed: order-07uQlcBmL6d0 -> 12345
```

## Machine Lease Details

Each IDGen process requests a machine lease from the IDGen service at startup, used for Snowflake algorithm worker ID coordination.

### Lifecycle

```text
Start() -> AllocateLease -> [renew loop via ecron] -> Stop() / StopWithoutRelease()
```

1. **Allocation**: On process startup, `AllocateLease` is called to obtain a unique `machineID`.
2. **Renewal**: A background cron job (`cron.idgen.machine.renew`) renews the lease at `renewInterval`.
3. **Loss handling**:
   - `fail_closed` (default): Refuse ID generation when the lease is lost.
   - `degraded`: Allow continued use of allocated segments.
4. **Release**: On process exit, call `Stop()` to release the lease, or `StopWithoutRelease()` to silently stop renewal.

### Graceful Shutdown

```go
// Recommended: register in shutdown.Manager
shutdown.RegisterWithTimeout(5*time.Second, func(ctx context.Context) {
    machineManager.StopWithoutRelease(ctx)
})
```

`StopWithoutRelease` stops renewal without calling the remote release endpoint, avoiding noisy errors when the IDGen service has already stopped. The lease is automatically reclaimed after TTL expiration.

### Instance ID Strategy

| Scenario | Recommended Configuration |
|----------|--------------------------|
| Local development | Leave `stableInstanceID` empty; uses `hostname:pid` |
| Docker Compose | Use the container name |
| Kubernetes | Use the Pod name (`metadata.name`) |

## How It Works

### Segment Mode

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
- **Concurrency safe**: The segment uses `atomic.Int64` for lock-free allocation.
- **Auto Ensure**: When `autoEnsure = true`, missing segment definitions are created in the database during startup.

### Dynamic Step Algorithm

When `dynamicStep = true`, the step size adjusts based on segment consumption speed:

- Consumption time < `targetDuration` (15 minutes): Step doubles (fast consumption needs a larger step).
- Consumption time >= 2 * `targetDuration`: Step halves (slow consumption reduces waste).
- Otherwise: Step remains unchanged.

The step is always constrained within `[minStep, maxStep]`.

### Prefetch Mechanism

Prefetch is triggered when the current segment's remaining capacity drops to or below `prefetchRemainingRatio` (default 20%):

1. A background goroutine loads the next segment from the database into `next`.
2. When `current` is exhausted, if `next` is ready, it switches immediately (zero latency).
3. If `next` is not ready, it waits up to `waitTimeout` (200ms), then falls back to synchronous fetch.
4. Prefetch is limited by `maxPrefetchWorkers` to avoid excessive concurrent database requests.

### Snowflake Mode

When machine leases are configured, IDGen also supports the Snowflake algorithm. The 64-bit ID structure:

```text
|  0  |  41-bit timestamp  |  10-bit machine ID  |  12-bit sequence  |
| --- | ------------------ | ------------------- | ----------------- |
|  1b |        41b         |         10b         |        12b        |
```

- **Timestamp**: Millisecond-precision, relative to a custom epoch.
- **Machine ID**: Allocated by machine lease, maximum 1023.
- **Sequence**: Auto-incrementing within the same millisecond, up to 4096 per millisecond.

### idcodec Encoding Algorithm

```text
Internal ID (int64)
    |
    v
Feistel network permutation (6 rounds, HMAC-SHA256 key derivation)
    |
    v
Base62 encoding (configurable alphabet)
    |
    v
Pad to minLength + concatenate prefix-separator-body
    |
    v
Public ID: "order-07uQlcBmL6d0"
```

- Uses a Feistel network for reversible bit permutation. Each round's function uses HMAC-SHA256 with the secret and prefix as input.
- Different prefixes produce different permutation mappings -- the same numeric ID encodes differently under different prefixes.
- Decoding is the exact inverse of encoding.

### Prometheus Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `idgen_generated_total` | Counter | component, name, operation, status | ID generation count |
| `idgen_segment_fetch_total` | Counter | component, name, status | Segment fetch count |
| `idgen_segment_fetch_seconds` | Histogram | component, name | Segment fetch latency |
| `idgen_segment_remaining` | Gauge | component, name | Current segment remaining |
| `idgen_segment_step` | Gauge | component, name | Current effective step size |
| `idgen_prefetch_total` | Counter | component, name, status | Prefetch attempt count |
| `idgen_machine_lease_renew_total` | Counter | component, status | Lease renewal count |
| `idgen_machine_lease_status` | Gauge | component | Lease status (1=valid, 0=lost) |
| `idgen_health_status` | Gauge | component | Component health status |

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

### Machine ID Collision

::: warning
Ensure each worker process has a unique machine lease.
:::

**Cause**: Multiple processes share the same `stableInstanceID`.

**Diagnosis**: Check the `[component.idgen.machine].stableInstanceID` configuration. In container environments, use unique Pod/container names.

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

### idcodec Decode Failure

**Symptoms**: `ErrInvalidFormat` or `ErrInvalidPrefix`.

**Checklist**:

- Is the public ID complete (not truncated)?
- Is the prefix correct (case-sensitive)?
- Does the separator match the configuration?
- Is the idcodec instance's secret the same as when encoding?

### Segment Exhaustion or Prefetch Timeout

**Symptoms**: `ErrSegmentExhausted` or `ErrStoreUnavailable`.

**Diagnosis**:

- Is the database connection healthy?
- Is `fetchTimeout` too short (default 2 seconds)?
- Is the segment's step size too small (consider increasing for high-concurrency scenarios)?
- Check `Stats()` for the `SegmentFetchFails` counter.

### Dynamic Step Adjustment Not Working

**Cause**: `dynamicStep = false` or unreasonable `targetDuration` configuration.

Dynamic step behavior:

- Consumption faster than target -> next step doubles (capped at `maxStep`).
- Consumption slower than target (over 2x) -> next step halves (floored at `minStep`).
- Consumption within target range -> step unchanged.

### ID Uniqueness After Lease Loss

In `LostPolicy = "degraded"` mode, ID generation continues after lease loss, but the Snowflake machine ID portion is no longer guaranteed unique. Use the `fail_closed` policy if strong uniqueness is required.

### Component Shutdown

```go
// Graceful shutdown: stop renewal, release remote lease
err := comp.Stop()

// Stop renewal without releasing the remote lease
// (suitable when the remote service has already stopped during process exit)
err := machineManager.StopWithoutRelease(ctx)
```

## Reference Links

- `internal/component/idgen/component.go` -- Component definition and lifecycle (Next, Reserve, Generator, Health)
- `internal/component/idgen/config.go` -- Config and MachineConfig structs with defaults
- `internal/component/idgen/container.go` -- Load / Build flow
- `internal/component/idgen/interface.go` -- Interface, Generator, SegmentStore, MachineAllocator interface definitions
- `internal/component/idgen/generator.go` -- segmentGenerator implementation (Next, Reserve, prefetch, switchOrFetch)
- `internal/component/idgen/segment.go` -- Lock-free segment data structure (atomic.Int64 cursor)
- `internal/component/idgen/step.go` -- Dynamic step policy (stepPolicy)
- `internal/component/idgen/machine_manager.go` -- ProcessMachineLeaseManager (Allocate, Renew, Release, StopWithoutRelease)
- `internal/component/idgen/machine_cron.go` -- ecron lease renewal cron job
- `internal/component/idgen/options.go` -- Build function options (WithConfig, WithSegmentStore, WithMachineLeaseManager)
- `internal/component/idgen/errors.go` -- Error constants
- `internal/component/idgen/metrics.go` -- Prometheus metric definitions
- `internal/component/idgen/idgetter.go` -- IDGetter adapter (xflake.Geter shape)
- `internal/component/idgen/idcodec/component.go` -- idcodec Feistel + Base62 encoding/decoding implementation
- `internal/component/idgen/idcodec/config.go` -- idcodec configuration and defaults
- `internal/component/idgen/idcodec/interface.go` -- idcodec Interface definition (Encode, Decode, DecodeWithPrefix)
- `internal/component/idgen/idcodec/container.go` -- idcodec Load / Build flow
- `internal/client/idgenclient/client.go` -- gRPC client (SegmentService + MachineLeaseService)
