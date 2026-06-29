# Monitoring & Alerting

EgoAdmin provides built-in health checks, distributed tracing, profiling, and structured logging to support service observability in production.

## Overview

EgoAdmin's monitoring system covers four dimensions:

| Dimension | Tool | Endpoint/Config |
|-----------|------|-----------------|
| Health checks | `/healthz` + `/readyz` | Governor HTTP port |
| Distributed tracing | OpenTelemetry + Jaeger | `trace.otlp` config |
| Profiling | pprof | Governor `/debug/pprof/*` |
| Structured logging | EGO log framework | `logs/ego.sys` |

## Core Usage

### Health Checks

Each EgoAdmin service exposes two health check endpoints:

```text
GET /healthz    # Liveness check
GET /readyz     # Readiness check
```

- `/healthz` verifies MySQL and Redis connections are healthy.
- `/readyz` only returns 200 after database migration completes and the service is fully initialized.

```go
// internal/platform/health/health.go
// Register health check endpoints on the Governor HTTP server
health.Start(checkFn, govHTTPComponent)

// Mark service as ready after migration completes
healthOpts.Ready()

// Mark service as unavailable during graceful shutdown (drain phase)
healthOpts.NotReady()
```

::: tip Kubernetes Probes
In K8s, configure `/healthz` as the livenessProbe and `/readyz` as the readinessProbe. When the readinessProbe fails, K8s stops sending new traffic to that Pod.
:::

### Distributed Tracing

EgoAdmin implements distributed tracing via OpenTelemetry, supporting trace data export to backends like Jaeger.

```bash
# Start Jaeger (development environment)
make dev-up
# Access Jaeger UI
# http://localhost:16686
```

View trace data for a specific service:

```text
Jaeger UI → Service: egoadmin-gateway-local → Find Traces
```

### pprof Profiling

The Governor port exposes pprof endpoints for CPU, heap, and goroutine profiling:

```bash
# Capture 30-second CPU profile
go tool pprof http://localhost:9003/debug/pprof/profile?seconds=30

# View heap memory allocations
go tool pprof http://localhost:9003/debug/pprof/heap

# View goroutines
go tool pprof http://localhost:9003/debug/pprof/goroutine

# Interactive web UI
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/profile?seconds=30
```

### Viewing Logs

The EGO framework outputs logs to files and stdout in structured JSON format:

```bash
# View system logs
tail -f logs/ego.sys

# Filter error level
grep '"level":"error"' logs/ego.sys

# Format with jq
tail -1 logs/ego.sys | jq .
```

## Configuration Examples

### Tracing Configuration

```toml
# configs/gateway/config.toml
[trace]
# Service name displayed in Jaeger UI
ServiceName = "egoadmin-gateway-local"
# Trace type: otlp (OpenTelemetry protocol)
OtelType = "otlp"
# Sampling rate: 1.0 = 100%, 0.1 = 10%
Fraction = 1.00

[trace.otlp]
# OTLP gRPC receiver endpoint
Endpoint = "127.0.0.1:4317"
```

::: warning Production Sampling Rate
In production, set `Fraction` to `0.1` or lower (10% or fewer requests are traced) to avoid large trace data impacting performance and storage. Development and test environments can use `1.00`.
:::

### Environment Variable Overrides

Tracing configuration can be overridden via environment variables for different deployment environments:

```bash
# Use Jaeger service name inside containers
export EGOADMIN_TRACE_SERVICE_NAME="egoadmin-gateway-prod"
export EGOADMIN_TRACE_FRACTION="0.1"
export EGOADMIN_TRACE_OTLP_ENDPOINT="jaeger:4317"
```

### Governor Configuration

The Governor port provides operational endpoints, configured in `server.governor`:

```toml
# configs/gateway/local-live.toml
[server.governor]
host = "0.0.0.0"
port = 9003
```

Governor ports per service:

| Service | Governor Port |
|---------|---------------|
| gateway | 9003 |
| user | 9103 |
| idgen | 9203 |

### Health Check Configuration

Health check configuration in Docker Compose:

```yaml
# deploy/compose/app.yml
services:
  gateway:
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O - http://127.0.0.1:9001/readyz >/dev/null"]
      interval: 10s
      timeout: 5s
      retries: 12
      start_period: 60s
```

## Real-World Examples

### Production Monitoring Architecture

```text
                         ┌──────────┐
                         │  Jaeger  │
                         │  :16686  │
                         └────▲─────┘
                              │ OTLP gRPC
┌──────────┐  ┌──────────┐  ┌┴─────────┐
│ gateway  │──│   user   │──│  idgen   │
│ :9003    │  │ :9103    │  │ :9203    │
│ pprof    │  │ pprof    │  │ pprof    │
│ /healthz │  │ /healthz │  │ /healthz │
│ /readyz  │  │ /readyz  │  │ /readyz  │
└──────────┘  └──────────┘  └──────────┘
```

### Day-to-Day Operations

```bash
# Check health status of all services
curl -s http://localhost:9001/healthz  # gateway
curl -s http://localhost:9101/healthz  # user
curl -s http://localhost:9201/healthz  # idgen

# Check service readiness
curl -s http://localhost:9001/readyz   # gateway

# View environment info
curl -s http://localhost:9003/env      # gateway governor

# Capture CPU profile
go tool pprof -top http://localhost:9003/debug/pprof/profile?seconds=30

# Check for goroutine leaks
curl -s http://localhost:9003/debug/pprof/goroutine?debug=2
```

### Monitoring in E2E Tests

E2E tests use health endpoints to wait for service readiness:

```text
test/compose starts middleware
  → make run SERVICE=gateway
  → poll /readyz until 200
  → run e2e tests
  → make run SERVICE=gateway shutdown (triggers NotReady)
```

## How It Works

### Health Check Implementation

```go
// health.Start registers two endpoints
func Start(fn CheckFn, c *egin.Component, opts ...Option) *Options {
    o := &Options{
        cf:    fn,
        ready: false,  // Initially not ready
    }

    // /readyz — checks the ready flag
    c.GET("/readyz", o.readyz)

    // /healthz — runs custom check function (MySQL ping + Redis ping)
    c.GET("/healthz", o.healthz)

    return o
}
```

Lifecycle:

```text
Startup → ready=false → Migration complete → Ready() → ready=true → Begin serving traffic
                                            ↓
                                  Shutdown signal → NotReady() → ready=false → drain → exit
```

### Trace Data Flow

```text
Application code → EGO trace interceptor → OpenTelemetry SDK → OTLP gRPC → Jaeger
                          ↓
                   Inject trace-id into HTTP/gRPC headers
                   Propagate context across services
```

During inter-service calls, trace context is automatically propagated through gRPC metadata without manual handling.

### pprof Endpoints

EGO Governor registers standard pprof handlers:

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/` | Index page |
| `/debug/pprof/profile` | CPU profile (default 30 seconds) |
| `/debug/pprof/heap` | Heap memory allocation |
| `/debug/pprof/goroutine` | Goroutine list |
| `/debug/pprof/trace` | Execution trace |
| `/debug/pprof/allocs` | Cumulative memory allocation |
| `/debug/pprof/block` | Block profiling |
| `/debug/pprof/mutex` | Mutex contention profiling |

### Log Structure

Structured log format from the EGO framework:

```json
{
  "level": "info",
  "ts": 1719600000.123,
  "caller": "server/egin.go:123",
  "msg": "access",
  "method": "POST",
  "path": "/api/user/login",
  "status": 200,
  "cost": 0.045,
  "traceId": "abc123def456"
}
```

## Common Issues

### /readyz Returns 502

The service has not completed initialization or database migration failed:

```bash
# Check service logs
docker compose logs gateway | grep -i "migrate\|error"

# Temporarily skip migration for diagnosis
EGOADMIN_ATLAS_MIGRATE=false docker compose up gateway
```

### No Trace Data in Jaeger

Check tracing configuration and OTLP endpoint connectivity:

```bash
# Verify Jaeger is running
docker compose ps jaeger

# Test OTLP endpoint
grpcurl -plaintext jaeger:4317 list

# Check sampling rate
grep "Fraction" configs/*/config.toml
```

::: info OTLP Port
Jaeger's OTLP gRPC receiver port is `4317`, not `16686`. Port `16686` is the Jaeger UI web port.
:::

### pprof Endpoints Inaccessible

Verify the Governor port is not blocked by a firewall:

```bash
# Check port listening
netstat -tlnp | grep 9003

# If running in Docker without port mapping, add to deploy/compose/app.yml:
# ports:
#   - "${GATEWAY_GOVERNOR_PORT:-9003}:9003"
```

### Log File Too Large

Configure log rotation or disable file logging in development:

```bash
# Check current log size
du -sh logs/ego.sys

# Clean up old logs
find logs/ -name "*.log" -mtime +7 -delete
```

### Tracing Data Using Too Much Memory

Reduce the sampling rate to decrease trace data volume:

```toml
[trace]
# Only trace 1% of requests
Fraction = 0.01
```

Or adjust at runtime via environment variable:

```bash
export EGOADMIN_TRACE_FRACTION="0.01"
```

## Reference Links

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Go pprof Documentation](https://pkg.go.dev/net/http/pprof)
- [Kubernetes Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Docker Containerization](/guide/en-US/deployment/docker)
- [Performance Optimization](/guide/en-US/deployment/performance)
