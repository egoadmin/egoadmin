# Debug Tools

This page introduces the debugging tools and techniques available in the EgoAdmin project, covering Make commands, Go debugging, middleware debugging, network tracing, log analysis, and health checks.

## Overview

EgoAdmin provides a complete toolchain from code quality checks to production issue diagnosis. During daily development, Make commands are the primary entry point; for deeper investigation, tools like pprof, Delve, Redis CLI, and MySQL CLI enable layered diagnostics.

Debug tools fall into these categories:

- **Code quality**: lint, type check, test.
- **Runtime debugging**: pprof, Delve debugger.
- **Middleware debugging**: Redis, MySQL, GORM.
- **Network and tracing**: gRPC reflection, Jaeger, curl testing.
- **Health and status**: healthz, readyz, configuration printing.

::: warning
Debug ports such as pprof and gRPC reflection are only enabled in development. Production environments should ensure governor and debug ports are not exposed to the public network.
:::

## Make Commands

Make is EgoAdmin's unified command entry point. All common operations are executed via `make <target>`.

### Code Quality

```bash
# Run all linters (gofmt, buf lint, golangci-lint)
make lint

# Format code
make fmt

# Generate proto / Go / Wire code
make gen
```

### Build

```bash
# Build all services
make build

# Build a single service
make build SERVICE=gateway
make build SERVICE=user
make build SERVICE=idgen
```

### Testing

```bash
# Run Go unit tests
make test

# Run e2e tests (requires all services and middleware running)
make e2e E2E_TIMEOUT=20m
```

### Service Checks

```bash
# Verify service Wire injection is correct
make service.check SERVICE=user

# View fully resolved service configuration
make service.config SERVICE=gateway
```

### Middleware Management

```bash
# Start/stop all middleware
make dev-up
make dev-down

# Control a single middleware
make dev.up-one MIDDLEWARE=mysql-user
make dev.down-one MIDDLEWARE=redis-user
make dev.reset-one MIDDLEWARE=etcd
```

### Database Migrations

```bash
# Create a new migration
make migrate.new SERVICE=user NAME=add_field

# Validate migration files
make migrate.validate SERVICE=user

# Recalculate migration hash
make migrate.hash SERVICE=user

# Apply migrations
make migrate.apply SERVICE=user ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

::: tip
`make migrate.new` automatically connects to the local database and generates diff SQL. Ensure `make dev-up` has completed and the target database exists.
:::

## Go Debugging

### Running Specific Tests

```bash
# Run a single test function
go test -v -run TestXxx ./internal/app/user/...

# Run tests for a specific package
go test -v ./internal/app/user/application/...

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./internal/app/user/...
```

::: tip
The `-race` flag detects concurrent data races. It is recommended to enable race detection in CI at all times.
:::

### pprof Performance Profiling

Each service exposes `/debug/pprof/` endpoints on its governor port:

| Service | Governor Port |
|---------|---------------|
| gateway | `9003` |
| user | `9103` |
| idgen | `9203` |

**CPU profiling**:

```bash
# Capture a 30-second CPU profile
go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

# Common pprof interactive commands
# (pprof) top 20
# (pprof) web
# (pprof) list funcName
```

**Heap memory profiling**:

```bash
# View current heap memory allocation
go tool pprof http://localhost:9103/debug/pprof/heap

# (pprof) top
# (pprof) top -inuse_space
# (pprof) list funcName
```

**Goroutine dump**:

```bash
# View all goroutine call stacks
go tool pprof http://localhost:9103/debug/pprof/goroutine

# (pprof) top
# (pprof) list
```

**Memory allocation rate**:

```bash
# Allocs analysis to find allocation hotspots
go tool pprof http://localhost:9103/debug/pprof/allocs
```

::: tip
Use `go tool pprof -http=:8081 http://localhost:9103/debug/pprof/heap` to directly open a web UI with flame graphs.
:::

### Delve Debugger

Delve (dlv) is Go's standard debugger, supporting breakpoints, stepping, and variable inspection.

```bash
# Start user service in debug mode
dlv debug ./cmd/user -- --config configs/user/config.toml

# Set breakpoint
(dlv) break internal/app/user/application/user_service.go:42

# Run to breakpoint
(dlv) continue

# Inspect variables
(dlv) print req
(dlv) print user.Name

# View call stack
(dlv) bt

# Step execution
(dlv) next
(dlv) step
```

**Remote debugging**:

```bash
# Start in headless mode on target machine
dlv debug ./cmd/user --headless --listen=:2345 -- --config configs/user/config.toml

# Connect from local machine
dlv connect localhost:2345
```

::: warning
Delve pauses process execution. Do not attach it to running services in production or during e2e tests.
:::

## Redis Debugging

### Enable Redis Debug Mode

Enable Redis debug logging in the service configuration:

```toml
[client.redis]
debug = true
```

When enabled, the service log `logs/ego.sys` records the execution time and parameters of each Redis command.

### Redis CLI

```bash
# Connect to Redis
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin

# Common commands
127.0.0.1:6380> KEYS session:*        # View session keys
127.0.0.1:6380> DBSIZE                # View key count
127.0.0.1:6380> INFO memory           # Memory usage
127.0.0.1:6380> INFO clients          # Client connections
```

### Real-Time Monitoring

```bash
# Monitor all Redis commands (use caution in production, impacts performance)
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin monitor
```

::: warning
`redis-cli monitor` prints all commands and has a significant performance impact. Use only in development, and do not run for extended periods.
:::

## Database Debugging

### GORM Debug Mode

Enable GORM debug logging in the service configuration:

```toml
[client.mysql]
debug = true
```

When enabled, each SQL statement's execution time, parameters, and returned row count are recorded in the logs.

### MySQL CLI

```bash
# Connect to MySQL
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin

# Switch database
mysql> USE egoadmin_user;

# Common commands
mysql> SHOW TABLES;
mysql> SHOW PROCESSLIST;
mysql> EXPLAIN SELECT * FROM sys_user WHERE username = 'admin';
```

### Slow Query Investigation

```bash
# Check MySQL slow query log settings
mysql> SHOW VARIABLES LIKE 'slow_query%';
mysql> SHOW VARIABLES LIKE 'long_query_time';
```

::: tip
The development MySQL has general log enabled by default. View container logs to see all SQL executions: `docker logs mysql-user`.
:::

### Migration Status Check

```bash
# Verify migration file integrity
make migrate.hash SERVICE=user

# View migration directory
ls -la atlas/migrations/user/

# Check current database migration version
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin egoadmin_user -e "SELECT * FROM atlas_schema_revisions ORDER BY version DESC LIMIT 5;"
```

## Network and gRPC Debugging

### Jaeger Distributed Tracing

EgoAdmin integrates OpenTelemetry. When Jaeger is running, you can inspect complete request traces:

```bash
# Start Jaeger (included in dev-up)
make dev-up

# Access Jaeger UI
# http://localhost:16686
```

Jaeger helps identify:

- Time distribution across cross-service calls.
- Which services and middleware a request traverses.
- Whether gRPC calls time out or fail.

::: tip
The Jaeger OTLP ingestion port is `4317` (gRPC). Ensure the trace endpoint in service configuration points to this address.
:::

### gRPC Reflection

gRPC reflection is enabled by default in development mode. You can use `grpcurl` for testing:

```bash
# List all services
grpcurl -plaintext 127.0.0.1:9002 list

# List methods of a service
grpcurl -plaintext 127.0.0.1:9002 list user.v1.UserService

# Call a method
grpcurl -plaintext -d '{"username": "admin"}' 127.0.0.1:9002 user.v1.UserService/GetUserList
```

### curl Testing HTTP API

The gateway handles HTTP POST requests directly via protoc-gen-go-http generated handlers, which you can test with curl:

```bash
# Login to obtain a token
curl -X POST http://localhost:9001/api/user.v1.UserService/Login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "123456"}'

# Call API with token
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{}'
```

::: tip
The gateway HTTP route format is `/api/<package>.<Service>/<Method>`. The request body is a JSON-encoded proto message.
:::

### etcd Service Discovery Debugging

```bash
# List all registered services
etcdctl --endpoints=127.0.0.1:2379 get --prefix /egoadmin --keys-only

# View details of a specific service
etcdctl --endpoints=127.0.0.1:2379 get /egoadmin-user --prefix

# Watch for key changes
etcdctl --endpoints=127.0.0.1:2379 watch /egoadmin --prefix
```

## Log Analysis

### EGO Structured Logs

EgoAdmin uses the EGO framework's logging system, outputting to `logs/ego.sys`.

Log format:

```text
2026-06-29 10:30:00 INF message key=value key2=value2
```

Key fields:

- `level`: Log level (DBG/INF/WRN/ERR).
- `comp`: Component name (e.g., `server.http`, `client.grpc`, `client.redis`).
- `access`: Request logs (when access logging is enabled).

### Enable Request Logging

Enable access logging in the configuration to see detailed per-request information:

```toml
[server.http]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = true
```

When enabled, logs record for each request:

- Request method and path.
- Request parameters.
- Response status code and body.
- Duration.

::: warning
Enabling access logging in production increases log volume. Enable only temporarily when troubleshooting.
:::

### Slow Log Thresholds

Redis and message queue components support slow log configuration:

```toml
[client.redis]
slowThreshold = "100ms"

[client.asyncq]
slowThreshold = "200ms"
```

Operations exceeding the threshold are flagged as slow, helping identify performance bottlenecks.

### Adjusting Log Levels

You can temporarily lower the log level for more detailed information during development:

```bash
# Adjust via environment variable
export EGO_LOG_LEVEL=debug
make run SERVICE=user
```

## Health Checks

### HTTP Health Check Endpoints

Each service exposes two health check endpoints:

| Service | healthz | readyz |
|---------|---------|--------|
| gateway | `http://localhost:9001/healthz` | `http://localhost:9001/readyz` |
| user | `http://localhost:9101/healthz` | `http://localhost:9101/readyz` |
| idgen | `http://localhost:9201/healthz` | `http://localhost:9201/readyz` |

```bash
# Quick check
curl http://localhost:9001/healthz
curl http://localhost:9101/readyz
```

::: warning
healthz and readyz use the HTTP port, not the gRPC port. For example, the user service HTTP port is `9101`, and the gRPC port is `9102`.
:::

### Governor Endpoints

Governor ports provide runtime information and debug entry points:

| Service | Governor Port | Common Endpoints |
|---------|---------------|------------------|
| gateway | `9003` | `/debug/pprof/`, `/health` |
| user | `9103` | `/debug/pprof/`, `/health` |
| idgen | `9203` | `/debug/pprof/`, `/health` |

```bash
# Governor health check
curl http://localhost:9103/health
```

The governor's `/health` endpoint checks connectivity to all downstream dependencies (MySQL, Redis, etcd) and returns the status of each component.

## Environment Variables

### EGOADMIN Prefix Variables

EgoAdmin supports overriding configuration file values through `EGOADMIN_*` environment variables:

```bash
# Service name (used for etcd registration)
export EGO_NAME=egoadmin-user

# Database migration URL
export ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

### Viewing Default Configuration

```bash
# Print default configuration (confirms all available config keys)
go run ./cmd/user config print-default

# Equivalent
make service.config SERVICE=user
```

::: tip
`config print-default` outputs all available keys with default values. This should be the first step when diagnosing configuration issues.
:::

### Derived Project Environment Variables

Derived projects can use `egoadminctl init --env-prefix DEMOADMIN` to set a new environment variable prefix. Then use `DEMOADMIN_*` instead of `EGOADMIN_*`.

## Comprehensive Debugging Workflow

When encountering issues, follow these steps in order:

```text
1. Check service health
   curl http://localhost:9101/healthz

2. Check downstream dependencies
   curl http://localhost:9103/health

3. View structured logs
   tail -f logs/ego.sys | grep ERR

4. Enable debug mode
   Set [client.mysql] debug = true / [client.redis] debug = true

5. Use pprof to find performance bottlenecks
   go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

6. Use Jaeger to inspect traces
   http://localhost:16686

7. Use Delve for breakpoint debugging
   dlv debug ./cmd/user -- --config configs/user/config.toml
```

## Reference Links

- `configs/gateway/config.toml` — gateway configuration
- `configs/user/config.toml` — user configuration
- `configs/idgen/config.toml` — idgen configuration
- `test/compose/docker-compose.dev.yml` — development middleware compose
- `cmd/gateway/` — gateway entry point
- `cmd/user/` — user entry point
- `cmd/idgen/` — idgen entry point
- `internal/component/authsession/` — authentication session component
- `internal/platform/` — platform infrastructure
- `logs/ego.sys` — structured log output
