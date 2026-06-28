# Graceful Shutdown

Read this before changing shutdown hooks, readiness drain, stop timeouts, resource close order, service `App` lifecycle, `Close()` methods, idgen machine lease shutdown, or Ctrl+C behavior.

## Evidence Paths

- `internal/platform/shutdown/**`
- `internal/platform/health/health.go`
- `internal/platform/config/**`
- `internal/app/<service>/server/shutdown.go`
- `internal/app/<service>/server/server.go`
- `internal/client/*/client.go`
- `internal/component/idgen/**`
- `configs/<service>/config.toml`
- `scripts/make/service.mk`

## Runtime Contract

Each service should:

1. Build `shutdown.Manager` through platform config.
2. Call `configureShutdown(opts)` during app construction, after startup-critical components are started and before returning the app.
3. Continue using `opts.app.Serve(...)` for EGO HTTP, gRPC, and governor servers; do not close EGO servers manually.
4. Mark `health.Ready()` only after migrations, startup hooks, and critical dependency checks have completed.
5. On shutdown, mark readiness not ready first, sleep `drainTimeout`, then let EGO stop servers, then close non-server resources.

Do not reintroduce service-local `App.Run` defer cleanup for process resources. The service `App` should embed `*ego.Ego`; shutdown behavior belongs in `shutdown.Manager`.

## Configuration

Use `[app.shutdown]` in service defaults:

- `stopTimeout`: total EGO stop timeout.
- `drainTimeout`: time between readiness down and server shutdown.
- `closeTimeout`: per-resource close timeout for non-server resources.

Keep service config minimal. Put safe defaults in `internal/platform/shutdown`; expose only deliberate tuning values in defaults. Environment overrides must follow the current `EGOADMIN_*` path rules.

## Resource Registration

Register resources in dependency order because `shutdown.Manager` closes in reverse order:

- Register `registry` early so it closes late.
- Register DB/Redis/cache clients before workers that depend on them.
- Register service gRPC clients before components that use them if those components must close first.
- Use `RegisterDB("mysql", db)` for `egorm.Component`.
- Add `Close() error` to reusable clients/components when they own connections.

Only return errors that should fail shutdown. Best-effort cleanup should return nil for expected dependency-down states, after preserving enough observability.

## Idgen Machine Lease

When gateway/user use remote idgen for machine lease allocation:

- Normal `Stop(ctx)` may release the remote lease.
- Shutdown paths should call `idgen.StopMachineLeaseBestEffort(ctx, manager, fallbackTimeout)`.
- Current process managers use `StopWithoutRelease(ctx)` in shutdown, stopping local renewal without calling remote `ReleaseLease`.
- This avoids noisy `codes.Unavailable` / `no children to pick from` when `make run` stops idgen, user, and gateway together.
- The database lease TTL bounds reuse, so a shutdown path may rely on TTL expiry instead of remote release.

Do not hide startup allocation or runtime renewal failures. Fail-closed idgen behavior must still report errors while the service is running.

## Logging And Signals

- `service marked not ready` during shutdown is expected.
- `make run` exiting with code `130` after Ctrl+C is expected shell behavior.
- Avoid logging expected shutdown dependency races as ERROR.
- Do not add fragile shell process-group logic unless it is tested across `/bin/sh` and the development environment.

## Tests

Add focused tests at the owning layer:

- `internal/platform/shutdown`: readiness down, drain delay, reverse close order, joined close errors.
- `internal/platform/health`: `Ready`, `NotReady`, and readiness status behavior when changed.
- `internal/component/idgen`: normal `Stop` releases; shutdown `StopWithoutRelease` does not; best-effort helper ignores expected shutdown errors.
- Service `server` packages: compile-time/wiring tests when shutdown options or resource registration changes are non-trivial.

Run race tests for lifecycle changes because shutdown touches goroutines and atomic state.

## Validation

Minimum validation for shutdown changes:

- `go test ./internal/platform/shutdown ./internal/component/idgen ./internal/app/gateway/server ./internal/app/user/server ./internal/app/idgen/server`
- `go test -race ./internal/platform/shutdown ./internal/component/idgen ./internal/app/gateway/server ./internal/app/user/server ./internal/app/idgen/server`
- `make service.check SERVICE=gateway && make service.check SERVICE=user && make service.check SERVICE=idgen`

Run `go test ./...` when component interfaces, clients, config, or shared lifecycle code changes. Run e2e only when shutdown changes alter startup, registry/discovery, gateway HTTP behavior, auth/session, permission, migrations, or local middleware behavior.
