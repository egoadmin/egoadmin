# Data And Migration

Read this before changing GORM models, database fields, indexes, relations, join tables, Atlas migrations, migration config, runtime migration behavior, or data fields that flow to proto/frontend.

## Evidence Paths

- `internal/app/gateway/internal/store/**`
- `internal/app/user/internal/store/**`
- `internal/platform/database/mysql/**`
- `tools/atlasloader/**`
- `atlas/atlas.hcl`
- `atlas/schema/**`
- `atlas/migrations/gateway/**`
- `atlas/migrations/user/**`
- `configs/gateway/**`
- `configs/user/**`

## Model Source

Service-owned persistence packages are schema sources. During migration, existing store packages remain valid schema sources:

- `internal/app/gateway/internal/store`
- `internal/app/user/internal/store`
- Target DDD path: `internal/app/<service>/adapter/persistence/mysql`

Common database plumbing belongs in `internal/platform/database/mysql`.

`tools/atlasloader` loads service model sources for Atlas external schema. It may know service schema packages, but `internal/platform` must not import app schema packages.

`atlasloader` defaults to MySQL and supports `--dialect mysql|postgres|sqlite|sqlserver` for review or future multi-database migration work. Keep MySQL as the default runtime and migration target unless a task explicitly asks for another dialect.

## Migration Boundaries

Migrations follow database boundaries:

```text
atlas/migrations/gateway -> gateway database
atlas/migrations/user    -> user database
```

Rules:

- Generate default MySQL migrations with `make migrate.new SERVICE=<service> NAME=<change_name>`.
- Rehash default MySQL migrations with `make migrate.hash SERVICE=<service>`.
- Apply default MySQL migrations with `make migrate.apply SERVICE=<service> ATLAS_URL=<database-url>`.
- For non-MySQL experiments, pass `DIALECT=<postgres|sqlite|sqlserver>` and keep files under `atlas/migrations/<dialect>/<service>`.
- Do not add an extra `mysql` directory layer; MySQL remains `atlas/migrations/<service>`.
- Generate audit-only HCL snapshots with `make migrate.schema SERVICE=<service> [DIALECT=<dialect>]`. These go under `atlas/schema/<dialect>/<service>.hcl` and are for review/audit, not the source of truth.
- Atlas does not create databases.
- Do not apply one service's migration directory to another service database.
- Do not hand-maintain HCL as a second schema source while GORM model + Atlas versioned SQL is the project source of truth.
- Do not rely on AutoMigrate for production.

## Model Change Checklist

Before generating a migration:

- Model is in the owning service persistence package.
- Repository interface and methods compile.
- Migration model list includes the model and join tables.
- Service and controller behavior are wired if the field/API is used.
- Proto/copier/frontend effects are understood.
- Existing data defaults and rollout behavior are safe.

## DB To API Sync

Every model field change must check:

- Proto entity/request/response fields.
- Validation and copier tags.
- Store conversion methods.
- Service filters and business rules.
- Controller response assembly.
- Frontend types/forms/lists.
- Permission implications.
- Atlas SQL, optional audit HCL, and `atlas.sum`.

## Local And e2e Migration

Local/e2e Docker Compose must create databases first. Atlas only migrates existing databases. e2e should verify `atlas_schema_revisions` when migration behavior changes.

## Validation

- `make migrate.new SERVICE=<service> NAME=<change_name>`
- `make migrate.schema SERVICE=<service>` when schema-review HCL changed or is requested.
- `make migrate.hash SERVICE=<service>` after manual SQL edits.
- `make gen`
- `go test -race ./internal/app/<service>/internal/store/... ./internal/app/<service>/service/...`
- `make e2e E2E_TIMEOUT=20m` for migration, startup, or gateway-facing data behavior.
