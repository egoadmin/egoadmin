# Performance Issues

This page explains how to diagnose and resolve runtime performance bottlenecks in EgoAdmin, covering CPU, memory, database, cache, and network latency scenarios.

## Overview

Performance issues in EgoAdmin typically manifest as increased request latency, abnormal CPU usage, continuously growing memory, or slow database responses. The core troubleshooting approach is to quantify metrics first, then locate the bottleneck, and finally optimize.

Recommended troubleshooting path:

1. Use pprof to capture CPU / memory / goroutine snapshots.
2. Check database slow query logs and GORM debug output.
3. Review Redis slow logs and connection pool status.
4. Use Jaeger distributed tracing to locate cross-service latency.

::: tip
EgoAdmin automatically registers pprof handlers through the EGO framework, exposed on the governor port (e.g., `:9103`, `:9203`). Accessible in development; restrict access in production.
:::

## Core Usage

### CPU Profiling

When CPU usage is abnormally high, capture a CPU profile to locate hotspot functions.

```bash
# Capture 30-second CPU profile (interactive mode)
go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

# After entering the interactive shell, common commands:
# top         - View functions with highest CPU usage
# top20       - View top 20 hotspots
# list <func> - View CPU cost of a specific function
# web         - Generate visual call graph (requires graphviz)
```

View flame graph in the pprof interactive shell:

```bash
# Start HTTP server for flame graph viewing
go tool pprof -http=:8080 http://localhost:9103/debug/pprof/profile?seconds=30
```

::: warning
CPU profiling incurs 5%-10% overhead. In production, always specify the `seconds` parameter to avoid long captures.
:::

### Heap Memory Analysis

When memory grows continuously, compare heap profiles at different points in time to locate leaks.

```bash
# Get current heap memory profile
go tool pprof http://localhost:9103/debug/pprof/heap

# Common interactive commands:
# top           - Functions with most memory allocations
# inuse_space   - Memory currently in use
# alloc_space   - Cumulative allocated memory (including GC'd)
# inuse_objects - Currently live object count
```

Compare heap profiles from two points in time to detect memory leaks:

```bash
# Time point 1: save heap profile
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.1

# Wait a few minutes or perform some operations

# Time point 2: save heap profile
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.2

# Compare differences
go tool pprof -base /tmp/heap.1 /tmp/heap.2
```

### Goroutine Analysis

Abnormal goroutine count growth usually indicates leaks or blocking.

```bash
# View goroutine stacks
go tool pprof http://localhost:9103/debug/pprof/goroutine

# View goroutine text stacks directly via HTTP
curl http://localhost:9103/debug/pprof/goroutine?debug=1

# View goroutine count overview
curl http://localhost:9103/debug/pprof/goroutine?debug=2
```

::: info
A healthy EgoAdmin service typically has 50-200 goroutines when idle. If it consistently exceeds 1000, investigate potential goroutine leaks.
:::

### Flame Graphs

Flame graphs visually show the time distribution of function call stacks.

```bash
# Launch interactive Web UI for flame graph viewing
go tool pprof -http=:8080 http://localhost:9103/debug/pprof/profile?seconds=30

# Open http://localhost:8080 in browser, then select the Flame Graph view
# You can also view Top, Graph, Peek, and other views
```

## Configuration Examples

### Database Slow Query Debugging

GORM provides a debug mode that prints all SQL statements with execution time.

```toml
# configs/user/config.toml
[client.mysql]
debug = true   # Enable GORM debug mode, prints all SQL
```

Sample log output when enabled:

```text
[0.234ms] [rows:10] SELECT * FROM `sys_user` WHERE `status` = 1 AND `deleted_at` IS NULL LIMIT 20
[1523.456ms] [rows:1] SELECT * FROM `sys_dept` WHERE id IN (1,2,3,4,5)
```

::: warning
`debug = true` outputs all SQL statements to logs. Always disable it in production to avoid excessive log volume and sensitive information leaks.
:::

### Redis Slow Query Debugging

```toml
# configs/user/config.toml
[client.redis]
debug = true   # Enable Redis command logging
```

When enabled, you can observe the execution time of each Redis command. Combined with Redis server-side slow log threshold:

```bash
# View current Redis slow log threshold (microseconds)
redis-cli CONFIG GET slowlog-log-slower-than

# View last 10 slow log entries
redis-cli SLOWLOG GET 10
```

### Connection Pool Configuration

Improper connection pool settings lead to connection exhaustion or frequent connection creation.

```toml
# configs/user/config.toml
[client.mysql]
maxOpenConns = 100          # Maximum open connections
maxIdleConns = 10           # Maximum idle connections
connMaxLifetime = "3600s"   # Maximum connection lifetime
connMaxIdleTime = "300s"    # Maximum idle connection lifetime
```

```toml
# configs/user/config.toml
[client.redis]
poolSize = 100              # Redis connection pool size
minIdleConns = 10           # Minimum idle connections
```

::: tip
Connection pool size should be set based on actual concurrency. Too small a `maxOpenConns` causes requests to queue waiting for connections; too large may exhaust the database connection limit.
:::

## Real-World Examples

### Example 1: N+1 Queries Causing Slow API Response

**Symptom**: User list API responds in over 2 seconds, but individual database queries are fast.

**Diagnosis**:

After enabling GORM debug mode, found that each user record triggers a department query:

```text
[0.5ms] [rows:20] SELECT * FROM sys_user WHERE status=1 LIMIT 20
[0.3ms] [rows:1] SELECT * FROM sys_dept WHERE id = 1
[0.3ms] [rows:1] SELECT * FROM sys_dept WHERE id = 2
... (repeated 20 times)
```

**Solution**:

Use `Preload` or `Joins` to batch-load related data:

```go
// Wrong: loop query (N+1)
for _, user := range users {
    db.First(&dept, user.DeptID)
}

// Correct: Preload batch load
var users []User
db.Preload("Dept").Where("status = ?", 1).Limit(20).Find(&users)

// Or use Joins (suitable when you only need partial fields)
db.Joins("Dept").Where("sys_user.status = ?", 1).Limit(20).Find(&users)
```

### Example 2: Continuous Memory Growth

**Symptom**: Service RSS memory grows continuously over hours, eventually killed by OOM killer.

**Diagnosis**:

```bash
# Compare heap profiles at two points in time
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.before
# Wait 10 minutes
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.after
go tool pprof -base /tmp/heap.before /tmp/heap.after
```

Pprof revealed a cache map that only writes and never deletes, growing indefinitely.

**Solution**:

Introduce an LRU cache or set TTLs to ensure expired data is cleaned up:

```go
// Replace unbounded map with a size-limited cache
// See samber/hot or github.com/hashicorp/golang-lru
```

### Example 3: Goroutine Leak

**Symptom**: Goroutine count rises continuously over time, with many goroutine blocking stacks in logs.

**Diagnosis**:

```bash
# View goroutine count trend
curl -s http://localhost:9103/debug/pprof/goroutine?debug=1 | head -30

# Count goroutines by function
curl -s http://localhost:9103/debug/pprof/goroutine?debug=2 | grep -c "^goroutine"
```

**Solution**:

Use `goleak` to detect goroutine leaks in tests:

```go
import "go.uber.org/goleak"

func TestSomething(t *testing.T) {
    defer goleak.VerifyNone(t)
    // test code
}
```

::: tip
`go.uber.org/goleak` checks for leaked goroutines when a test finishes. Recommended to enable in CI for all packages.
:::

### Example 4: Redis Hot Key

**Symptom**: Certain Redis commands show latency spikes, Jaeger traces reveal abnormal Redis operation times.

**Diagnosis**:

```bash
# View Redis slow logs
redis-cli SLOWLOG GET 20

# View current hot keys (requires Redis 4.0+)
redis-cli --hotkeys
```

**Solution**:

- Add local caching for hot keys to reduce Redis access frequency.
- Split large keys into multiple smaller keys.
- Use pipeline for batch operations to reduce RTT.

### Example 5: Cross-Service Call Latency

**Symptom**: An API is overall slow to respond, but each individual service processes quickly.

**Diagnosis**:

Jaeger distributed tracing showed that the gateway -> user gRPC call took too long:

```bash
# Open Jaeger UI
open http://localhost:16686

# Search for gateway service traces, find the span with highest latency
```

**Solution**:

Check gRPC client configuration:

```toml
# configs/gateway/config.toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"       # Read timeout
dialTimeout = "3s"       # Connection timeout
```

::: warning
Too large `readTimeout` and `dialTimeout` cause requests to wait excessively; too small leads to false timeouts. Set reasonable values based on P99 latency.
:::

## How It Works

EgoAdmin's performance monitoring is based on the EGO framework's governor mechanism. Governor is a separate HTTP server responsible for exposing pprof, health check, metrics, and other operational endpoints.

```text
Request flow:
Client → Gateway HTTP → Gateway gRPC → User gRPC → MySQL/Redis
         ↓ governor       ↓ governor
         :9101/:9001      :9103        :9203
         pprof/healthz    pprof/healthz
```

Each service's governor port is defined in the configuration file:

```toml
[server.govern]
host = "0.0.0.0"
port = 9103
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
```

## Common Issues

### pprof Endpoint Unreachable

**Symptom**: `curl http://localhost:9103/debug/pprof/` returns connection refused.

**Diagnosis**:

```bash
# Check if governor port is listening
netstat -tlnp | grep 9103

# Check govern port in config
grep "govern" configs/user/config.toml
```

**Solution**:

Confirm the service has started and the governor port is correct. Gateway, user, and idgen each use their own governor port.

### Production pprof Security

::: danger
Production pprof endpoints contain sensitive runtime information (memory contents, goroutine stacks). Access must be restricted. Recommendations:
- Only access governor ports through internal network.
- Use firewalls or reverse proxies to restrict access origins.
- Or disable governor in production configuration.
:::

### Performance-Related Lint Checks

Running `make lint` checks for common performance issues:

```bash
# Common performance-related lint rules
# - prealloc: suggests pre-allocating slice capacity
# - unconvert: unnecessary type conversions
# - unparam: unused function parameters
make lint
```

## Reference Links

- [Go pprof Official Documentation](https://pkg.go.dev/net/http/pprof)
- [Go Blog: Profiling Go Programs](https://go.dev/blog/pprof)
- [GORM Performance Optimization](https://gorm.io/docs/performance.html)
- [goleak - Goroutine Leak Detection](https://github.com/uber-go/goleak)
- [Jaeger Distributed Tracing](https://www.jaegertracing.io/)
- [Redis Slow Log Documentation](https://redis.io/docs/latest/commands/slowlog/)
