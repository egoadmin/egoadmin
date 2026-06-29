# Testing & Deployment

## Test Layers

| Layer | Command | Purpose |
|-------|---------|---------|
| unit tests | `go test ./...` | domain, application, helpers |
| race tests | `go test -race ./...` | concurrency safety |
| service check | `make service.check SERVICE=user` | layout, providers, registration, health |
| frontend type check | `cd web && pnpm run type-check` | TypeScript |
| frontend build | `cd web && pnpm run build` | build and permission contract |
| e2e | `make e2e E2E_TIMEOUT=20m` | gateway user-visible flows |

## Service Check

```bash
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
```

## Build

```bash
make build
make build SERVICE=user
make build.alpine SERVICE=gateway
make build.alpine-arm64 SERVICE=idgen
```

## Images

```bash
make image.build
make image.build SERVICE=user
```

Published images:

| Image | Description |
|-------|-------------|
| `ghcr.io/egoadmin/egoadmin-gateway` | gateway with embedded frontend |
| `ghcr.io/egoadmin/egoadmin-user` | user service |
| `ghcr.io/egoadmin/egoadmin-idgen` | ID generation service |

## Deployment

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

Stop:

```bash
make deploy-down
```

## Release Checklist

```bash
make gen
make lint
go test -race ./...
cd web && pnpm run build
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
make e2e E2E_TIMEOUT=20m
```

For migration changes:

```bash
make migrate.validate SERVICE=gateway
make migrate.validate SERVICE=user
make migrate.validate SERVICE=idgen
```
