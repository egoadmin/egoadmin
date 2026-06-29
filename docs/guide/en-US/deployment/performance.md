# Performance Optimization

EgoAdmin provides multi-layer performance optimization strategies covering database connection pools, caching, concurrency control, gRPC tuning, and memory optimization.

## Overview

EgoAdmin's performance optimization spans five layers:

| Layer | Mechanism | Key Config |
|-------|-----------|------------|
| Database | Connection pool tuning, index optimization | `client.mysql` |
| Cache | JetCache local + remote dual-layer cache | `client.jetcache` |
| Concurrency | Redis distributed locks, async queue | `client.asyncq` |
| gRPC | Connection pooling, timeouts, retries | `client.grpc.*` |
| Memory | Preallocation, avoiding copies | Compiler lint rules |

## Core Usage

### Database Connection Pool

GORM connection pool parameters directly affect database throughput:

```bash
# Check current connection count
mysql> SHOW STATUS LIKE 'Threads_connected';

# View connection pool usage
mysql> SHOW PROCESSLIST;
```

### JetCache Caching

EgoAdmin uses JetCache for dual-layer caching (local memory + Redis). Configuration controls TTL, local cache size, and refresh strategy:

```toml
[client.jetcache]
name = "default"
# Remote cache (Redis) expiry
remoteExpiry = "1h0m0s"
# Local cache entry count
localSize = 256
# Local cache expiry
localExpiry = "1m0s"
# Async refresh interval
refreshDuration = "2m0s"
# Stop refresh window
stopRefreshAfter = "1h0m0s"
# Short TTL for cache misses
notFoundExpiry = "1m0s"
# Enable cache metrics
enableMetrics = true
# Disable local cache sync (enable for multi-instance as needed)
enableSyncLocal = false
# Serialization codec
codec = "msgpack"
```

::: tip Cache Layer Design
Local cache (local) provides microsecond-level reads, suitable for high-frequency reads where consistency is not extremely critical. Remote cache (Redis) provides cross-instance consistency. JetCache defaults to checking local first, then remote, then database.
:::

### Async Task Queue

EgoAdmin uses asyncq, a Redis-based async task queue:

```toml
[client.asyncq]
redisAddr = "127.0.0.1:6379"
redisPassword = "egoadmin"
redisDB = 0
enableClient = true
enableServer = false
# Worker goroutine count
concurrency = 10
# Queue priority configuration
queues = { critical = 6, default = 3, low = 1 }
# Maximum retry count
maxRetry = 3
# Retry delay strategy
retryDelayFunc = "exponential"
# Per-task timeout
taskTimeout = "30s"
# Slow log threshold
slowLogThreshold = "1s"
```

### Benchmarking

Use Go's built-in benchmark tools to measure hot paths:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run benchmarks for a specific package
go test -bench=. -benchmem ./internal/app/gateway/application/...

# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof ./internal/app/user/domain/...

# Generate memory profile
go test -bench=. -memprofile=mem.prof ./...

# Analyze profile
go tool pprof cpu.prof
go tool pprof mem.prof
```

## Configuration Examples

### Database Connection Pool

```toml
# configs/gateway/config.toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=1s&timeout=1s&writeTimeout=3s"
```

Override connection pool parameters via environment variables:

```bash
export EGOADMIN_CLIENT_MYSQL_MAXOPENCONNS=100
export EGOADMIN_CLIENT_MYSQL_MAXIDLECONNS=10
export EGOADMIN_CLIENT_MYSQL_CONNMAXLIFETIME="1h"
```

Connection pool parameter guide:

| Parameter | Recommended | Description |
|-----------|-------------|-------------|
| maxOpenConns | 50-100 | Max open connections, adjust based on MySQL max_connections |
| maxIdleConns | 10-20 | Max idle connections, set to 10-20% of maxOpenConns |
| connMaxLifetime | 1h | Max connection lifetime, avoids using connections closed by server |
| readTimeout | 1s | Read timeout |
| writeTimeout | 3s | Write timeout |

::: warning Connection Leaks
If `maxOpenConns` is set to 0 (unlimited), high concurrency may exhaust MySQL connections. Always set a reasonable upper bound.
:::

### JetCache Full Configuration

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

### gRPC Client Tuning

```toml
# configs/gateway/config.toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"
```

::: info Service Discovery
gRPC addresses use the etcd service discovery format `etcd:///egoadmin-user`. The EGO framework automatically resolves to actual instance addresses and maintains a connection pool.
:::

### DSN Timeout Parameters

Timeout parameters in the MySQL DSN significantly impact performance:

```bash
# Recommended DSN parameters
?readTimeout=1s&timeout=1s&writeTimeout=3s
```

| Parameter | Description | Recommended |
|-----------|-------------|-------------|
| `timeout` | Connection establishment timeout | 1s |
| `readTimeout` | Query read timeout | 1s |
| `writeTimeout` | Write timeout | 3s |

## Real-World Examples

### Production Tuning Checklist

```text
Database
  ├── Set reasonable connection pool limits
  ├── Confirm slow query log is enabled
  ├── Add indexes for high-frequency query fields
  └── Use EXPLAIN to analyze critical SQL

Cache
  ├── Enable JetCache for high-frequency read data
  ├── Set localExpiry reasonably to avoid cache avalanche
  ├── Use notFoundExpiry to cache empty results
  └── Monitor cache hit rate (enableMetrics = true)

gRPC
  ├── Adjust readTimeout based on network latency
  ├── Use etcd service discovery for load balancing
  └── Avoid passing large data in hot paths

Memory
  ├── Run prealloc linter to check slice preallocation
  ├── Use go test -bench -memprofile to analyze allocation hotspots
  └── Avoid frequent allocation of temporary objects in loops
```

### Cache Warming

After service startup, you can warm the cache proactively:

```bash
# Proactively call high-frequency endpoints after startup
curl -s http://localhost:9001/api/permission/menu
curl -s http://localhost:9001/api/user/info

# JetCache's refreshDuration will automatically refresh hot caches
```

### Distributed Lock Usage Pattern

Use Redis distributed locks to protect shared resources:

```text
Request → Acquire lock → Execute operation → Release lock
           ↓ (acquire failed)
         Wait or return error
```

When asyncq has `enableServer = true`, the current instance also acts as a task consumer. In production, consider enabling this on only some instances:

```toml
# Production instance A: consumer
[client.asyncq]
enableClient = true
enableServer = true
concurrency = 20

# Production instance B: producer only
[client.asyncq]
enableClient = true
enableServer = false
```

## How It Works

### JetCache Dual-Layer Cache Flow

```text
Request → Query local cache
           ├── Hit → Return (microsecond-level)
           └── Miss → Query remote cache (Redis)
                       ├── Hit → Write to local → Return (millisecond-level)
                       └── Miss → Query database
                                    ├── Has data → Write to remote + local → Return
                                    └── No data → Write notFoundExpiry cache → Return empty
```

The `refreshDuration` config enables background async refresh: a background goroutine refreshes the cache before it expires, preventing cache stampede.

### Connection Pool Mechanics

```text
Application request → Get idle connection from pool
                       ├── Idle available → Use → Return
                       ├── Below limit → Create new → Use → Return
                       └── At limit → Wait (until connMaxLifetime releases old connections)
```

`connMaxLifetime` ensures connections are not held indefinitely, preventing server-side timeout disconnects.

### gRPC Connection Reuse

The EGO framework maintains a connection pool for each gRPC client, with automatic load balancing via etcd service discovery:

```text
gateway → gRPC client pool
            ├── channel 1 → user instance 1
            ├── channel 2 → user instance 2
            └── channel N → user instance N
```

## Common Issues

### Too Many Database Connections

```bash
# Check MySQL current connection count
mysql> SHOW STATUS LIKE 'Threads_connected';

# Check connection pool config for each service
grep -A3 "client.mysql" configs/*/config.toml
```

Adjust connection pool parameters:

```toml
[client.mysql]
maxOpenConns = 50
maxIdleConns = 10
connMaxLifetime = "1h"
```

### Low Cache Hit Rate

Check JetCache metrics:

```bash
# Confirm enableMetrics is on
grep "enableMetrics" configs/*/config.toml

# View cache hit statistics (via EGO metrics endpoint)
curl -s http://localhost:9003/metrics | grep jetcache
```

::: tip Tuning Advice
If cache hit rate is below 80%, consider increasing `localSize`, adjusting `localExpiry`, or reviewing cache key design.
:::

### High Memory Usage

Use pprof to locate memory allocation hotspots:

```bash
# Capture heap memory profile
go tool pprof -top -inuse_space http://localhost:9003/debug/pprof/heap

# View functions with most allocations
go tool pprof -top -alloc_objects http://localhost:9003/debug/pprof/heap

# View flame graph in browser
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/heap
```

### gRPC Call Timeout

Check gRPC client configuration and network latency:

```bash
# Test gRPC server port connectivity
grpcurl -plaintext etcd:2379 list

# Check instances registered in etcd
etcdctl get --prefix /egoadmin/

# Adjust timeouts
# For cross-datacenter calls, increase readTimeout and dialTimeout appropriately
```

### asyncq Task Backlog

Check Redis queue length and worker status:

```bash
# Check pending tasks in queues
redis-cli LLEN asyncq:default
redis-cli LLEN asyncq:critical
redis-cli LLEN asyncq:low

# Increase worker concurrency
# [client.asyncq]
# concurrency = 20

# Confirm enableServer = true on at least one instance
```

## Reference Links

- [GORM Connection Pool Documentation](https://gorm.io/docs/generic_interface.html#Connection-Pool)
- [Go pprof Documentation](https://pkg.go.dev/runtime/pprof)
- [JetCache Configuration](https://github.com/alibaba/jetcache)
- [Redis Distributed Locks](https://redis.io/docs/manual/patterns/distributed-locks/)
- [Monitoring & Alerting](/guide/en-US/deployment/monitoring)
- [Docker Containerization](/guide/en-US/deployment/docker)
