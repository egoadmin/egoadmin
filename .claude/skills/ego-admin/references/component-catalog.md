# Component Catalog

Read this before using or changing existing components, choosing component boundaries, or adding third-party middleware.

## Evidence Paths

- `internal/component/authsession`
- `internal/component/logincrypto`
- `internal/component/idgen`
- `internal/component/asyncq`
- `internal/component/eredis`
- `internal/component/jetcache`
- `internal/component/meilisearch`
- `internal/component/etusupload`
- `internal/platform/**`
- `internal/platform/shutdown/**`

## Catalog

- `authsession`: access/refresh token validity, logout/revoke, forced offline, auth context middleware.
- `logincrypto`: RSA-OAEP-SHA256 challenges and encrypted password/action payloads.
- `idgen`: segment ID generation with machine lease behavior.
- `idcodec`: reversible public string IDs for existing numeric IDs; not authorization.
- `shutdown`: process-level readiness drain, EGO stop timeout binding, reverse-order non-server resource cleanup.
- `eredis`: Redis component/client foundation.
- `jetcache`: cache abstraction when local/remote cache behavior is needed.
- `asyncq`: async task client/server.
- `meilisearch`: search client.
- `etusupload`: TUS resumable upload support.

## Selection Rules

- Use an existing component before adding a new library wrapper.
- Add a component when config, connection lifecycle, health, observability, or workers are involved.
- Keep service-specific business behavior out of reusable components.
- Inject components only where needed.
- For lifecycle-aware components, define clear start/stop/close semantics and register them through `shutdown.Manager`.
- For shutdown-only best-effort cleanup, prefer shared helpers like idgen machine lease best-effort stop over service-local duplicate code.

## Validation

- Component package tests: `go test -race ./internal/component/<name>/...`
- Shutdown platform tests: `go test -race ./internal/platform/shutdown`
- Provider changes: `make gen`
- Startup/e2e when component changes affect runtime or user-visible behavior.
