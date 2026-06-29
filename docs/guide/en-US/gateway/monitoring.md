# Monitoring and Operations

The gateway provides health checks, readiness probes, OpenTelemetry distributed tracing, pprof performance profiling, and graceful shutdown for operational needs.

## Overview

The gateway's operational system is built around three dimensions: observability (tracing + metrics), health state (liveness + readiness probes), and lifecycle management (graceful shutdown). The Governor service runs independently of the business HTTP/gRPC ports and is dedicated to exposing operational endpoints. OpenTelemetry sends trace data to backends like Jaeger via the OTLP protocol.

```text
Load Balancer
  |-> GET :9001/healthz    Liveness probe (checks MySQL + Redis)
  |-> GET :9001/readyz     Readiness probe (true only after migrations complete)
  |-> :9003/debug/pprof/*  Performance profiling
  |-> :4317 OTLP           Trace data
```

## Core Usage

### Health Check (Liveness Probe)

The gateway's health check registers `/healthz` and `/readyz` endpoints on the HTTP server:

```go
// internal/platform/health/health.go
func Start(fn CheckFn, c *egin.Component, opts ...Option) *Options {
    o := &Options{cf: fn, ready: false}
    c.GET("/readyz", o.readyz)
    c.GET("/healthz", o.healthz)
    return o
}
```

The `/healthz` liveness probe checks that MySQL and Redis are reachable:

```go
// internal/app/gateway/server/server.go
func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
    return health.Start(func() bool {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
        defer cancel()

        // Check database
        dbx, err := db.DB()
        if err != nil {
            return false
        }
        err = dbx.PingContext(ctx)
        if err != nil {
            return false
        }

        // Check Redis
        _, err = rds.Ping(ctx)
        if err != nil {
            return false
        }

        return true
    }, c)
}
```

| Endpoint | Status Code | Meaning |
|----------|-------------|---------|
| `/healthz` | 200 | MySQL and Redis are both healthy |
| `/healthz` | 502 | MySQL or Redis is unavailable |
| `/readyz` | 200 | Service is ready to accept traffic |
| `/readyz` | 502 | Service not ready (migrating or starting up) |

### Readiness Probe

The readiness state defaults to `false` and is only set to `true` after all initialization steps (including database migrations) complete:

```go
// internal/app/gateway/server/server.go - newApp()
func newApp(opts Options) (*App, error) {
    // ... Initialize components ...

    // Sync API dictionary
    if err := opts.apiSrv.SyncFromCatalog(context.Background(), egoadmin.APICatalog); err != nil {
        return nil, err
    }

    // Validate permission contract
    if err := opts.permission.EnsurePermissionContract(context.Background()); err != nil {
        return nil, err
    }

    // Register file upload routes
    // ...

    // Initialize frontend SPA
    web.StartWithFS(egoadmin.FrontendAssets, opts.conf.Web(), opts.http)

    // Mark ready after all initialization is complete
    opts.health.Ready()

    return &App{Ego: opts.app}, nil
}
```

::: info Readiness Delay
During the time window between process startup and marking ready, the load balancer's `/readyz` probe returns 502, and new requests are not routed to this instance. This is critical for rolling deployments and zero-downtime restarts.
:::

Readiness state can be toggled dynamically:

```go
// Mark ready (after initialization)
opts.health.Ready()

// Mark not ready (shutdown drain phase)
opts.health.NotReady()

// Query current state
if opts.health.IsReady() {
    // Service is ready
}
```

### OpenTelemetry Distributed Tracing

The gateway sends trace data to backends (like Jaeger) via OTLP protocol through OpenTelemetry:

```toml
# configs/gateway/config.toml

[trace]
ServiceName = "egoadmin-gateway-local"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:4317"
```

| Config Key | Description |
|------------|-------------|
| `ServiceName` | Service name, displayed in Jaeger UI |
| `OtelType` | Transport protocol, `otlp` uses OpenTelemetry Protocol |
| `Fraction` | Sample rate, `1.00` means 100% sampling |
| `Endpoint` | OTLP Collector address (gRPC port) |

The trace interceptor is enabled in both HTTP and gRPC servers:

```toml
[server.http]
enableTraceInterceptor = true
```

Trace context automatically propagates across services via gRPC metadata. When the gateway calls the user service, the trace parent span is automatically passed, forming a complete distributed trace.

### Governor Operations Service

Governor is an operations HTTP server independent of the business service, exposing debugging and management endpoints:

```go
// internal/app/gateway/server/govern_server.go
func NewGovernServer(_ config.EgoReady) *egovernor.Component {
    return egovernor.Load("server.governor").Build()
}
```

Governor defaults to port 9003, providing the following endpoints:

| Endpoint | Description |
|----------|-------------|
| `/health` | Health check (Governor itself) |
| `/debug/pprof/` | Go pprof profiling entry |
| `/debug/pprof/profile` | CPU profile (default 30 seconds) |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/goroutine` | Goroutine dump |
| `/debug/pprof/trace` | Execution trace |

### pprof Performance Profiling

During development and troubleshooting, you can obtain runtime performance data via pprof:

```bash
# CPU analysis (default 30 second sampling)
go tool pprof http://localhost:9003/debug/pprof/profile?seconds=30

# Heap memory analysis
go tool pprof http://localhost:9003/debug/pprof/heap

# Goroutine analysis
go tool pprof http://localhost:9003/debug/pprof/goroutine

# View current goroutine count
curl http://localhost:9003/debug/pprof/goroutine?debug=1

# Get flame graph data
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/profile
```

::: warning Production pprof
pprof endpoints should have restricted access in production. Use network policies or reverse proxies to allow only internal network access to the Governor port.
:::

### Graceful Shutdown

The gateway uses `shutdown.Manager` to uniformly manage resource closing order. The shutdown process ensures the service is marked as not-ready before closing connections, so the load balancer stops sending new requests.

```go
// internal/app/gateway/server/shutdown.go
func configureShutdown(opts Options) {
    opts.shutdown.RegisterCloser("config", opts.conf)
    opts.shutdown.RegisterRegistry(opts.registry)     // Deregister from etcd
    opts.shutdown.RegisterDB("mysql", opts.db)        // Close MySQL connection pool
    if opts.redis != nil {
        opts.shutdown.RegisterCloser("redis", opts.redis)  // Close Redis
    }
    if opts.userClient != nil {
        opts.shutdown.RegisterCloser("user-grpc-client", opts.userClient)  // Close gRPC client
    }
    if opts.idgenClient != nil {
        opts.shutdown.RegisterCloser("idgen-grpc-client", opts.idgenClient)
    }
    if opts.idm != nil {
        opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
            return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
        })
    }
    opts.shutdown.Bind(opts.app)  // Bind to ego lifecycle
}
```

Graceful shutdown execution order:

```text
1. Receive SIGTERM/SIGINT signal
2. Stop accepting new HTTP/gRPC connections
3. Wait for in-flight requests to complete (with timeout)
4. Deregister from etcd service registration
5. Close gRPC client connections (user, idgen)
6. Release idgen machine lease
7. Close Redis connection
8. Close MySQL connection pool
9. Write final config/state
10. Process exits
```

### Prometheus Metrics

The gateway collects request metrics via the Ego framework's built-in metric interceptors:

```toml
[server.http]
enableMetricInterceptor = false   # Enable as needed
enableAccessInterceptorReq = false  # Request body logging
enableAccessInterceptorRes = false  # Response body logging
```

Available metrics include:

| Metric | Type | Description |
|--------|------|-------------|
| Request latency | Histogram | Request duration by method and status code |
| Request count | Counter | Total requests by method and status code |
| Active connections | Gauge | Currently processing connections |

## Configuration Examples

### Full Tracing Configuration

```toml
# configs/gateway/config.toml

[trace]
ServiceName = "egoadmin-gateway-local"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:4317"
```

Production recommendation to reduce sample rate:

```toml
[trace]
ServiceName = "egoadmin-gateway-prod"
OtelType = "otlp"
Fraction = 0.10  # 10% sampling
```

### Governor Configuration

```toml
[server.governor]
host = "0.0.0.0"
port = 9003
enableLocalMainIP = false
```

### Service Discovery Registration Configuration

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"
```

| Config Key | Description |
|------------|-------------|
| `serviceTTL` | Service lease TTL, deregisters from etcd if not renewed within timeout |
| `prefix` | Service registration path prefix |

## Real-World Examples

### Setting Up Local Jaeger Tracing

```bash
# Start Jaeger (all-in-one mode)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# Start gateway
make run SERVICE=gateway

# Send request
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"page": 1, "pageSize": 10}'

# Open Jaeger UI to view traces
open http://localhost:16686
```

### Health Check Script

```bash
#!/bin/bash
# Health check script for Docker/K8s probes

# Liveness probe
healthz=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/healthz)
if [ "$healthz" != "200" ]; then
    echo "Liveness check failed: $healthz"
    exit 1
fi

# Readiness probe
readyz=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/readyz)
if [ "$readyz" != "200" ]; then
    echo "Readiness check failed: $readyz"
    exit 1
fi

echo "All checks passed"
```

### Docker Compose Probe Configuration

```yaml
# deploy/docker-compose.yaml
services:
  gateway:
    image: egoadmin/gateway:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9001/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 30s
    ports:
      - "9001:9001"
      - "9003:9003"
```

## How It Works

### Health Check State Machine

```text
Process starts
  |
  v
ready = false  ->  /readyz returns 502
  |                   (load balancer does not send new requests)
  v
Atlas migration complete
  |
  v
API dictionary sync complete
  |
  v
Permission contract validation complete
  |
  v
Frontend SPA initialization complete
  |
  v
health.Ready()  ->  /readyz returns 200
  |                   (starts accepting new requests)
  v
Receive SIGTERM
  |
  v
health.NotReady()  ->  /readyz returns 502
  |                      (load balancer stops sending new requests)
  v
Wait for active requests to complete (drain)
  |
  v
Close resources -> Process exits
```

Trace context automatically propagates across services via gRPC metadata. The gateway HTTP entry creates a root span, passes trace context via gRPC metadata when calling the user service, and the user service creates a child span. All spans are ultimately reported to the OTLP Collector and displayed as a complete trace in the Jaeger UI.

## Common Issues

### Missing Trace Data

**Symptom**: Jaeger UI does not show gateway trace data.

**Troubleshooting**:

1. Confirm Jaeger/OTLP Collector is running: `curl http://localhost:16686/`
2. Check `trace.otlp.endpoint` configuration is correct
3. Confirm `enableTraceInterceptor = true`
4. Check that `Fraction` is not `0` (0 means no sampling)
5. Verify network connectivity: `telnet 127.0.0.1 4317`

### Health Check Returns 502

**Symptom**: `/healthz` or `/readyz` returns 502.

**Troubleshooting**:

For `/healthz`:

1. Check MySQL is reachable: `mysqladmin ping`
2. Check Redis is reachable: `redis-cli ping`
3. Check connection timeout configuration (default 10 seconds)

For `/readyz`:

1. Check if database migration is stuck
2. Check if API dictionary sync failed
3. Check if permission contract file is complete
4. Review startup logs for error messages

### Graceful Shutdown Not Working

**Symptom**: Requests are immediately interrupted after sending SIGTERM.

**Troubleshooting**:

1. Confirm shutdown.Manager has all resources registered correctly
2. Check that `opts.shutdown.Bind(opts.app)` is called
3. Confirm load balancer probe interval is reasonable (recommend under 10 seconds)
4. Check drain timeout configuration

### pprof Endpoint Unreachable

**Symptom**: `curl http://localhost:9003/debug/pprof/` has no response.

**Troubleshooting**:

1. Confirm Governor port configuration is correct
2. Check firewall allows port 9003
3. Confirm `NewGovernServer` is in the Wire injection chain

### Service Auto-Deregisters from etcd

**Symptom**: Service suddenly deregisters from etcd while running.

**Troubleshooting**:

1. Check etcd cluster health status
2. Confirm `serviceTTL` is reasonable (recommend 10 seconds or more)
3. Check for network jitter causing lease renewal failures
4. Review etcd client logs for connection errors

## Reference Links

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Jaeger Official Documentation](https://www.jaegertracing.io/docs/)
- [Go pprof Documentation](https://pkg.go.dev/net/http/pprof)
- [Kubernetes Probe Configuration](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- Relevant project source code:
  - `internal/platform/health/health.go` -- Health check and readiness probe
  - `internal/app/gateway/server/govern_server.go` -- Governor operations service
  - `internal/app/gateway/server/shutdown.go` -- Graceful shutdown configuration
  - `internal/app/gateway/server/server.go` -- Service initialization and readiness marking
  - `configs/gateway/config.toml` -- Tracing and Governor configuration
