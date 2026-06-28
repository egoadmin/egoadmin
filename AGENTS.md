# Repository Guidelines

## Project Structure & Module Organization
- `cmd/<service>` holds service entrypoints: `gateway`, `user`, and `idgen`.
- `internal/app/<service>` contains service code; prefer `server`, `controller`, `application`, `domain`, `adapter`, and `schema`.
- `internal/platform` is shared infrastructure. `internal/component` is reusable middleware/component code. `internal/client` wraps cross-service gRPC calls.
- `atlas/migrations/<service>` stores per-service database migrations.
- `test/compose` and `test/e2e/gateway` support local middleware and end-to-end tests.
- `deploy/` contains deployment compose files and runtime config examples.

## Build, Test, and Development Commands
- `make gen` generates proto, Go, and Wire code.
- `make build` or `make build SERVICE=user` builds binaries; `make image.build` builds service images.
- `make run` starts all services locally; `make run SERVICE=gateway` starts one service.
- `make dev-up` / `make dev-down` manage local dev middleware.
- `make deploy-up` / `make deploy-down` manage the deploy compose stack.
- `make test` runs Go tests; `make e2e` runs gateway end-to-end tests.
- `make migrate.new SERVICE=user NAME=add_field` creates a new Atlas migration.

## Coding Style & Naming Conventions
- Use `gofmt`-formatted Go code and keep package names short, lower-case, and service-scoped.
- Do not edit generated files such as `wire_gen.go`, proto outputs, or frontend generated artifacts.
- Keep runtime config minimal. Prefer `EGOADMIN_*` env overrides for deployment-specific values.
- Use service and file naming that matches existing patterns: `internal/app/<service>/...`, `configs/<service>/config.toml`.

## Testing Guidelines
- Prefer table-driven tests and `TestXxx` naming.
- Add or update e2e coverage for user-facing or cross-service changes, especially gateway flows.
- Run `go test ./...` for broad validation and `make e2e E2E_TIMEOUT=20m` for runtime paths.

## Commit & Pull Request Guidelines
- Commit messages follow conventional prefixes seen in history: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`.
- PRs should describe the change, list touched services, and note migrations, compose changes, or config changes.
- Include screenshots or request/response examples for frontend or API behavior changes.

## Security & Configuration
- Never commit secrets or production DSNs. Use `deploy/.env` and service config overrides instead.
- In containers, use service DNS names, not `127.0.0.1`, for inter-service connections.
