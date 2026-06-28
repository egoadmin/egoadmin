# Server Runtime

Read this before changing process startup, EGO app assembly, gRPC/HTTP/governor registration, middleware order, migrations, readiness, health checks, upload, embedded web, cron/jobs, registry/discovery, graceful shutdown, or runtime initialization.

## Evidence Paths

- `cmd/gateway`, `cmd/user`
- `internal/app/gateway/server/**`
- `internal/app/user/server/**`
- `internal/platform/health/**`
- `internal/platform/shutdown/**`
- `internal/platform/discovery/**`
- `internal/platform/config/**`
- `atlas/apply.sh`
- `cmd/gateway/Dockerfile`

## Runtime Responsibilities

Each service runtime owns:

- EGO app construction and server registration.
- gRPC/HTTP/governor server setup.
- Registry/discovery wiring.
- Runtime migration order.
- Middleware order and allowlists.
- Generated service registration.
- Health/readiness state.
- Graceful shutdown binding, readiness drain, and non-server resource cleanup.
- Cron/job lifecycle when used.

Gateway additionally owns:

- External HTTP compatibility entry.
- Embedded frontend serving and SPA fallback.
- Upload/TUS upload registration.
- API catalog and permission aggregation when gateway-facing.

User owns user-domain gRPC services and user-domain startup requirements such as auth/session dependencies and built-in data needed by the user domain.

## Startup Rules

- Run migrations before any startup logic touches tables.
- Build Casbin/permission state after schema is ready.
- Register generated gRPC services before API discovery or permission checks rely on them.
- Mark readiness only after startup hooks, migrations, critical dependencies, upload/static web registration, and permission checks have completed.
- Bind graceful shutdown before serving the app and before returning the service `App`.
- Keep optional components optional. Do not make absent optional dependencies break default startup.
- Every microservice must start an HTTP server that exposes `/healthz` and `/readyz`, even when it has no business HTTP API.
- Health-only HTTP servers must register only health/readiness routes. Do not add auth, permission, validation, ecode, access log, metrics, trace, or business middleware to health-only HTTP servers.

## Middleware Order

Preserve equivalent intent across gRPC and HTTP compatibility paths:

1. Recovery.
2. Auth session.
3. Permission.
4. Validator.
5. Ecode/error normalization.

Public/login-only/protected classifications must be consistent between middleware, OpenAPI docs, and permission behavior.

Always allow gRPC health checks through auth/permission middleware.

## Graceful Shutdown

For detailed rules, read [graceful-shutdown.md](graceful-shutdown.md).

- Use `internal/platform/shutdown.Manager` as the single process-level shutdown coordinator.
- `beforeStop` must mark readiness down before drain sleep. Do not accept new traffic while the process is draining.
- Register non-EGO resources with `Register`, `RegisterCloser`, `RegisterDB`, or `RegisterRegistry`; EGO servers remain owned by `opts.app.Serve(...)`.
- Resources close in reverse registration order. Register dependencies first and dependent clients/workers later so workers stop before their clients and databases close.
- Keep expected dependency-down cleanup best-effort. For example, idgen machine lease shutdown should stop local renewal without requiring a remote `ReleaseLease` when idgen is stopping in the same `make run`.
- Do not reintroduce service-local `App.Run` defer cleanup for DB, registry, Redis, clients, idgen, cron, or other process resources.
- `make run` returning `130` after `Ctrl+C` is normal. Avoid treating that shell exit code as an application shutdown failure.

## gRPC And HTTP Registration

- Register generated gRPC services in the owning service's `server` package.
- Register HTTP compatibility only for APIs that need gateway/external access.
- Internal-only APIs should stay gRPC-only unless a frontend or external client explicitly needs HTTP compatibility.
- `/healthz` and `/readyz` must be HTTP endpoints without authentication or permission checks so Kubernetes, probes, load balancers, and operations tooling can call them.
- Services without business HTTP APIs still need an HTTP server for health checks; create a health-only server and register no other routes or middleware. Its `server.http` config should disable access, metric, and trace interceptors because probes should not enter the business middleware chain.
- Do not hand-edit generated registration code or Wire output.

## Migration Runtime

Rules:

- Atlas is the versioned migration path.
- `EGOADMIN_ATLAS_MIGRATED=true` means an entrypoint or e2e runner already applied migrations; runtime should not apply them again.
- `ATLAS_URL` or service config migration URL is required when Atlas migration is enabled.
- Migration directories must match service database boundaries.

## Registry And Discovery

- Set `EGO_NAME` before process startup.
- Use stable configured ports, not port `0`, for registered services.
- Register resolver support before building `etcd:///...` clients.
- Validate discovery with e2e/local smoke when service names, registry config, or client addresses change.

## Access Logs

- Keep success-only operation logging at controller/request edge where existing code does so.
- Do not log secrets, passwords, tokens, password ciphers, private keys, challenge IDs, captcha codes, or large payloads.
- If logging becomes async, preserve shutdown behavior and failure visibility.

## Upload And Embedded Web

- Gateway owns upload and embedded web startup.
- Do not treat `/upload` as a normal proto API.
- Do not edit `web/dist` by hand; build from frontend sources.
- Runtime frontend config injection must not leak secrets.

## Validation

- `make gen`
- `go test -race ./internal/app/gateway/server ./internal/app/user/server`
- `make service.check SERVICE=gateway`
- `make service.check SERVICE=user`
- `make e2e E2E_TIMEOUT=20m` when startup, registry, migration, gateway HTTP, upload/static web, auth, or permission behavior changes.
