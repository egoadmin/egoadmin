# Microservice Architecture

Read this before adding services, changing gateway/user boundaries, service discovery, service-to-service gRPC clients, service-owned store packages, Atlas service migrations, Docker Compose middleware, or e2e topology.

## Evidence Paths

- `cmd/gateway`, `cmd/user`
- `configs/gateway`, `configs/user`
- Target layout: `internal/app/<service>/{server,controller,application,domain,adapter,schema}`
- Migration-era layout: `internal/app/<service>/{server,controller,service,internal/store,schema}`
- `internal/platform/**`
- `internal/component/**`
- `internal/client/**`
- `atlas/migrations/gateway`, `atlas/migrations/user`
- `test/docker-compose.yml`, `test/compose/**`, `test/e2e/gateway/**`
- `.agents/skills/ego-admin/references/dtm-distributed-transactions.md`
- `Makefile`

## Current Topology

EgoAdmin is currently a microservice template, not a single-process template.

- `gateway` owns external HTTP compatibility, embedded frontend serving, upload entrypoints, API/permission aggregation, and calls downstream services through gRPC.
- `user` owns users, roles, departments, auth/session, permission validation, audit logs, and user-domain persistence.
- Services run as independent processes with independent configs and provider graphs.
- Service-to-service traffic uses gRPC. Do not add HTTP typed clients for internal calls unless there is a deliberate external contract.
- Every service exposes unauthenticated HTTP `/healthz` and `/readyz` endpoints for Kubernetes and operations probes, even when the service has no business HTTP API.
- If a service only exposes HTTP for health checks, the HTTP server must not register auth, permission, validation, ecode, access log, metrics, trace, or business middleware.

## Directory Contract

Use this shape for every service:

```text
cmd/<service>
configs/<service>
internal/app/<service>/server
internal/app/<service>/controller
internal/app/<service>/application
internal/app/<service>/domain/<aggregate>
internal/app/<service>/adapter/persistence/mysql
internal/app/<service>/schema
```

Existing `service` and `internal/store` packages are migration-era packages. Keep them working while migrating existing aggregates, but prefer the DDD target shape for new or substantially refactored code.

Shared roots:

```text
internal/platform        # shared infrastructure adapters and config
internal/component       # reusable EGO-style components
internal/client          # generated gRPC client wrappers
tools                    # project tools such as atlasloader and egoadminctl
atlas/migrations/<svc>   # migration dirs by database boundary
```

Do not create new code in old monolith roots as a compatibility shortcut. If old paths appear in historical notes, mark them as old paths and do not recommend them.

## Service Ownership

Rules:

- A service may directly access only its owned domain/repository/persistence packages.
- A service must not import another service's `internal/store`, `adapter`, or domain internals.
- Cross-service reads and writes go through an `internal/client/<target>client` wrapper.
- Cross-service write consistency uses DTM only when local transactions are not enough; read `dtm-distributed-transactions.md` first.
- Domain packages own aggregates, value objects, domain services, domain errors, and Repository interfaces.
- Application packages own use-case orchestration, transactions, permission/session side effects, and service-to-service calls.
- Persistence adapters own GORM models, repository implementations, scopes, conversion helpers, and migration model lists for that service.
- Migration-era store packages may keep existing GORM models and repositories until the aggregate is moved.
- `internal/platform/database/mysql` owns shared DB plumbing: GORM setup, transaction context, migration helpers, and base behavior.
- `internal/platform` must not import `internal/app/<service>`.
- `internal/platform/cache/redis` and related platform packages own reusable infrastructure, not service business rules.

Valid dependency example:

```text
internal/app/gateway/service
  -> internal/client/userclient
  -> internal/platform/*
```

```text
internal/app/user/service
  -> internal/app/user/internal/store
  -> internal/component/authsession
  -> internal/component/logincrypto
```

Target DDD dependency example:

```text
internal/app/user/application
  -> internal/app/user/domain/user
  -> internal/component/authsession
  -> internal/platform/database/mysql
```

```text
internal/app/user/adapter/persistence/mysql
  -> internal/app/user/domain/user
  -> internal/platform/database/mysql
```

Invalid dependency example:

```text
internal/app/gateway/service -> internal/app/user/internal/store
```

## Service-To-Service gRPC Clients

Use generated gRPC clients behind thin wrappers under `internal/client`.

Rules:

- Keep connection construction, generated client construction, narrow interfaces, and provider wiring inside the client wrapper.
- Calling services depend on interfaces so tests can replace remote calls.
- Propagate `context.Context` through every call.
- Keep addresses in config, not business code.
- Do not scatter `egrpc.Load(...)` or `NewXxxServiceClient(...)` inside service methods.

Config example:

```toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "5s"
```

## EGO Registry And Discovery

Microservices must set explicit service names before process startup:

```bash
EGO_NAME=egoadmin-gateway
EGO_NAME=egoadmin-user
```

Rules:

- Do not set `EGO_NAME` inside `main()` and expect package initialization to observe it.
- Do not use dynamic port `0` for registered services.
- Register etcd/egrpc resolvers before building clients that use `etcd:///...`.
- Prove discovery with local smoke/e2e when changing registry, service names, or client config.

## Atlas Migration Boundaries

Migration directories follow database boundaries:

```text
atlas/migrations/gateway -> egoadmin_gateway
atlas/migrations/user    -> egoadmin_user
```

Rules:

- Generate migrations with `make migrate.new SERVICE=<service> NAME=<change_name>`.
- Rehash with `make migrate.hash SERVICE=<service>`.
- Apply with `make migrate.apply SERVICE=<service> ATLAS_URL=<database-url>`.
- Atlas does not create databases. Local/e2e Compose must create service databases first.
- Do not apply a service migration directory to another service database.
- Commit migration SQL and the service `atlas.sum` together.

## API Catalog And Permission Ownership

Gateway-facing admin APIs must stay closed over backend and frontend:

- gRPC service/method registration.
- API catalog generation.
- `web/src/api/api-manifest.ts`.
- `web/src/config/routeMenu.ts`.
- `web/dist/permission-contract.json`.
- Role API boundary checks.

Internal-only gRPC services must not mutate the frontend permission catalog unless they are intentionally part of the gateway-facing management API.

## Local Docker Compose Middleware

Development middleware should run locally through Docker Compose and be split by service when ownership differs:

- service-specific MySQL databases.
- service-specific Redis instances or keys when state ownership differs.
- shared etcd, MinIO, Jaeger, DTM, and optional services when appropriate.

DTM rules:

- `dtm` is a shared middleware service, not an EgoAdmin business microservice.
- `mysql-dtm` owns DTM transaction state such as `egoadmin_dtm`.
- Business services create `dtm_barrier` only when they participate in DTM local branch transactions.
- DTM server uses `dtm-driver-ego` and etcd to discover business branch services.
- Branch target addresses are explicit config such as `etcd:///egoadmin-user`; do not infer them from `ClientConn.Target()` or `egrpc.Load()`.

Do not depend on shared external servers for ordinary local development or e2e.

## e2e Topology

For admin user workflows, e2e enters through gateway:

```text
test client -> gateway HTTP compatibility -> user gRPC via discovery -> service DB/Redis
```

Rules:

- Use real service processes.
- Use temporary local Docker Compose middleware.
- Run Atlas per service database.
- Verify gRPC health, gateway readiness, auth/session, permission, CRUD, logs, and cleanup when affected.
- Add `test/e2e/<service>` only when that service has an independent external entry or public gRPC consumer.

## New Service Checklist

1. Add proto package and generated code.
2. Add `cmd/<service>` and `configs/<service>`.
3. Add `internal/app/<service>/{server,controller,application,domain,adapter,schema}`.
4. Add provider sets and Wire graph.
5. Add service-specific migrations.
6. Add client wrapper in `internal/client/<service>client` for callers.
7. Add service config and local Compose middleware.
8. Add gRPC server and an HTTP health server exposing unauthenticated `/healthz` and `/readyz`. Health-only HTTP servers must keep HTTP interceptors disabled.
9. Add `make service.check SERVICE=<service>` coverage.
10. Decide gateway exposure, API catalog ownership, frontend permissions, and e2e path.

## Validation

- `make service.check SERVICE=gateway`
- `make service.check SERVICE=user`
- `make gen`
- `go test -race ./...`
- `make e2e E2E_TIMEOUT=20m` when service discovery, gateway/user contracts, migrations, auth, permission, or user-visible workflows change.
