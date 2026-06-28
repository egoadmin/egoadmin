# Validation

Read this before reporting completion, committing, or handing off EgoAdmin changes.

For test placement and coverage decisions, read [testing-quality.md](testing-quality.md).

## Command Matrix

| Change type | Required validation | Extra checks |
| --- | --- | --- |
| Proto service/rpc/message/tag/errors | `make gen`, `buf lint` | Generated tags/docs, OpenAPI output, controller/client compile |
| Controller/service API | targeted `go test -race ./internal/app/<service>/...` | Request getters, logs, service call, response conversion |
| Store model/query | `go test -race ./internal/app/<service>/internal/store/...` | Migration review, conversion, frontend sync |
| Component | `go test -race ./internal/component/<name>/...` | defaults, lifecycle, health, close/stop |
| Platform/config | targeted `go test -race ./internal/platform/...` | env overrides, redaction, service config |
| Graceful shutdown | `go test -race ./internal/platform/shutdown ./internal/component/idgen ./internal/app/gateway/server ./internal/app/user/server ./internal/app/idgen/server` | readiness drain, close order, best-effort idgen lease shutdown, service checks |
| DTM/distributed transaction | `make dev-up`, `curl http://127.0.0.1:36789/api/ping`, `make dev-down` | fixed image check, branch discovery, barrier/integration/e2e when behavior exists |
| Auth/session/logincrypto | component tests plus user/gateway service tests | challenge, token, refresh, logout, forced offline |
| Permission/menu/API | `cd web && pnpm run build` plus service tests | API manifest, routeMenu, permission contract, e2e |
| Microservice/discovery | `make service.check SERVICE=<service>` | EGO_NAME, resolver, client wrapper, stable ports |
| Migration | `make migrate.new SERVICE=<service> NAME=<name>` | service `atlas.sum`, local apply when needed |
| Upload/static web | frontend build and gateway tests | SPA fallback, `/api` exclusion, runtime config |
| Complete gateway-facing feature | targeted tests plus `make e2e E2E_TIMEOUT=20m` | cleanup, diagnostics, real gRPC/discovery |

## Common Commands

- `make gen`
- `buf lint`
- `go test -race ./...`
- `make service.check SERVICE=gateway`
- `make service.check SERVICE=user`
- `make service.check SERVICE=idgen`
- `make e2e E2E_TIMEOUT=20m`
- `cd web && pnpm run build`
- `make build`
- `make migrate.new SERVICE=<service> NAME=<change_name>`
- `make migrate.hash SERVICE=<service>`
- `make migrate.apply SERVICE=<service> ATLAS_URL=<database-url>`

## e2e Completion Audit

Before completion, answer:

- Is this a complete user-visible feature?
- Does it touch gateway HTTP compatibility, service discovery, cross-service gRPC, auth/session, permission, migration, upload/static web, or local middleware?
- If yes, did `make e2e` run or was existing e2e coverage identified?
- Did e2e cleanup leave no `egoadmin-e2e-*` containers and no `test/e2e/.tmp` run dir?

Do not mark a full feature complete without the e2e decision.

## Proto Checks

- Forbidden RESTful mappings: `rg -n "get:|put:|delete:|\\{id\\}|\\{filename\\}" api/proto`.
- Generated tags: `rg -n "validate:|label:|copier:" api/gen/go`.
- Inspect generated errors when `errors.proto` changes.
- Inspect OpenAPI output when comments or docs options change.

## Permission Checks

- Confirm generated/updated `web/src/api/api-manifest.ts`.
- Confirm `routeMenu.ts` uses generated `APIs` constants.
- Confirm `web/dist/permission-contract.json`.
- Test non-admin backend rejection.
- Run e2e when permission behavior reaches gateway.

## Migration Checks

- Confirm model is included in owning service migration model list.
- Confirm join tables are included.
- Commit SQL and service `atlas.sum`.
- Apply to the correct service database only.

## Do Not

- Do not treat one narrow test as full-chain validation.
- Do not skip generation after proto/Wire changes.
- Do not skip frontend build when embedded web or permissions change.
- Do not skip e2e decision after complete feature work.
