# ERedis Cache Component

ERedis wraps go-redis into an EGO component, supporting Cluster, Standalone (Stub), Sentinel, and Ring modes. It provides a unified `Cmdable` interface, a distributed lock client, and debug/trace interceptors for observability.

## Overview

All Redis access in EgoAdmin goes through the `internal/component/eredis` package. The component automatically selects the appropriate go-redis client type at startup based on configuration and injects observability interceptors (logging, metrics, distributed tracing), so business code does not need to deal with underlying connection details.

Core capabilities:

- **Four deployment modes**: `stub` (standalone), `cluster`, `sentinel`, and `ring` (sharded ring).
- **Unified interface**: All modes expose `redis.Cmdable`, giving business layers a consistent API.
- **Distributed lock**: Built-in lock client based on `SET NX PX` + Lua scripts, with configurable retry strategies.
- **Observability interceptors**: Debug logs, structured access logs, Prometheus metrics, OpenTelemetry tracing.
- **Connection pool monitoring**: Exposes pool metrics via the governor endpoint `/debug/redis/stats`, with Prometheus gauges auto-collected every 10 seconds.
- **Convenience methods**: The Component itself wraps common operations like `Get`, `Set`, `HGet`, `HSet`, `Del`, `Incr`, `ZAdd`, `LPush`, `SAdd`, and more.

## Core Usage

### Loading and Building the Component

Use `eredis.Load` to read parameters from configuration, then call `Build` to construct the component instance:

```go
redisComp := eredis.Load("client.redis").Build()
```

In the Wire injection system, the component is automatically injected as part of Options:

```go
type Options struct {
    Redis *eredis.Component
}

// Get the unified Cmdable interface
client := s.Redis.Client()
```

### Mode Selection

After startup, you can query the current mode via `Mode()` or use mode-specific accessors:

```go
// Query mode
mode := s.Redis.Mode() // "stub" | "cluster" | "sentinel" | "ring"

// Mode-specific accessors (return nil if mode does not match)
cluster := s.Redis.Cluster()   // *redis.ClusterClient
stub    := s.Redis.Stub()      // *redis.Client
sentinel := s.Redis.Sentinel() // *redis.Client
ring    := s.Redis.Ring()      // *redis.Ring
```

::: warning Mode Accessor Safety
`Cluster()` returns `nil` in stub mode rather than panicking. Always check the return value or verify `Mode()` first.
:::

### Common Data Operations

The component provides convenience methods covering String, Hash, List, Set, ZSet, and Geo, all delegating to `redis.Cmdable`:

```go
// String
err := s.Redis.Set(ctx, "key", "value", time.Hour)
val, err := s.Redis.Get(ctx, "key")
err := s.Redis.SetNX(ctx, "lock:resource", "token", 10*time.Second)
cnt, err := s.Redis.Del(ctx, "key1", "key2")

// Hash
err := s.Redis.HSet(ctx, "user:1", "name", "John")
val, err := s.Redis.HGet(ctx, "user:1", "name")
m, err := s.Redis.HGetAll(ctx, "user:1")

// List
cnt, err := s.Redis.LPush(ctx, "queue", "task1", "task2")
val, err := s.Redis.RPop(ctx, "queue")
items, err := s.Redis.LRange(ctx, "queue", 0, -1)

// Set
cnt, err := s.Redis.SAdd(ctx, "tags", "go", "redis")
members, err := s.Redis.SMembers(ctx, "tags")

// ZSet
cnt, err := s.Redis.ZAdd(ctx, "leaderboard", redis.Z{Score: 100, Member: "alice"})
items, err := s.Redis.ZRange(ctx, "leaderboard", 0, 9)

// Counter
n, err := s.Redis.Incr(ctx, "page:views")
n, err := s.Redis.IncrBy(ctx, "counter", 5)
```

You can also use the `redis.Cmdable` interface directly for any command:

```go
client := s.Redis.Client()
client.Set(ctx, "key", "value", time.Hour)
client.Get(ctx, "key").Result()
client.Del(ctx, "key1", "key2").Err()
```

### Distributed Lock

ERedis includes a built-in distributed lock client based on Redis `SET NX PX` and atomic Lua scripts:

```go
lockClient := s.Redis.LockClient()

// Acquire lock (no retry by default)
lock, err := lockClient.Obtain(ctx, "user:add", 5*time.Second)
if errors.Is(err, eredis.ErrNotObtained) {
    // Lock is held by another process
    return fmt.Errorf("resource locked")
}
if err != nil {
    return err
}
defer lock.Release(ctx)

// Critical section
// ...
```

#### Retry Strategies

Lock acquisition supports pluggable retry strategies:

```go
import "internal/component/eredis"

// No retry (default)
lock, err := lockClient.Obtain(ctx, "key", ttl)

// Fixed interval retry
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(eredis.LinearBackoffRetry(100*time.Millisecond)),
)

// Exponential backoff retry (min 16ms, max 1s)
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(eredis.ExponentialBackoffRetry(16*time.Millisecond, time.Second)),
)

// Limited retry count
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(
        eredis.LimitRetry(eredis.LinearBackoffRetry(100*time.Millisecond), 5),
    ),
)
```

#### Lock Lifecycle Management

```go
lock, err := lockClient.Obtain(ctx, "key", 10*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx)

// Query remaining TTL
remaining, err := lock.TTL(ctx)

// Refresh (only succeeds if the lock is still held)
err = lock.Refresh(ctx, 20*time.Second)

// Get the lock's key and token
key := lock.Key()
token := lock.Token()
```

### Build Options

The `Build` method supports functional options to override configuration at build time:

```go
redisComp := eredis.Load("client.redis").Build(
    eredis.WithStub(),              // Force stub mode
    eredis.WithAddr("127.0.0.1:6380"),
    eredis.WithPassword("secret"),
    eredis.WithDB(1),
    eredis.WithPoolSize(50),
)
```

Available options:

| Option | Description |
|--------|-------------|
| `WithStub()` | Set mode to stub |
| `WithCluster()` | Set mode to cluster |
| `WithSentinel()` | Set mode to sentinel |
| `WithRing()` | Set mode to ring |
| `WithAddr(addr)` | Set standalone address |
| `WithAddrs(addrs)` | Set cluster/sentinel/ring address list |
| `WithPassword(p)` | Set password |
| `WithDB(n)` | Set database number |
| `WithPoolSize(n)` | Set connection pool size |
| `WithMasterName(name)` | Set sentinel master node name |

## Configuration Examples

### Standalone (Stub) Mode

```toml
[client.redis]
addr = "127.0.0.1:6380"
mode = "stub"
password = "egoadmin"
db = 0
debug = true
```

### Cluster Mode

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "cluster"
password = "egoadmin"
readOnly = true
```

### Sentinel Mode

```toml
[client.redis]
addrs = ["sentinel-1:26379", "sentinel-2:26379", "sentinel-3:26379"]
mode = "sentinel"
masterName = "mymaster"
password = "egoadmin"
sentinelPassword = "sentinel-pass"
db = 0
```

### Ring Mode

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "ring"
password = "egoadmin"
db = 0
```

### Full Configuration Reference

```toml
[client.redis]
addr = "127.0.0.1:6380"           # Standalone mode address
addrs = []                         # Cluster/sentinel/ring address list
mode = "stub"                      # stub | cluster | sentinel | ring
masterName = ""                    # Sentinel master node name
password = ""                      # Connection password
sentinelUsername = ""              # Sentinel mode username
sentinelPassword = ""              # Sentinel mode password
db = 0                             # Database number (not applicable in cluster mode)
poolSize = 20                      # Max connections per node
maxRetries = 0                     # Max retries on network errors
minIdleConns = 4                   # Minimum idle connections
dialTimeout = "1s"                 # Connection timeout
readTimeout = "1s"                 # Read timeout
writeTimeout = "1s"                # Write timeout
idleTimeout = "60s"                # Max connection idle time
readOnly = false                   # Cluster mode read from replicas
debug = false                      # Debug log toggle
slowLogThreshold = "250ms"         # Slow log threshold
onFail = "panic"                   # Startup failure behavior: panic | error

# Interceptors
enableMetricInterceptor = true     # Prometheus metrics
enableTraceInterceptor = true      # OpenTelemetry tracing
enableAccessInterceptor = false    # Structured access log
enableAccessInterceptorReq = false # Log request parameters
enableAccessInterceptorRes = false # Log response parameters

# TLS
[client.redis.authentication]
# TLS configuration (see authentication.go)
```

### JetCache Integration

ERedis works with JetCache to provide multi-level caching (local LRU + remote Redis):

```toml
[client.jetcache]
name = "default"
remoteExpiry = "1h0m0s"
localSize = 256
localExpiry = "1m0s"
refreshDuration = "2m0s"
stopRefreshAfter = "1h0m0s"
notFoundExpiry = "1m0s"
enableMetrics = true
enableSyncLocal = false
codec = "msgpack"
```

Use cases: permission snapshots, user basic info, high-frequency reads where short-lived staleness is acceptable.

## Real-World Examples

### Injecting and Using ERedis in a Service

```go
// internal/app/user/server/options.go
type Options struct {
    Redis    *eredis.Component
    JetCache *ejetcache.Component
}

// internal/app/user/server/service.go
type UserService struct {
    opts Options
}

func (s *UserService) GetUserProfile(ctx context.Context, userID uint64) (*Profile, error) {
    key := fmt.Sprintf("user:profile:%d", userID)

    // Try cache
    data, err := s.opts.Redis.GetBytes(ctx, key)
    if err == nil {
        var p Profile
        if err := json.Unmarshal(data, &p); err == nil {
            return &p, nil
        }
    }

    // Cache miss, load from database
    profile, err := s.loadFromDB(ctx, userID)
    if err != nil {
        return nil, err
    }

    // Write to cache
    bytes, _ := json.Marshal(profile)
    _ = s.opts.Redis.Set(ctx, key, bytes, 30*time.Minute)

    return profile, nil
}
```

### Duplicate Submission Prevention

```go
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderReq) error {
    lockKey := fmt.Sprintf("lock:order:create:%d", req.UserID)
    lock, err := s.Redis.LockClient().Obtain(ctx, lockKey, 10*time.Second,
        eredis.WithLockOptionRetryStrategy(eredis.LinearBackoffRetry(50*time.Millisecond)),
    )
    if errors.Is(err, eredis.ErrNotObtained) {
        return fmt.Errorf("duplicate submission")
    }
    if err != nil {
        return err
    }
    defer lock.Release(ctx)

    // Order creation logic
    return s.doCreateOrder(ctx, req)
}
```

### Rate Limiting Counter

```go
func (s *GatewayService) RateLimit(ctx context.Context, userID uint64, limit int64) (bool, error) {
    key := fmt.Sprintf("ratelimit:%d:%d", userID, time.Now().Unix()/60)
    n, err := s.Redis.Incr(ctx, key)
    if err != nil {
        return false, err
    }
    if n == 1 {
        // First access, set expiry
        _, _ = s.Redis.Expire(ctx, key, time.Minute)
    }
    return n <= limit, nil
}
```

## How It Works

### Initialization Flow

```
eredis.Load("client.redis")
    |
    v
Container.Build()
    |-- Parse config, determine mode
    |-- Inject interceptors in order:
    |     1. fixedInterceptor (basic error wrapping)
    |     2. debugInterceptor (only when debug=true)
    |     3. metricInterceptor (only when enableMetricInterceptor=true)
    |     4. accessInterceptor (only when enableAccessInterceptor=true)
    |     5. traceInterceptor (only when enableTraceInterceptor=true)
    |
    v
Create client based on mode:
    |-- cluster  -> redis.NewClusterClient (uses Addrs)
    |-- stub     -> redis.NewClient (uses Addr)
    |-- sentinel -> redis.NewFailoverClient (uses Addrs + MasterName)
    |-- ring     -> redis.NewRing (uses Addrs, auto-shards named shard1, shard2...)
    |
    v
Register Hook on the client (AddHook)
    |
    v
Execute Ping connectivity test
    |-- Success -> return Component
    |-- Failure -> panic or error based on onFail config
    |
    v
Create lockClient (shares underlying Cmdable)
    |
    v
Register to instances (for stats and monitoring)
```

### Interceptor Chain

ERedis leverages go-redis's `Hook` interface for interception. Each interceptor implements `ProcessHook` and `ProcessPipelineHook`, inserting logic before and after command execution:

| Interceptor | Trigger Condition | Purpose |
|-------------|-------------------|---------|
| `fixed` | Always enabled | Record start time, wrap non-NOSCRIPT errors |
| `debug` | `debug=true` AND `EGO_DEBUG=true` | Print command arguments, response, and latency to stdout |
| `metric` | `enableMetricInterceptor=true` | Record Prometheus histograms (latency) and counters (status) |
| `access` | `enableAccessInterceptor=true` or exceeds slow log threshold | Output structured JSON logs with request/response, latency, trace ID |
| `trace` | `enableTraceInterceptor=true` | Create OpenTelemetry span, inject net.peer, db.system attributes |

### Distributed Lock Mechanism

The lock uses three Lua scripts for atomicity:

| Script | Purpose | Redis Command |
|--------|---------|---------------|
| `SET NX PX` | Acquire lock | Atomic set value and TTL |
| `luaRefresh` | Refresh | `GET` + `PEXPIRE` (only when value matches) |
| `luaRelease` | Release | `GET` + `DEL` (only when value matches) |
| `luaPTTL` | Query remaining TTL | `GET` + `PTTL` (only when value matches) |

The lock value consists of a 16-byte random token (Base64-encoded) plus optional metadata. All Lua scripts use value comparison to prevent accidentally releasing another process's lock.

### Connection Pool Monitoring

The component registers a governor endpoint and background monitoring goroutine in `init()`:

- `GET /debug/redis/stats`: Returns connection pool statistics for all Redis instances (JSON).
- Background goroutine collects `PoolStats` every 10 seconds, writing to Prometheus gauges with labels including `hits`, `misses`, `timeouts`, `total_conns`, `idle_conns`, `stale_conns`.

## Common Issues

### Cluster() Returns nil

Calling `Cluster()` in stub mode returns `nil`; dereferencing it will panic. The correct approach is to check `Mode()` first:

```go
if s.Redis.Mode() == eredis.ClusterMode {
    cluster := s.Redis.Cluster()
    // Use cluster...
}
```

### Connection Pool Exhaustion

The default `poolSize=20` may be insufficient under high concurrency. Symptoms include request timeouts or `connection pool exhausted` errors. Increase the pool size and adjust `minIdleConns`:

```toml
[client.redis]
poolSize = 100
minIdleConns = 20
```

Also check for unclosed connections or long-running blocking commands (e.g., `KEYS *`).

### Lock Not Released

If a process crashes while holding a lock, the lock is automatically released after TTL expires. However, in normal paths you must explicitly release:

```go
lock, err := lockClient.Obtain(ctx, "key", 10*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx) // Always defer
```

Do not rely on TTL as the sole release mechanism -- TTL is only a safety net.

### Debug Enabled in Production

`debug = true` prints logs for every Redis command. Under high QPS this generates massive log output and severely impacts performance. Ensure `debug = false` in production.

If troubleshooting is needed, temporarily enable `enableAccessInterceptor` with `slowLogThreshold` to only log slow requests:

```toml
[client.redis]
debug = false
enableAccessInterceptor = true
enableAccessInterceptorReq = true
slowLogThreshold = "100ms"
```

### redis.Nil Error

`redis.Nil` is a special error returned by go-redis when a key does not exist -- it is not a real exception. ERedis's access interceptor downgrades it to WARN (not ERROR). In business code, check for key absence using `errors.Is(err, redis.Nil)`:

```go
val, err := s.Redis.Get(ctx, key)
if errors.Is(err, redis.Nil) {
    // Key does not exist, not an error
} else if err != nil {
    return err
}
```

### Sentinel Mode Configuration Missing

Sentinel mode requires both `addrs` (sentinel addresses) and `masterName` (master node name) to be configured. Missing either one will cause the component to panic on startup.

### Container Environment Address Configuration

Inside containers, you must use Docker Compose service DNS names, not `127.0.0.1`:

```toml
[client.redis]
addr = "redis:6380"
mode = "stub"
```

Same for cluster mode:

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "cluster"
```

## Reference Links

- `internal/component/eredis/component.go` -- Component definition and mode accessors
- `internal/component/eredis/config.go` -- Config struct and defaults
- `internal/component/eredis/container.go` -- Load / Build flow and four client builders
- `internal/component/eredis/option.go` -- Build functional options
- `internal/component/eredis/interceptor.go` -- Interceptor implementations (debug, metric, access, trace)
- `internal/component/eredis/comopnent_cmds.go` -- Component convenience methods (Get/Set/HGet/HSet, etc.)
- `internal/component/eredis/lock.go` -- Distributed lock implementation (Obtain/Release/Refresh/TTL)
- `internal/component/eredis/lock_retry.go` -- Retry strategies (Linear/Exponential/Limit/NoRetry)
- `internal/component/eredis/error.go` -- Error constants (ErrNotObtained, ErrLockNotHeld)
- `internal/component/eredis/stat.go` -- Connection pool monitoring and governor endpoint
