---
name: ego-admin
description: Guidance for developing the current EgoAdmin microservice template and derived projects. Use when modifying Go backend, Vue frontend, protobuf contracts, OpenAPI annotations, validation tags, controller/service/store flows, GORM/MySQL repositories, gateway/user service runtime, graceful shutdown/readiness drain, service-to-service gRPC clients, EGO registry/discovery, DTM/distributed transactions, Saga, TCC, transactional message, barrier, compensation, authsession/logincrypto, idgen/idcodec, data permissions/DataScope, backend or frontend i18n/localization, internal/platform infrastructure, internal/component wrappers, runtime config/env overrides, permissions/API manifests/routeMenu, Atlas service migrations, Docker Compose middleware, e2e tests, build, deploy, or template rename/init tooling.
---

# EgoAdmin

## Overview

Use this skill as the development guide for the current EgoAdmin microservice template. Treat the repository code as authoritative. If this skill and code disagree, inspect the code path and update the skill or implementation deliberately; do not fall back to the old monolith layout.

Current default services:

- `gateway`: external HTTP compatibility entry, embedded web, upload, API/permission aggregation.
- `user`: user, role, department, auth/session, permission, audit log domain.

Current structural roots:

- `cmd/<service>` and `configs/<service>`.
- Target layout for new/refactored service internals: `internal/app/<service>/{server,controller,application,domain,adapter,schema}`.
- Migration-era layout: `internal/app/<service>/{server,controller,service,internal/store,schema}`. Keep it working while existing aggregates move; do not expand it for new domain work when a DDD package is practical.
- `internal/platform/*` for shared infrastructure.
- `internal/component/*` for reusable EGO-style components.
- `internal/client/*` for service-to-service gRPC clients.
- `atlas/migrations/<service>` for database-boundary migrations.

## Required Workflow

1. Identify touched layers: proto/API, gateway/user runtime, controller, service, store, platform infrastructure, component, service client, auth/session, permission, data permission, i18n, frontend, migration, Docker/local middleware, tests, build/deploy, or template tooling.
2. Read every relevant reference before editing. Do not implement from memory.
3. For cross-layer work, read the full chain. A field, API, permission, migration, or service call usually crosses several files.
4. Preserve current microservice boundaries. Do not introduce old monolith paths or transitional aliases.
5. For any complete user-visible feature or cross-service behavior, decide whether e2e coverage is required. Add or update e2e unless the handoff explicitly proves existing tests cover it or the change cannot affect an end-to-end path.
6. Before finishing, run validation from [validation.md](references/validation.md) that matches the changed layers and report what did not run.

## Reference Router

- Read [microservice-architecture.md](references/microservice-architecture.md) before adding services, moving ownership, changing service discovery, service clients, local middleware, or e2e topology.
- Read [backend-architecture.md](references/backend-architecture.md) for service/controller/store/platform dependency direction.
- Read [server-runtime.md](references/server-runtime.md) before changing process startup, EGO app assembly, gRPC/HTTP/governor registration, readiness, migrations, upload, embedded web, or health checks.
- Read [graceful-shutdown.md](references/graceful-shutdown.md) before changing shutdown hooks, readiness drain, app stop timeout, resource close order, `Close()` methods, idgen machine lease shutdown, or Ctrl+C/run lifecycle behavior.
- Read [proto-contract.md](references/proto-contract.md) before changing proto files, comments, validator tags, copier tags, OpenAPI options, HTTP compatibility mappings, or generated errors.
- Read [backend-api-flow.md](references/backend-api-flow.md) before implementing controller/service/store request-response flow.
- Read [gorm-mysql.md](references/gorm-mysql.md), then [gorm-models.md](references/gorm-models.md), [gorm-queries.md](references/gorm-queries.md), or [gorm-transactions-conversion.md](references/gorm-transactions-conversion.md) before changing store models, repositories, queries, transactions, or DB-to-RPC conversion.
- Read [data-migration.md](references/data-migration.md) before changing schema, Atlas migrations, migration generation, or runtime migration behavior.
- Read [auth-session.md](references/auth-session.md) before changing logincrypto, authsession, Bearer middleware, logout, refresh, forced offline, captcha, password transport, or auth allowlists.
- Read [api-permission.md](references/api-permission.md) before changing auth packs, Casbin, `RegisterAPIs`, API manifests, routeMenu, permission contracts, roles, menus, or buttons.
- Read [data-permission.md](references/data-permission.md) before changing role data permissions, `DataScope`, user/role/dept/log visibility, ownership fields, or data-permission query filters.
- Read [i18n.md](references/i18n.md) before changing backend user-facing messages, localized EGO errors, frontend labels, route/menu titles, language switching, or `Accept-Language` propagation.
- Read [config-wire-components.md](references/config-wire-components.md) before changing runtime config, env overrides, provider sets, Wire, Redis/MinIO/Resty, service config, or component wiring.
- Read [dtm-distributed-transactions.md](references/dtm-distributed-transactions.md) before adding or changing DTM, distributed transactions, Saga, TCC, transactional messages, barrier tables, compensation/cancel/confirm handlers, or cross-service write consistency.
- Read [ego-framework.md](references/ego-framework.md) and [component-catalog.md](references/component-catalog.md) before adding or changing EGO components or third-party middleware.
- Read [upload-static-web.md](references/upload-static-web.md) before changing upload, TUS upload, embedded `web/dist`, SPA fallback, or runtime frontend config.
- Read [frontend.md](references/frontend.md) before changing Vue pages, API modules, router, routeMenu, store, permissions, or frontend build behavior.
- Read [toolchain-build-deploy.md](references/toolchain-build-deploy.md) before changing Make targets, Buf/Wire/OpenAPI generation, Docker, Compose, CI, deploy, release, or tools.
- Read [testing-quality.md](references/testing-quality.md) before adding or changing unit, integration, e2e, permission-boundary, component, or frontend tests.
- Read [validation.md](references/validation.md) before reporting completion.
- Read [template-project.md](references/template-project.md) when changing template rename/init behavior or derived-project guidance.

## Cross-Layer Entry Points

- New user-facing API: read proto, backend flow, backend architecture, server runtime, permission, frontend, testing, and validation references. Implement proto, store/service/controller, server registration, permission/menu/frontend contract, and tests as one coherent change.
- New persisted domain: read proto, GORM/store, migration, backend flow, server runtime, config if needed, testing, and validation. Prefer service-owned `domain`, `application`, and `adapter/persistence/mysql` packages; use existing `internal/store` only for migration-era code or while incrementally moving an existing aggregate.
- New service or service-to-service call: read microservice, proto, backend, runtime, config/Wire, migration, permission, toolchain, testing, and validation references. Decide ownership, client wrapper, registry/discovery, database boundary, migration path, and e2e impact before editing.
- New cross-service write transaction: read DTM, microservice, backend, proto, GORM transaction, config/Wire, toolchain, testing, and validation references. Decide whether DTM is necessary, choose Saga/Msg/TCC deliberately, configure DTM server and branch targets explicitly, and add integration/e2e coverage.
- Auth/session/permission behavior: read auth-session, api-permission, server-runtime, frontend, testing, and validation. e2e is normally required.
- Data permission behavior: read data-permission, api-permission, backend flow, GORM queries, frontend if visible, testing, and validation. Cover admin/root bypass and ordinary-user denial paths.
- Internationalized user-facing behavior: read i18n, frontend, backend flow, service clients, testing, and validation. Keep backend and frontend language packages in sync.
- Runtime lifecycle change: read server runtime, graceful shutdown, config/Wire/components, testing, and validation. Preserve readiness drain, reverse close order, EGO server ownership, and best-effort shutdown semantics for expected dependency-down cases.
- Build/deploy/local middleware change: read toolchain, data-migration if migrations are involved, upload-static-web if `web/dist` is packaged, testing, and validation.

## Non-Negotiable Rules

- Start API work from proto. Do not add ad hoc Gin routes for business APIs or hand-edit generated files.
- Treat HTTP as gRPC compatibility, not REST-first design. Use gRPC method identity and POST body request messages.
- Use the current microservice layout. Do not add old monolith entrypoints or public compatibility aliases.
- Keep service data ownership local. Service business code uses its own `internal/store`; cross-service reads/writes go through `internal/client/*` gRPC wrappers.
- For new/refactored service internals, keep domain pure: domain packages define aggregates, value objects, domain services, errors, and Repository interfaces; they must not import protobuf, GORM, EGO/Gin, Redis, MinIO, Casbin adapters, or other service app packages.
- Application packages own use-case orchestration, transactions, permission/session side effects, component coordination, and cross-service calls through `internal/client/*`.
- For distributed transactions, application packages own DTM Saga/Msg/TCC orchestration. Controllers do not orchestrate DTM; domains do not import DTM SDKs; persistence adapters do not start global transactions.
- Persistence adapters implement domain repositories and own GORM models, scopes, model conversion, and migration model lists.
- Do not scatter `egrpc.Load(...)` or generated client construction inside business services.
- Use `authsession` and `logincrypto` components for auth flows; do not reintroduce raw JWT parsing, MD5, plaintext password transport, or service-local token stores.
- Use `DataScope` helpers for role/user/dept/log data permission. Built-in admin/root users bypass data permission; ordinary users must be denied by default outside their scope.
- Use `internal/platform/i18n` and frontend `vue-i18n` packages for user-facing text. Do not add raw localized strings in new service/controller/frontend code.
- Use `internal/platform/shutdown.Manager` for non-server resource cleanup. Do not reintroduce service-local `App.Run` defer cleanup for process resources.
- Keep runtime config per service and minimal. Use env overrides for secrets and deployment-specific values.
- Keep permission changes closed over backend and frontend: gRPC method, API catalog, routeMenu, permission contract, page/button checks, and role-boundary validation.
- Use Atlas migrations per database boundary. Do not use a service model source against the wrong database.
- Use `idcodec` only for reversible public IDs over internal numeric IDs. It is not authorization.
- Complete user-visible functionality must be evaluated for e2e. For gateway-facing admin workflows, e2e should enter through `test/e2e/gateway`.
