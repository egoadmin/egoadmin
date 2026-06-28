# Testing And Quality

Read this before adding or changing tests, permission-boundary checks, store transaction tests, component tests, frontend build checks, validation strategy, or e2e coverage.

## Evidence Paths

- `internal/app/<service>/**/*_test.go`
- `internal/component/**/*_test.go`
- `internal/platform/**/*_test.go`
- `test/e2e/gateway/**`
- `Makefile`
- `web/package.json`

## Principle

Test the layer that owns the rule:

- Proto: generation, lint, tags, OpenAPI.
- Controller: request conversion, service call, response conversion, operation logs.
- Service: business invariants, locks, transactions, permission boundaries, token/session behavior.
- Store: queries, scopes, transactions, associations, conversion helpers.
- Component/platform: config defaults, lifecycle, health, close/stop behavior.
- Frontend: type-check, build, route/menu/API contract.
- e2e: real user workflow across processes, middleware, migrations, service discovery, auth, permission, and storage.

## e2e Requirement For Complete Features

After completing a full user-visible feature or cross-service behavior, explicitly decide whether e2e is required.

Add or update e2e when a change affects:

- gateway external HTTP compatibility.
- service-to-service gRPC.
- service discovery.
- startup/shutdown lifecycle when it changes observable service availability or local multi-service behavior.
- Atlas migration/startup.
- DTM/distributed transaction middleware, branch callbacks, barrier behavior, or cross-service write consistency.
- auth/session/login/password flows.
- permission, role, menu, API catalog, routeMenu, permission contract.
- user/role/dept/log/upload/static web user workflows.
- Docker Compose local middleware needed by runtime.

For admin workflows, e2e should enter through `test/e2e/gateway`. Direct service tests do not prove gateway behavior.

If no e2e is added, handoff must state why and identify existing tests that cover the behavior.

## Service Tests

Write service tests for:

- duplicate checks.
- delete guards.
- status transitions.
- role/API boundary validation.
- lock-protected writes.
- transaction rollback.
- token refresh/logout/forced offline.
- auth snapshot cache invalidation.
- cross-resource rules.

## Store Tests

Write store tests for:

- transaction propagation.
- association updates.
- pagination and stable ordering.
- scopes/filtering.
- duplicate/existence queries.
- zero-result behavior.
- conversion helpers.

## Auth And Permission Tests

Test login/session changes with success and failure paths. Permission behavior should include backend rejection, not just frontend hidden buttons.

## DTM Tests

Complete DTM features require integration or e2e coverage for:

- success path.
- business failure and compensation.
- duplicate branch requests.
- empty compensation.
- hanging/suspended branch behavior.
- DTM unavailable.
- branch service unavailable.

For local middleware-only DTM changes, run a smoke test proving `dtm`, `mysql-dtm`, and `etcd` start and DTM responds on `/api/ping`.

## Graceful Shutdown Tests

Read [graceful-shutdown.md](graceful-shutdown.md) before changing shutdown behavior. Cover readiness down/drain, reverse close order, close error aggregation, client/component `Close()` methods, and idgen machine lease best-effort shutdown. Use race tests for lifecycle code.

## e2e Quality Rules

- Use `//go:build e2e`.
- Ordinary `go test ./...` must not start e2e.
- `make e2e` should be one command from temporary middleware to cleanup.
- Use fresh temporary Docker Compose data.
- Verify cleanup: no `egoadmin-e2e-*` containers and no `test/e2e/.tmp` run dir on success.
- Print diagnostics before cleanup on failure.

## Validation Matrix

| Change | Minimum check |
| --- | --- |
| Proto/API | `make gen`, `buf lint`, generated tag/docs inspection |
| Service/controller | targeted `go test -race ./internal/app/<service>/...` |
| Store/model/query | `go test -race ./internal/app/<service>/internal/store/...`, migration review |
| Component/platform | `go test -race ./internal/component/<name>/...` or platform package tests |
| Graceful shutdown | `go test -race ./internal/platform/shutdown ./internal/component/idgen ./internal/app/gateway/server ./internal/app/user/server ./internal/app/idgen/server` |
| Permission/menu/API | frontend build plus service tests and likely e2e |
| Microservice/discovery | `make service.check SERVICE=<service>` and e2e when runtime path changes |
| DTM/local middleware | `make dev-up`, DTM ping, `make dev-down`; add e2e/integration for complete transaction behavior |
| Complete gateway-facing feature | targeted tests plus `make e2e E2E_TIMEOUT=20m` |

## Do Not

- Do not skip tests because a change is "just wiring".
- Do not mock away transaction/discovery behavior you need to prove.
- Do not leave permission contract unbuilt after routeMenu/API changes.
- Do not report completion without stating validations run and e2e decision.
