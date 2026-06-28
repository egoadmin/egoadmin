# Config, Wire, And Components

Read this before changing runtime config, service configs, env overrides, Wire provider sets, platform infrastructure, components, Redis/MinIO/Resty, upload/cache/search/async infrastructure, shutdown dependencies, or startup dependencies.

## Evidence Paths

- `configs/gateway/**`, `configs/user/**`
- `internal/platform/config/**`
- `internal/platform/**`
- `internal/component/**`
- `internal/app/<service>/server/wire.go`
- `internal/app/<service>/server/shutdown.go`
- `internal/app/<service>/**/provider*.go`

## Config Boundaries

Runtime config is service-specific:

```text
configs/gateway
configs/user
configs/<future-service>
```

Typed config and env override behavior belongs in `internal/platform/config`. Component-specific defaults belong inside component config/container code.

Rules:

- Keep runtime config minimal: environment addresses, credentials, service-specific names, and deliberate tuning.
- Put reusable safe defaults in platform/component defaults.
- Use `EGOADMIN` as the current default env prefix unless template rename changes it.
- Do not commit real secrets, production DSNs, private keys, or long-term credentials.
- Do not duplicate every default into runtime config.
- Do not scatter direct config unmarshalling through services; inject typed config or component containers.
- For `[app.shutdown]`, keep service-level config to `stopTimeout`, `drainTimeout`, and `closeTimeout`; use `internal/platform/shutdown` defaults for safe behavior.

## Environment Overrides

- Environment variables override runtime config.
- Unknown env keys must not create hidden config.
- Preserve TOML path segments when deriving env names.
- Do not split camelCase or initialisms.
- Redact secrets in any merged config output.

## Wire Rules

Provider sets define dependency graphs for each service:

- `server.ProviderSet` for runtime assembly.
- `controller.ProviderSet` for controller constructors.
- `service.ProviderSet` for service constructors.
- store/platform/component/client provider sets as needed.

After provider or constructor changes, run `make gen`. Do not patch `wire_gen.go` as the lasting fix.

## Component Boundaries

Reusable components live under `internal/component`.

Use EGO-style components for middleware or infrastructure with config, connections, lifecycle, observability, health checks, or background workers.

Rules:

- Component packages must not import app service packages.
- Optional components must not break default startup unless explicitly enabled/injected.
- Startup-critical components fail fast with clear config errors.
- Health/readiness participation must be deliberate.
- Components or clients that own connections/background workers should expose `Close() error` or an explicit stop method so `shutdown.Manager` can close them.
- Best-effort shutdown helpers belong in shared component/platform packages, not duplicated in service `server` packages.

## Current Important Components

- `authsession`: Bearer access-token validation, refresh, logout/revoke, auth context.
- `logincrypto`: RSA-OAEP-SHA256 challenge/password transport.
- `idgen` and `idcodec`: ID generation and public ID encoding.
- `shutdown`: process-level readiness drain and non-server resource cleanup.
- `asyncq`, `jetcache`, `meilisearch`, `etusupload`, `eredis`: optional or infrastructure components.

## e2e Requirement

Config, Wire, component, or service client changes need e2e when they affect startup, auth/session, upload/static web, service discovery, migrations, or gateway-facing user behavior.

Read [graceful-shutdown.md](graceful-shutdown.md) when config/Wire/component changes affect process stop behavior, close order, readiness drain, or idgen lease shutdown.

## Validation

- `make gen`
- `go test -race ./internal/platform/... ./internal/component/...`
- `make service.check SERVICE=<service>`
- `go test -race ./...`
- `make e2e E2E_TIMEOUT=20m` for startup/cross-service/user-visible config behavior.
