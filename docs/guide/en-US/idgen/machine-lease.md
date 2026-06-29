# Machine Lease Management

EgoAdmin's IDGen component provides distributed machine leases for unique worker ID coordination in the Snowflake algorithm. Each IDGen process acquires a machine ID lease at startup, maintains it through background renewal, and releases it gracefully on shutdown.

## Overview

Machine lease management is implemented by `ProcessMachineLeaseManager`, which handles the complete lease lifecycle for a single process. The system supports two allocator backends:

| Allocator | Package Path | Use Case |
|-----------|-------------|----------|
| Redis allocator | `machine/redis` | Small-to-medium deployments using Redis distributed locks |
| gRPC allocator | `machine/grpcallocator` | Large-scale deployments with remote management via IDGen service |

| Config Key | Description |
|------------|-------------|
| `group` | Lease group name; processes in the same group share the machine ID space |
| `maxMachineID` | Maximum machine ID, range `[0, maxMachineID]` |
| `ttl` | Lease TTL; reclaimed automatically after expiration |
| `renewInterval` | Renewal interval; must be less than TTL |
| `lostPolicy` | Lease loss policy: `fail_closed` (refuse service) or `degraded` (continue) |

::: tip Default Policy
The default policy is `fail_closed`. When a lease is lost, ID generation is refused to prevent duplicate IDs caused by Snowflake machine ID conflicts.
:::

## Core Usage

### Lease Lifecycle

```text
Process startup
    |
    v
Start() -> AllocateLease (acquire machine ID)
    |
    v
[Background cron renewal: cron.idgen.machine.renew]
    |
    v
Renew() -> Update TTL, extend lease
    |
    v
Process shutdown
    |
    v
Stop() / StopWithoutRelease() -> Release lease / Wait for TTL expiration
```

### Lease Manager Interface

```go
type MachineLeaseManager interface {
    Start(ctx context.Context) error       // Start, acquire lease
    Stop(ctx context.Context) error        // Stop, release remote lease
    Renew(ctx context.Context) error       // Renew lease
    Lease() (MachineLease, bool)           // Get current lease
    Health(ctx context.Context) error      // Health check
}
```

### Lease Data Structure

```go
type MachineLease struct {
    Namespace     string        // Lease namespace
    InstanceID    string        // Instance identifier (hostname:pid or stable ID)
    SessionID     string        // Session ID (random, different each allocation)
    MachineID     int           // Allocated machine ID
    TTL           time.Duration // Lease TTL
    RenewInterval time.Duration // Renewal interval
    ExpiresAt     time.Time     // Expiry time
}
```

### Graceful Shutdown

Use `StopWithoutRelease` to stop renewal without calling the remote release endpoint. This avoids noisy errors when all services are stopping simultaneously and the IDGen service is already unavailable:

```go
// Register in configureShutdown
opts.shutdown.RegisterFunc("idgen-machine", func(ctx context.Context) error {
    return opts.idm.StopWithoutRelease(ctx)
})
```

The lease is automatically reclaimed after TTL expiration, and the machine ID becomes available for other processes.

## Configuration Examples

### Basic Configuration

```toml
# Machine lease configuration
[component.idgen.machine]
group = "egoadmin-local"           # Lease group name; processes in the same group share the machine ID space
maxMachineID = 1023                # Maximum machine ID (range [0, 1023])
ttl = "60s"                        # Lease TTL
renewInterval = "10s"              # Renewal interval (must be less than TTL)
renewTimeout = "5s"                # Renewal timeout
minRenewWindows = 5                # Minimum renewal windows
reallocateBackoff = "2s"           # Backoff after allocation failure
stableInstanceID = ""              # Stable instance ID; defaults to hostname:pid
lostPolicy = "fail_closed"         # Lease loss policy
```

### Minimal Configuration

```toml
[component.idgen.machine]
group = "egoadmin-local"
```

::: tip Defaults
Unconfigured fields use the defaults from `DefaultMachineConfig()`: `maxMachineID = 1023`, `ttl = 60s`, `renewInterval = 10s`, `lostPolicy = "fail_closed"`.
:::

### Default Values Quick Reference

| Config Key | Default | Description |
|------------|---------|-------------|
| `group` | `"default"` | Lease group name |
| `maxMachineID` | `1023` | Maximum machine ID |
| `ttl` | `60s` | Lease TTL |
| `renewInterval` | `10s` (auto-calculated) | Renewal interval, TTL/3 |
| `renewTimeout` | `5s` | Renewal timeout |
| `minRenewWindows` | `5` | Minimum renewal windows |
| `reallocateBackoff` | `2s` | Reallocation backoff |
| `lostPolicy` | `"fail_closed"` | Lease loss policy |

### Instance ID Strategy

| Scenario | Recommended Configuration |
|----------|--------------------------|
| Local development | Leave `stableInstanceID` empty; uses `hostname:pid` |
| Docker Compose | Use the container name |
| Kubernetes | Use the Pod name (`metadata.name`) |

## Real-World Examples

### Redis Allocator

The Redis allocator uses Lua scripts for atomic machine ID allocation. On process startup, it attempts to acquire a free machine ID via `SET NX PX`:

```go
import "github.com/egoadmin/egoadmin/internal/component/idgen/machine/redis"

// Create Redis allocator
allocator := redis.New(redisClient, redis.WithKeyPrefix("idgen"))

// Create lease manager
manager, err := idgen.NewMachineLeaseManager(
    "component.idgen.machine",
    config,
    allocator,
    logger,
)

// Start
err := manager.Start(ctx)
```

Redis key structure:

```text
idgen:{namespace}:machine:id:{machineID}     -> "{instanceID}|{sessionID}"  (TTL)
idgen:{namespace}:machine:instance:{instanceID} -> "{machineID}|{sessionID}"  (TTL)
```

### gRPC Allocator

The gRPC allocator manages leases remotely through the IDGen service, suitable for large-scale deployments:

```go
import "github.com/egoadmin/egoadmin/internal/component/idgen/machine/grpcallocator"

// Injected via Wire
allocator := grpcallocator.New(machineLeaseClient)

manager, err := idgen.NewMachineLeaseManager(
    "component.idgen.machine",
    config,
    allocator,
    logger,
)
```

### Background Renewal

Renewal runs automatically via an EGO cron job (`cron.idgen.machine.renew`):

```go
func NewMachineLeaseRenewCron(manager MachineLeaseManager) ecron.Ecron {
    return ecron.Load("cron.idgen.machine.renew").Build(
        ecron.WithJob(func(ctx context.Context) error {
            if manager == nil {
                return nil
            }
            return manager.Renew(ctx)
        }),
    )
}
```

Renewal logic:

1. Check if the current lease is valid (not expired).
2. Call the allocator's `Renew` method.
3. On success: update local `ExpiresAt`, reset failure counter.
4. On failure: based on `lostPolicy`, either degrade or mark lease as lost.
5. Renewal failure beyond TTL: attempt to reallocate a new machine ID.

## How It Works

### Redis Allocator Lua Script

The allocation script (`allocateScript`) has atomic logic:

```lua
-- 1. Check if instance already has an unexpired lease
local instanceKey = keyPrefix .. ":" .. namespace .. ":machine:instance:" .. instanceID
local existing = redis.call("GET", instanceKey)
if existing then
    -- Reuse existing machine ID, update sessionID
    redis.call("PSETEX", machineKey, ttlMillis, value)
    redis.call("PSETEX", instanceKey, ttlMillis, machineID .. "|" .. sessionID)
    return { existingID, sessionID }
end

-- 2. Iterate [0, maxMachineID], try SET NX PX to acquire a free ID
for id = 0, maxMachineID do
    local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. id
    if redis.call("SET", machineKey, value, "PX", ttlMillis, "NX") then
        redis.call("PSETEX", instanceKey, ttlMillis, id .. "|" .. sessionID)
        return { id, sessionID }
    end
end

-- 3. All IDs are occupied
return { -1, sessionID }
```

The renewal script (`renewScript`) validates the `instanceID|sessionID` value then updates TTL; the release script (`releaseScript`) validates then deletes the keys. All operations are atomic via Lua scripts.

### MySQL Allocator

The IDGen server uses MySQL to manage machine leases (`idgen_machine_lease` table). Allocation logic:

1. **Reuse existing lease**: Query for an unexpired lease from the same `instanceID`; reuse if found.
2. **Claim expired ID**: Iterate `[0, maxMachineID]`, using `SELECT ... FOR UPDATE` to lock rows. If expired, update for the current instance.
3. **Create new ID**: If the row does not exist, create it directly.
4. **Overflow**: All IDs are occupied and not expired, returns `ErrMachineIDOverflow`.

```go
// Find reusable instance lease
existing, err := r.findReusableInstanceLease(ctx, req, now)
if existing != nil {
    // Update sessionID and expiry
    return modelToLease(existing), nil
}

// Iterate machine IDs
for id := int32(0); id <= int32(req.MaxMachineID); id++ {
    // SELECT ... FOR UPDATE
    row := machineLeaseModel{}
    db.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("namespace = ? AND machine_id = ?", req.Namespace, id).
        First(&row)

    if row.ExpiresAt.After(now) {
        continue  // Not expired, skip
    }
    // Claim expired ID
    row.InstanceID = req.InstanceID
    row.SessionID = sessionID
    row.ExpiresAt = now.Add(req.TTL)
    // UPDATE ...
    return modelToLease(&row), nil
}
return ErrMachineIDOverflow
```

### Renewal Error Handling

When renewal fails:

```text
Renew() failed
    |
    v
ErrMachineLeaseLost? ----yes----> Mark lease lost, attempt reallocate
    |
    no
    |
    v
Is lease still valid (not expired)? --yes--> Warning log, keep current lease
    |
    no
    |
    v
Mark lease lost, attempt reallocate
```

Key design: When renewal fails but the lease has not expired, it is not immediately marked as lost. This avoids unnecessary degradation caused by transient network issues.

### Backoff on Reallocation

When reallocation fails, `reallocateBackoff` prevents high-frequency retries:

```go
func (m *ProcessMachineLeaseManager) reallocate(ctx context.Context) error {
    now := time.Now()
    if next := m.nextAllocateAt.Load(); next > 0 && now.Before(time.Unix(0, next)) {
        return ErrMachineLeaseLost  // Within backoff period, do not retry
    }
    if err := m.allocate(ctx); err != nil {
        m.nextAllocateAt.Store(now.Add(m.config.ReallocateBackoff).UnixNano())
        return err
    }
    return nil
}
```

### Conflict Resolution

When a lease expires, the machine ID is freed for reuse:

- **Redis**: Keys expire automatically via TTL; `SET NX PX` ensures atomicity.
- **MySQL**: Expiration is checked via `expires_at`; `SELECT ... FOR UPDATE` ensures transaction safety.

SessionID distinguishes different lease periods for the same machine ID. Renewal validates the `sessionID`; a mismatch means the lease has been claimed by another process.

### TTL and Renewal Window

Configuration validation ensures sufficient renewal windows:

```text
TTL >= (minRenewWindows + 1) * renewInterval
```

Example: `ttl = 60s`, `renewInterval = 10s`, `minRenewWindows = 5`:
- TTL covers 6 renewal opportunities (60s / 10s = 6).
- At least 5 renewal windows (`minRenewWindows = 5`).
- Even with 5 consecutive failures, the 6th attempt can still succeed.

### Failure Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `fail_closed` (default) | Refuse ID generation when lease is lost | Strong consistency requirements |
| `degraded` | Continue using allocated segments when lease is lost | Availability-first |

::: warning fail_closed is the Default
In `fail_closed` mode, a lost lease causes all `Next()` and `Reserve()` calls to return `ErrMachineLeaseLost`. This is intentional to ensure Snowflake ID uniqueness.
:::

### Expired Lease Cleanup

The IDGen server cleans up expired machine lease records via a cron job:

```go
const (
    machineCleanupCronName         = "cron.idgen.machine.cleanup"
    defaultMachineCleanupLimit     = 1000
    defaultMachineCleanupRetention = 7 * 24 * time.Hour  // Keep 7 days
)
```

Configurable via:

```toml
# Custom cleanup policy
[idgen]
machineLeaseCleanupRetention = "72h"
machineLeaseCleanupLimit = 500
```

### Prometheus Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `idgen_machine_lease_renew_total` | Counter | component, status | Renewal operation count |
| `idgen_machine_lease_status` | Gauge | component | Lease status (1=valid, 0=lost) |
| `idgen_health_status` | Gauge | component | Component health status |

## Common Issues

### Machine ID Collision

::: warning
Ensure each worker process has a unique machine lease.
:::

**Cause**: Multiple processes share the same `stableInstanceID`, or old leases in Redis/MySQL have not expired properly.

**Diagnosis**:

```bash
# Check machine ID occupancy in Redis
redis-cli KEYS "idgen:*:machine:id:*"

# Check active leases in MySQL
SELECT namespace, machine_id, instance_id, session_id, expires_at
FROM idgen_machine_lease
WHERE expires_at > NOW()
ORDER BY namespace, machine_id;
```

**Resolution**: Use unique `stableInstanceID` in container environments (e.g., Kubernetes Pod name).

### Lease Renewal Failure

**Symptoms**: Logs show `idgen machine lease lost`.

**Diagnosis**:

- Is the Redis or MySQL connection healthy?
- Is the `cron.idgen.machine.renew` cron job running normally?
- Is `renewTimeout` sufficient (default 5 seconds)?
- Is `renewInterval` less than TTL?

```bash
# Check Redis connectivity
redis-cli PING

# Check MySQL connectivity
mysql -h 127.0.0.1 -u egoadmin -e "SELECT 1"
```

### ID Uniqueness After Lease Loss

In `LostPolicy = "degraded"` mode, segment ID generation continues after lease loss (segments do not depend on machine ID), but Snowflake mode cannot continue generating IDs.

If strong uniqueness is required, use the default `fail_closed` policy.

### Clock Drift

::: danger
The Snowflake algorithm is sensitive to system clock. Clock rollback may produce duplicate IDs.
:::

**Diagnosis**:

```bash
# Check NTP synchronization status
timedatectl status
ntpq -p
```

**Mitigation**: Segment mode (EgoAdmin's default) is unaffected by clock drift, as it does not rely on the system clock to generate IDs.

### JavaScript Precision Loss

::: warning
Go `int64` / `uint64` values exceeding JavaScript's `Number.MAX_SAFE_INTEGER` (2^53 - 1) will lose precision.
:::

**Solution**: Use idcodec to encode as string, or define ID fields as `string` in proto/JSON.

### All Machine IDs Occupied

**Symptoms**: `ErrMachineIDOverflow`.

**Cause**: All machine IDs in `[0, maxMachineID]` are occupied by unexpired leases.

**Diagnosis**:

- Check for zombie processes holding leases (process crashed but TTL not expired).
- Increase `maxMachineID` (default 1023).
- Reduce `ttl` for faster expiration and reclamation.
- Verify the `CleanupExpired` cron job is running normally.

### Graceful Shutdown Patterns

```go
// Recommended: use StopWithoutRelease
// Avoids release failure noise when all services stop simultaneously
opts.shutdown.RegisterFunc("idgen-machine", func(ctx context.Context) error {
    return opts.idm.StopWithoutRelease(ctx)
})

// If immediate release is needed (e.g., exclusive test environment)
err := machineManager.Stop(ctx)
```

`StopWithoutRelease` stops renewal without calling the remote release endpoint. The lease is automatically reclaimed after TTL expiration. `Stop` attempts to call the remote release endpoint, suitable for exclusive environments needing fast reclamation.

## Reference Links

- `internal/component/idgen/machine_manager.go` -- ProcessMachineLeaseManager implementation (Allocate, Renew, Release, StopWithoutRelease)
- `internal/component/idgen/machine_cron.go` -- ecron renewal cron job
- `internal/component/idgen/shutdown.go` -- StopMachineLeaseBestEffort graceful shutdown
- `internal/component/idgen/machine/redis/allocator.go` -- Redis allocator implementation
- `internal/component/idgen/machine/redis/scripts.go` -- Redis Lua scripts (allocate, renew, release)
- `internal/component/idgen/machine/grpcallocator/allocator.go` -- gRPC allocator implementation
- `internal/component/idgen/interface.go` -- MachineAllocator, MachineLeaseManager interface definitions
- `internal/component/idgen/config.go` -- MachineConfig struct and defaults
- `internal/app/idgen/adapter/persistence/mysql/machine_repository.go` -- MySQL machine lease repository
- `internal/app/idgen/adapter/persistence/mysql/machine_model.go` -- Database model
- `internal/app/idgen/server/cron.go` -- Expired lease cleanup cron job
- `internal/platform/idgen/provider.go` -- Platform-layer Wire assembly
