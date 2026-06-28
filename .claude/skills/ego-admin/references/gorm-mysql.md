# GORM And MySQL

Read this before writing service-owned GORM models, repository interfaces, repository methods, scopes, transactions, pagination, sorting, migration model lists, or DB-to-RPC conversion logic.

This is the router. Load focused references before editing code:

- [gorm-models.md](gorm-models.md) for model/repository organization and provider wiring.
- [gorm-queries.md](gorm-queries.md) for CRUD/list/count/scopes/association queries.
- [gorm-transactions-conversion.md](gorm-transactions-conversion.md) for transactions, errors, and conversion.
- [data-migration.md](data-migration.md) for Atlas and schema rollout.
- [proto-contract.md](proto-contract.md) when fields reach API contracts.
- [backend-api-flow.md](backend-api-flow.md) for controller/service/store flow.

## Source Boundaries

- Target DDD persistence path: `internal/app/<service>/adapter/persistence/mysql`: service-owned GORM models, repository implementations, scopes, migration model lists, and conversion helpers.
- Migration-era path: `internal/app/<service>/internal/store`: existing service-owned models, repositories, scopes, migration model lists, and conversion helpers.
- `internal/app/<service>/domain/<aggregate>`: aggregate types and Repository interfaces for new/refactored code.
- `internal/app/<service>/application`: transactions and use-case orchestration.
- `internal/platform/database/mysql`: shared GORM setup, transaction context, base helpers, migration plumbing.
- `internal/app/<service>/controller`: proto request/response mapping and operation logs.
- `atlas/migrations/<service>`: service database schema evolution.

Do not spread raw GORM chains through controllers or application/services. If a query is reused, validates persistence state, or expresses persistence behavior, put it behind the owning persistence adapter or migration-era store.

## New Persisted Domain Checklist

1. Define proto contract first.
2. Add domain aggregate/value objects and Repository interface under `domain/<aggregate>` when using the DDD target layout.
3. Add model, repository implementation, scopes, and conversion methods in `adapter/persistence/mysql` or the migration-era `internal/store`.
4. Add model/join table to that service's migration model list.
5. Add repository constructor to provider wiring.
6. Add repository interface to application options where needed.
7. Implement application business rules and transactions.
8. Implement controller conversion/logging.
9. Generate migration with `SERVICE=<service>`.
10. Add tests and evaluate e2e for user-visible flows.

## Non-Negotiable Rules

- Every repository method takes `context.Context` first.
- Repositories return domain aggregates, persistence models, option structs, IDs, counts, or errors according to the package's migration state; never protobuf response messages.
- Repository methods use transaction-aware DB helpers.
- Multi-step business writes belong in application/service-level transactions.
- Service-owned store packages must not import other service packages.
- Cross-service data access goes through `internal/client/*`.
- Model changes require migration review.
- Do not hand-edit generated code, Wire output, or migration metadata as a lasting fix.

## Validation

- `go test -race ./internal/app/<service>/internal/store/...`
- `go test -race ./internal/app/<service>/service/...`
- `make gen`
- `make migrate.new SERVICE=<service> NAME=<change_name>` for schema changes.
- `make e2e E2E_TIMEOUT=20m` for complete gateway-facing workflows.
