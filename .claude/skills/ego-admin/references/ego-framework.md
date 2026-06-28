# EGO Framework And Components

Read this before integrating third-party middleware, adding EGO components, changing server/cron/job lifecycle, EGO config, logs, metrics, tracing, registry, discovery, or health behavior.

## Evidence Paths

- `internal/component/**`
- `internal/platform/**`
- `internal/app/<service>/server/**`
- `configs/<service>/**`

## Component Pattern

Use EGO-style components for dependencies with:

- runtime config.
- connections or clients.
- lifecycle start/stop.
- health/readiness behavior.
- logging, metrics, tracing, or background workers.

Component packages live in `internal/component`. Shared adapters live in `internal/platform`. Service-specific business wrappers live in `internal/app/<service>/service` or the owning package.

## Rules

- Keep reusable components independent of app service packages.
- Expose narrow interfaces to business services.
- Load runtime config through component containers or typed platform config.
- Register lifecycle-owned work through EGO server/cron/job paths, not unmanaged goroutines.
- Do not make optional dependencies mandatory by default.
- Do not log secrets or large payloads.

## Registry And Discovery

EGO registry/discovery configuration is part of service runtime. Changes to service names, resolver registration, registry config, or client addresses must be validated with microservice smoke/e2e.

## Cron And Jobs

- Register scheduled work through EGO lifecycle.
- Use distributed locks where overlap would be unsafe.
- Keep service-owned jobs inside the owning service tree.

## Validation

- `go test -race ./internal/component/<name>/...`
- `go test -race ./internal/platform/...`
- `make gen` when providers change.
- `make e2e E2E_TIMEOUT=20m` when lifecycle, discovery, or user-visible component behavior changes.
