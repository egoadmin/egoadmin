# Graceful Shutdown & Lifecycle

This page explains how EgoAdmin uses `shutdown.Manager` to coordinate process exit, ensuring connection draining, ordered resource cleanup, and health check integration.

## Overview

When a microservice process receives `SIGTERM` or `SIGINT`, exiting immediately interrupts in-flight requests, leaks database connections, and leaves stale instances in the service registry. EgoAdmin provides a unified shutdown orchestration through the `internal/platform/shutdown` package so every service follows the same lifecycle semantics.

`shutdown.Manager` handles two responsibilities: before EGO stops the servers, it marks readiness as unavailable and waits for the drain period; after EGO stops the servers, it closes non-server resources (databases, Redis, gRPC clients, etc.) in reverse registration order. The EGO framework itself handles graceful shutdown of HTTP/gRPC/governor servers -- `Manager` does not duplicate that logic.

Each service's `newApp()` follows these steps: start critical components, call `configureShutdown()` to register closable resources, hand server lifecycle to EGO via `opts.app.Serve()`, and finally mark `health.Ready()`. This ensures the service only begins accepting traffic once everything is fully initialized.

## Core Usage

### Integrating shutdown.Manager in a Service

A typical service construction flow looks like this:

```go
func newApp(opts Options) (*App, error) {
    // Start critical components (idm, idgen, etc.)
    if opts.idm != nil {
        if err := opts.idm.Start(context.Background()); err != nil {
            cleanup()
            return nil, fmt.Errorf("start idgen machine manager: %w", err)
        }
    }
    if opts.idgen != nil {
        if err := opts.idgen.Start(); err != nil {
            cleanup()
            return nil, fmt.Errorf("start idgen: %w", err)
        }
    }

    // Register closable resources (must be before Serve)
    configureShutdown(opts)

    // Hand server lifecycle to EGO
    opts.app.Registry(opts.registry)
    opts.app.Serve(opts.http, opts.grpc, opts.govern)

    // Mark ready after all readiness checks pass
    opts.health.Ready()

    return &App{Ego: opts.app}, nil
}
```

### Registering Closable Resources

Register resources in dependency order inside `configureShutdown()`. The `Manager` closes them in reverse order:

```go
func configureShutdown(opts Options) {
    if opts.shutdown == nil {
        return
    }
    opts.shutdown.RegisterCloser("config", opts.conf)
    opts.shutdown.RegisterRegistry(opts.registry)
    opts.shutdown.RegisterDB("mysql", opts.db)
    if opts.redis != nil {
        opts.shutdown.RegisterCloser("redis", opts.redis)
    }
    if opts.jetcache != nil {
        opts.shutdown.RegisterCloser("jetcache", opts.jetcache)
    }
    if opts.idgenClient != nil {
        opts.shutdown.RegisterCloser("idgen-grpc-client", opts.idgenClient)
    }
    if opts.idm != nil {
        opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
            return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
        })
    }
    if opts.idgen != nil {
        opts.shutdown.RegisterCloser("idgen", opts.idgen)
    }
    opts.shutdown.Bind(opts.app)
}
```

::: tip Registration Order Principle
- Resources registered first are closed last. Components that depend on other resources should be registered **later**.
- `registry` should be registered early so it disconnects from the registry after all other resources have been closed.
- Databases, Redis, and other infrastructure should be registered before workers/gRPC clients that depend on them.
:::

### Registration API Reference

`shutdown.Manager` provides the following registration methods:

| Method | Use Case | Description |
|--------|----------|-------------|
| `Register(name, CloseFunc)` | Custom close logic | Receives `context.Context`, can honor `closeTimeout` |
| `RegisterCloser(name, Closer)` | Components implementing `Close() error` | Wraps as `CloseFunc`, ignores context |
| `RegisterRegistry(registry)` | etcd service registry | Ensures deregistration happens after other resources close |
| `RegisterDB(name, db)` | `egorm.Component` and similar DB wrappers | Internally calls `sql.DB.Close()` |

All registration methods are `nil`-safe -- passing `nil` registers nothing and does not panic.

### Per-Service configureShutdown Comparison

The three services differ in their close registrations, reflecting their resource topologies:

| Resource | gateway | user | idgen |
|----------|---------|------|-------|
| config | Yes | Yes | Yes |
| registry | Yes | Yes | Yes |
| mysql | Yes | Yes | Yes |
| redis | Conditional | Conditional | -- |
| jetcache | -- | Conditional | -- |
| user-grpc-client | Conditional | -- | -- |
| idgen-grpc-client | Conditional | Conditional | -- |
| idgen-machine-lease | Conditional | Conditional | -- |
| idgen | Conditional | Conditional | Conditional |

"Conditional" means the resource is only registered when the component is non-nil. This ensures correctness across different deployment topologies (e.g., no-Redis mode).

## Configuration Examples

### TOML Configuration

```toml
[app.shutdown]
# Total EGO stop timeout (includes drain + server shutdown + resource cleanup)
stopTimeout = "20s"

# Wait time after readiness is marked unavailable
# During this window load balancers remove the instance and in-flight requests finish
drainTimeout = "2s"

# Per-resource close timeout for non-server resources
closeTimeout = "5s"
```

### Environment Variable Overrides

Configuration values can be overridden via `EGOADMIN_*` prefixed environment variables. See [Runtime Configuration](./configuration.md).

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `stopTimeout` | duration | `20s` | Total EGO framework stop timeout, covers drain and all cleanup |
| `drainTimeout` | duration | `2s` | Time between marking readiness false and beginning server shutdown |
| `closeTimeout` | duration | `5s` | Per-resource timeout during the `afterStop` phase |

::: warning Timeout Normalization
If `drainTimeout` or `closeTimeout` exceeds `stopTimeout`, they are automatically clamped to `stopTimeout`. Negative values are corrected to defaults.
:::

## Real-World Examples

### Adding Close Support to a New Component

Any component implementing `Close() error` can be registered directly:

```go
// Custom component
type CacheClient struct {
    // ...
}

func (c *CacheClient) Close() error {
    return c.conn.Close()
}

// Register in configureShutdown
opts.shutdown.RegisterCloser("my-cache", opts.cacheClient)
```

You can also register arbitrary shutdown logic with `RegisterFunc`:

```go
opts.shutdown.Register("my-worker-pool", func(ctx context.Context) error {
    opts.workerPool.Shutdown()
    return opts.workerPool.WaitTimeout(ctx)
})
```

### IDGen Machine Lease Special Handling

When multiple IDGen instances are deployed, each acquires a machine number via database row locking. If a shutting-down instance actively releases its lease, it can race with the idgen service's own stopping path. The shutdown path therefore uses `StopWithoutRelease`, which only stops local lease renewal without calling the remote release API. The lease TTL naturally expires after a timeout.

```go
if opts.idm != nil {
    opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
        return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
    })
}
```

`StopMachineLeaseBestEffort` behavior:

1. If the manager implements `StopWithoutRelease`, it calls that (no remote lease release).
2. Otherwise, falls back to normal `Stop`.
3. For expected shutdown errors (`ErrStoreUnavailable`, `ErrMachineLeaseLost`, `context.Canceled`, `context.DeadlineExceeded`), the error is silently swallowed.

::: warning make run Full Stop
Running `make run` stops idgen, user, and gateway simultaneously. If idgen stops first, lease renewal for user/gateway will fail. Using `StopWithoutRelease` avoids these noisy errors.
:::

## How It Works

### Shutdown Timeline

```
Process receives SIGTERM/SIGINT
    |
    v
EGO framework calls beforeStop hooks
    |
    v
shutdown.Manager.beforeStop()
    ├── health.NotReady()           // readiness set to false
    ├── log "service marked not ready"
    └── sleep(drainTimeout)         // wait for load balancer removal
    |
    v
EGO framework stops all servers (HTTP/gRPC/governor)
    ├── waits for in-flight requests to complete
    ├── rejects new connections
    └── governed by stopTimeout
    |
    v
EGO framework calls afterStop hooks
    |
    v
shutdown.Manager.afterStop()
    ├── iterates closer list (reverse order)
    ├── for each resource: context.WithTimeout(closeTimeout) -> fn(ctx)
    ├── logs each resource's close result
    └── errors.Join aggregates all errors
    |
    v
elog.Flush()                       // flush log buffers
    |
    v
Process exits
```

### How Bind Hooks into the EGO Lifecycle

```go
func (m *Manager) Bind(app *ego.Ego) {
    ego.WithStopTimeout(m.config.StopTimeout)(app)
    ego.WithBeforeStopClean(m.beforeStop)(app)
    ego.WithAfterStopClean(m.afterStop, elog.DefaultLogger.Flush, elog.EgoLogger.Flush)(app)
}
```

`Bind` passes `stopTimeout` to EGO, then registers `beforeStop` and `afterStop` hooks. The `afterStop` hook also includes log flushing to ensure shutdown logs are not lost.

### Error Handling During Resource Close

`afterStop` does not abort if a single resource fails to close. All errors are aggregated via `errors.Join` and returned to the EGO framework. Each resource close error is individually logged at ERROR level.

### Readiness and Health Checks

`health.Options` provides two HTTP endpoints:

| Endpoint | Behavior |
|----------|----------|
| `/healthz` | Custom check function (e.g., ping DB/Redis), returns 200 or 502 |
| `/readyz` | Based on the `ready` flag, used for Kubernetes readiness probes |

During shutdown, `/readyz` immediately returns 502, causing the load balancer (e.g., Kubernetes Service) to remove the instance from the endpoint list. The `drainTimeout` gives the removal time to propagate.

### Wire Integration

`shutdown.Manager` is wired via dependency injection. The Wire ProviderSet is defined in `internal/platform/shutdown/provider.go`:

```go
var ProviderSet = wire.NewSet(
    NewConfig,    // parses [app.shutdown] from platform/config.Config
    NewLogger,    // returns elog.EgoLogger
    NewManager,   // constructs *Manager
)
```

`NewConfig` reads the `[app.shutdown]` section from platform config with `stopTimeout`, `drainTimeout`, `closeTimeout` fields as Go duration strings (e.g., `"20s"`, `"500ms"`). When configuration is missing or fields are empty, defaults are used.

Each service's Wire injector provides `*shutdown.Manager`, which is then used in `newApp` via `configureShutdown` to register resources and call `Bind`.

### Test Coverage

Core test cases in the shutdown package:

| Test | What It Verifies |
|------|------------------|
| `TestManagerBeforeStopMarksNotReadyAndDrains` | beforeStop marks readiness false and waits drainTimeout |
| `TestManagerAfterStopClosesResourcesInReverseOrder` | afterStop closes resources in reverse registration order |
| `TestManagerAfterStopAggregatesCloseErrors` | Errors from multiple failed closes are aggregated via errors.Join |

Health package tests:

| Test | What It Verifies |
|------|------------------|
| `Ready()` / `NotReady()` | State transitions and concurrency safety |
| `/readyz` endpoint | Returns 200 when ready, 502 when not ready |

IDGen package tests:

| Test | What It Verifies |
|------|------------------|
| `TestStopMachineLeaseBestEffort` | nil manager safety, normal stop |
| `TestStopMachineLeaseBestEffortUsesStopWithoutRelease` | Prefers StopWithoutRelease over Stop |
| `TestMachineLeaseManager_StopWithoutReleaseKeepsRemoteLease` | No remote release call made |

## Common Issues

### Why do I see "service marked not ready" in shutdown logs?

This is expected behavior. `beforeStop` outputs this log at the start of the drain phase, indicating the service has been marked as not accepting new traffic. It is not an error.

### Why do I see machine lease ERROR logs during make run shutdown?

If idgen stops before user/gateway, lease renewal requests will fail. Normally `StopMachineLeaseBestEffort` treats these as expected and silently handles them. If you still see ERROR-level logs, check:

1. Whether you are calling `Stop()` instead of `StopWithoutRelease`.
2. Whether `isExpectedMachineLeaseShutdownError` is missing a new error type.

### What happens if resources are registered in the wrong order?

If a gRPC client is registered before the database, the database will be closed before the client. If the client needs database access during its shutdown (e.g., to log), that will fail. Solution: register in dependency order -- databases, Redis, and other infrastructure before business clients.

### How long should drainTimeout be?

This depends on how long the load balancer takes to remove instances:

- Kubernetes: `preStop` hook time + readiness probe interval * failureThreshold, usually 5-15 seconds is sufficient.
- Direct connection mode: set to 0, since there is no external removal mechanism.
- General recommendation: `drainTimeout` should not exceed half of `stopTimeout`, leaving time for server shutdown and resource cleanup.

### How to test shutdown behavior?

```bash
# Unit tests: verify drain and reverse-order close
go test ./internal/platform/shutdown/...

# Unit tests: verify health state transitions
go test ./internal/platform/health/...

# Unit tests: verify idgen shutdown path
go test ./internal/component/idgen/...

# Integration test: start service then send SIGTERM
make run SERVICE=user &
sleep 5
kill -TERM $(pgrep egoadmin-user)
```

### What if I forget to register a new resource?

The resource will not be explicitly closed when the process exits. Database connection pools are usually reclaimed by the OS on process exit, but connections may linger on the MySQL side until `wait_timeout` expires. Unclosed gRPC clients may cause the remote end to see abnormal disconnections. It is recommended to register a close function for every non-ephemeral resource in `configureShutdown`.

### What if closeTimeout is too short?

If a resource (e.g., database) does not finish closing within `closeTimeout`, `context.DeadlineExceeded` causes that resource's close to fail. `afterStop` logs an ERROR and continues to the next resource. Setting a reasonable `closeTimeout` (default 5 seconds) is usually sufficient.

### What is the relationship between stopTimeout and drainTimeout?

`stopTimeout` is the total EGO framework stop timeout. `drainTimeout` is one phase within it. The actual time available for server shutdown and resource cleanup is approximately `stopTimeout - drainTimeout`. If `drainTimeout` is too large (close to `stopTimeout`), the server shutdown and resource cleanup phases may be compressed or timeout.

::: danger Configuration to Avoid
Do not set `drainTimeout` to equal or exceed `stopTimeout`. While the normalization logic will clamp it, this usually indicates a misconfiguration.
:::

### What should I watch for in multi-instance deployments?

During Kubernetes rolling updates, old and new instances briefly coexist. After the old instance enters the drain phase, `/readyz` returns 502, but old connections may still have in-flight requests. Ensure `drainTimeout` is long enough for all in-flight requests to complete. For long-running requests (e.g., file uploads), adjust `stopTimeout` accordingly.

### How do I customize the health check function?

The first argument to `health.Start` is a `CheckFn` that returns `true` when healthy. A typical implementation:

```go
func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
    return health.Start(func() bool {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
        defer cancel()

        dbx, err := db.DB()
        if err != nil {
            return false
        }
        if err = dbx.PingContext(ctx); err != nil {
            return false
        }
        if _, err = rds.Ping(ctx); err != nil {
            return false
        }
        return true
    }, c)
}
```

The health check function is called regardless of whether the service is in ready or not-ready state. `/healthz` always invokes this function and is not affected by the readiness flag.

## Reference Links

- `internal/platform/shutdown/manager.go` -- Manager core logic
- `internal/platform/shutdown/config.go` -- Config definition and defaults
- `internal/platform/shutdown/provider.go` -- Wire ProviderSet
- `internal/platform/shutdown/manager_test.go` -- Unit tests
- `internal/platform/health/health.go` -- Readiness and healthz endpoints
- `internal/component/idgen/shutdown.go` -- StopMachineLeaseBestEffort
- `internal/component/idgen/machine_manager.go` -- StopWithoutRelease
- `internal/app/user/server/shutdown.go` -- user service configureShutdown example
- `internal/app/gateway/server/shutdown.go` -- gateway service configureShutdown example
- `internal/app/user/server/server.go` -- user service newApp lifecycle
