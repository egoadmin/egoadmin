# Feature Overview

This page is the capability map for EgoAdmin. Use it to locate the right modules, commands and deeper guides before changing code.

## Capability Map

| Area | Main Paths | Command | Guide |
|------|------------|---------|-------|
| Runtime | `cmd/*`, `internal/app/*/server`, `configs/*` | `make run SERVICE=user` | [Architecture](/en-US/guide/architecture) |
| API development | `api/proto`, `controller`, `application`, `adapter` | `make gen` | [API Workflow](/en-US/guide/api-development) |
| Permission | `authsession`, `routeMenu.ts`, `api-manifest.ts` | `cd web && pnpm run build` | [Permission System](/en-US/guide/permission-system) |
| Database | `schema`, `atlas/migrations/*` | `make migrate.new SERVICE=user NAME=xxx` | [Database & Migrations](/en-US/guide/database-migration) |
| Frontend | `web/src/views`, `web/src/api`, `web/src/router` | `pnpm run type-check` | [Frontend](/en-US/guide/frontend-development) |
| DTM | `application`, branch proto APIs | `make e2e E2E_TIMEOUT=20m` | [DTM](/en-US/guide/distributed-transactions) |
| Components | `internal/component`, `internal/platform` | `go test ./internal/component/...` | [Components](/en-US/guide/components) |
| Configuration | `configs/*/config.toml` | `make service.config SERVICE=user` | [Configuration](/en-US/guide/configuration) |
| Testing & deployment | `test/e2e`, `deploy`, `scripts/make` | `make e2e` | [Testing & Deployment](/en-US/guide/testing-deployment) |

## Services

| Service | Responsibility |
|---------|----------------|
| `gateway` | external HTTP compatibility entry, embedded frontend, upload, API aggregation, calls user/idgen |
| `user` | users, roles, departments, auth/session, permissions, audit logs, data scope |
| `idgen` | Snowflake IDs, segments, machine leases, namespaces, stable ID codec |

## Backend Layers

```text
internal/app/<service>/
├── server/       # runtime assembly, gRPC/HTTP/governor, migrations, health
├── controller/   # generated gRPC server implementation and protocol conversion
├── application/  # use cases, transactions, permissions, cross-service calls, DTM
├── domain/       # aggregates, value objects, domain errors, repository interfaces
├── adapter/      # MySQL/cache/object-store adapters
└── schema/       # migration model lists
```

## Proto-First APIs

Business APIs start from `.proto` files:

```protobuf
rpc GetRoleList(GetRoleListRequest) returns (GetRoleListResponse) {
  option (google.api.http) = {
    post: "/user.v1.RoleService/GetRoleList"
    body: "*"
  };
}
```

Frontend calls use the same gRPC compatibility path:

```ts
api.post('/user.v1.RoleService/GetRoleList', {
  page: 1,
  limit: 20,
})
```

## Permission Chain

```text
authsession Bearer token
  -> API classification (public / login-only / protected)
  -> Casbin gRPC service/method permission
  -> DataScope query filtering
  -> API manifest
  -> routeMenu.ts
  -> permission-contract.json
```

Permission ID format:

```text
USER.V1.ROLESERVICE/ADDROLE
```

## Validation Matrix

| Change | Minimum Validation |
|--------|--------------------|
| proto / provider | `make gen` |
| service layout | `make service.check SERVICE=user` |
| backend logic | `go test -race ./...` |
| frontend page | `cd web && pnpm run type-check` |
| permission / routeMenu | `cd web && pnpm run build` |
| migrations | `make migrate.validate SERVICE=user` |
| user-facing gateway flow | `make e2e E2E_TIMEOUT=20m` |
