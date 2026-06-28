# Auth And Session

Read this before changing login, password transport, logincrypto, authsession, Bearer middleware, auth context, logout, refresh, forced offline, captcha, token revocation, auth allowlists, or session-related jobs.

## Evidence Paths

- `internal/app/gateway/server/grpc_server.go`
- `internal/app/user/server/grpc_server.go`
- `internal/app/user/controller/**`
- `internal/app/user/service/**`
- `internal/app/user/internal/store/**`
- `internal/component/logincrypto/**`
- `internal/component/authsession/**`
- `internal/platform/cache/**`

## Password Transport

Password entry flows use `logincrypto` RSA-OAEP-SHA256 challenges.

Rules:

- Send only encrypted payload fields such as `passwordCipher`, `keyId`, and `challengeId`.
- Do not send plaintext passwords.
- Do not reintroduce frontend MD5, stable password-equivalent hashes, OPAQUE, or service-local password transport.
- Challenge consumption must be one-time and action/UA/nonce/timestamp aware.
- External login/decrypt/password mismatch errors should remain generic.

Backend password storage remains bcrypt of raw password; logincrypto protects transport only.

## Auth Session

Use `internal/component/authsession` for:

- Bearer access-token validation.
- Refresh-token rotation.
- Logout/revoke.
- Forced offline.
- Auth context injection.
- Context validation and cache invalidation after identity/session-relevant changes.

Do not replace it with raw JWT parsing or service-local token stores.

## Gateway And User Middleware

- Public endpoints include login, login crypto, captcha, and gRPC health.
- Login-only endpoints include menus, logout, heartbeat, and personal center flows.
- Protected management endpoints require Bearer auth and permission.
- Gateway and user classifications must remain consistent for externally reachable flows.

If an endpoint is protected, do not permanently place it in public allowlists to bypass permission failures.

## Component Boundaries

- `logincrypto` and `authsession` live under `internal/component`.
- Component packages must not import app service packages or service-owned store packages directly.
- Project adapters may live in the service layer and inject narrow interfaces into components.
- Redis/cache/idgen dependencies should enter through platform/component provider paths.

## e2e Requirement

Auth/session changes almost always need e2e because middleware, gateway HTTP compatibility, Redis/session state, and user service behavior interact. Add or update gateway e2e for:

- login success/failure.
- challenge replay/mismatch/expiry.
- refresh token rotation.
- logout/revoke.
- forced offline.
- password change invalidation.
- menu/login-only/protected classification changes.

If no e2e is added, document the existing e2e path that already covers the behavior.

## Validation

- `go test -race ./internal/component/logincrypto/...`
- `go test -race ./internal/component/authsession/...`
- `go test -race ./internal/app/user/service ./internal/app/user/controller`
- `go test -race ./internal/app/gateway/server ./internal/app/user/server`
- `make e2e E2E_TIMEOUT=20m` for user-visible auth/session behavior.
