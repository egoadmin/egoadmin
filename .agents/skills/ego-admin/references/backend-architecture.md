# Backend Architecture

Read this before changing service layout, controllers, services, store packages, platform infrastructure, client wrappers, provider sets, or dependency direction.

## Evidence Paths

- `cmd/gateway`, `cmd/user`
- Target layout: `internal/app/<service>/{server,controller,application,domain,adapter,schema}`
- Migration-era layout: `internal/app/<service>/{server,controller,service,internal/store,schema}`
- `internal/platform/**`
- `internal/component/**`
- `internal/client/**`
- `api/proto/**`, `api/proto-internal/**`

## Layer Responsibilities

`cmd/<service>` is a thin entrypoint. Keep business logic and dependency assembly in `internal/app/<service>`.

For new or refactored code, use the service-local DDD shape:

```text
internal/app/<service>/server
internal/app/<service>/controller
internal/app/<service>/application
internal/app/<service>/domain/<aggregate>
internal/app/<service>/adapter/persistence/mysql
internal/app/<service>/schema
```

Existing `service` and `internal/store` packages are migration-era packages. They may remain while an aggregate is being moved, but do not grow them for new domain work when a DDD package is practical.

`server` owns process-level runtime:

- EGO app, logging, config loading, registry/discovery, health/readiness.
- gRPC, HTTP compatibility when exposed, governor, cron/jobs.
- Atlas migration sequencing.
- Generated service registration.
- Middleware order and allowlists.
- Gateway-only embedded web/upload/API aggregation behavior.

`controller` implements generated gRPC server interfaces:

- Read request fields with `in.GetXxx()`.
- Let validator middleware enforce generated tags.
- Convert proto request data into application commands/queries.
- Call application use cases or migration-era services.
- Record operation logs where the existing flow does so.
- Convert application results into proto responses.
- Avoid direct GORM model construction in new/refactored code.

`application` owns use-case orchestration:

- Transactions, locks, duplicate checks, delete guards, status transitions.
- Permission and role boundary validation.
- Token/session side effects.
- Cross-service orchestration through `internal/client/*`.
- DTM Saga/Msg/TCC orchestration when a cross-service write truly needs distributed transaction coordination.
- Coordination of domain, repositories, platform, component, cache, object storage, and async dependencies.
- Command/query/result DTOs that are independent from protobuf and GORM.

`domain/<aggregate>` owns the business model:

- Aggregates, entities, value objects, domain errors, domain services, and Repository interfaces.
- Business invariants that do not require infrastructure.
- No imports of protobuf generated code, GORM, EGO/Gin, Redis, MinIO, Casbin adapters, or other service app packages.

`adapter/persistence/mysql` owns MySQL persistence for that service:

- GORM models, repository implementations, scopes, query options.
- Transaction-aware DB access.
- Local DTM branch data changes guarded by a barrier when the service participates as an RM.
- GORM-to-domain conversion.
- Migration model lists and join-table setup.

`internal/app/<service>/internal/store` is the migration-era persistence package:

- GORM models, repository interfaces/implementations, scopes, query options.
- Migration model lists and join-table setup.
- Transaction-aware DB access.
- DB-to-RPC conversion helpers used by copier.

`internal/platform` owns shared infrastructure such as config, database, cache, discovery, health, HTTP client, object storage, ID adapters, and singleflight.

`internal/component` owns reusable EGO-style components such as authsession, logincrypto, idgen/idcodec, asyncq, jetcache, upload, and meilisearch.

`internal/client` owns service-to-service gRPC wrappers.

## Dependency Direction

Follow this direction:

- `server` depends on controllers, application/services, platform, components, clients, and EGO runtime.
- `controller` depends on application use cases or migration-era services and request-edge helpers.
- `application` depends on domain, domain Repository interfaces, platform interfaces, components, and client wrapper interfaces.
- `domain` depends only on the standard library and narrow project value objects.
- `adapter` depends on domain and platform database helpers, not on controller/application.
- Migration-era `service` depends on owned store interfaces, platform interfaces, components, and client wrapper interfaces.
- Migration-era `store` depends on platform database helpers and local schema/model code, not on controller/service.
- `client` depends on generated gRPC clients and EGO client construction.
- `proto` is source of API contracts; generated code is consumed by controller/client/server registration.

Do not introduce reverse dependencies such as store importing service, service importing controller, or one service importing another service's store.

## Feature Flow

For a new user-facing feature:

1. Define or update proto first.
2. Run generation.
3. Implement domain types and Repository interfaces for persisted aggregates.
4. Implement persistence adapters and migration model lists.
5. Implement application rules and transactions.
6. Implement controller conversion/logging.
7. Register generated services in the correct service runtime.
8. Update permission/API manifest/frontend when gateway-facing.
9. Add unit/integration tests and evaluate e2e.

For internal-only service APIs, document the boundary and avoid frontend/permission catalog changes unless intentionally exposed through gateway.

For DTM/distributed transaction features:

1. Read `dtm-distributed-transactions.md`.
2. Choose Saga, TCC, or transactional message deliberately. Do not use DTM for a single-service local transaction.
3. Keep DTM orchestration in application code.
4. Configure DTM server target and branch service targets explicitly.
5. Implement branch methods in the service that owns the local data.
6. Use barrier-protected local transactions inside branch handlers.
7. Add integration or e2e coverage for success, compensation, idempotency, empty compensation, hanging, and unavailable service cases.

## Registration Checklist

- Proto service/RPC exists and generated code is refreshed.
- Service-owned store constructor and migration model list are complete.
- Service constructor is in the service provider set.
- Controller constructor is in the controller provider set.
- Server registers generated gRPC service and HTTP compatibility server when needed.
- Auth allowlists and permission checks classify the API correctly.
- Gateway-facing protected APIs update API catalog, routeMenu, and permission contract.
- Wire output is regenerated from source provider sets.
- Complete user-visible behavior has e2e coverage or a documented reason it does not need new e2e.

## Do Not

- Do not add business routes by hand outside proto/gRPC.
- Do not add transitional aliases to old monolith paths.
- Do not use a shared store package for all services.
- Do not bypass `internal/client/*` for cross-service calls.
- Do not infer DTM branch URLs from `ClientConn.Target()` or `egrpc.Load()`.
- Do not patch generated code or Wire output as the lasting fix.
- Do not put durable business rules in controllers.
- Do not import app service packages from `internal/platform`.
- Do not import protobuf/GORM/EGO infrastructure from domain packages.

## Validation

- Generation/provider changes: `make gen`.
- Broad backend confidence: `go test -race ./...`.
- Service structure: `make service.check SERVICE=<service>`.
- Gateway-facing user workflows: `make e2e E2E_TIMEOUT=20m` when behavior changes.
