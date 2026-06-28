# API Permission Contract

Read this before changing authorization, API registration, auth allowlists, route permissions, `api-manifest.ts`, `routeMenu`, permission contracts, role APIs, menus, or page/button permissions.

## Evidence Paths

- `internal/app/gateway/server/**`
- `internal/app/user/server/**`
- `internal/app/user/service/**`
- `internal/app/user/internal/store/**`
- `web/src/api/api-manifest.ts`
- `web/src/config/routeMenu.ts`
- `web/dist/permission-contract.json`
- `web/scripts/generate-contract.cjs`
- `web/scripts/permission-guard-sdk.cjs`

## Permission Chain

Permission is a closed backend/frontend chain:

- `authsession` Bearer token validity and `AuthContext`.
- Public/login-only/protected API classification.
- Casbin object/action checks based on gRPC service/method identity.
- API catalog generated from registered gRPC metadata.
- `web/src/api/api-manifest.ts`.
- `web/src/config/routeMenu.ts`.
- `web/dist/permission-contract.json`.
- Role API boundary validation.
- Page/button permission checks.

Do not rely only on hidden frontend buttons or only on backend Casbin.

## API Classification

Every RPC must be classified:

- Public: no login, such as login, login crypto, captcha, gRPC health.
- Login-only: authenticated but no Casbin permission, such as menus, logout, heartbeat, personal center.
- Protected: default for management APIs.

OpenAPI security docs, middleware allowlists, and permission behavior must agree.

## API Catalog Ownership

Gateway-facing management APIs must update API catalog and frontend permission contracts. Internal-only gRPC services should not mutate the frontend permission catalog unless intentionally exposed through gateway.

`RegisterAPIs` uses gRPC service/method metadata and permission identity shaped like:

```text
USER.V1.USERSERVICE/ADDUSER
```

Do not use REST paths as permission identity.

## Frontend Contract

- `routeMenu.ts` is source for menu/button permission declarations.
- Use generated `APIs` constants rather than hardcoded API path strings.
- `permission-contract.json` is generated output. Do not hand-edit it as a lasting fix.
- Role save/update must reject API IDs outside selected menu/button boundaries.

## Workflow For Protected APIs

1. Add proto RPC and HTTP compatibility mapping if gateway-facing.
2. Run `make gen`.
3. Implement store/service/controller.
4. Register gRPC and HTTP compatibility where needed.
5. Regenerate or confirm `api-manifest.ts`.
6. Bind generated `APIs` constants in `routeMenu.ts`.
7. Build frontend contract.
8. Add service tests and gateway e2e when user-visible.

## e2e Requirement

Permission, role, menu, auth classification, API catalog, or frontend contract changes normally need gateway e2e. Cover both success and denial paths, especially ordinary-user forbidden access and role permission shrink behavior.

## Validation

- `make gen`
- `cd web && pnpm run build`
- `go test -race ./internal/app/user/service`
- Inspect `web/dist/permission-contract.json`.
- `make e2e E2E_TIMEOUT=20m` for permission chain changes.
