# GORM Models And Store Structure

Read this before adding or changing service store files, repository interfaces, provider wiring, model structs, GORM tags, table names, ID hooks, migration model lists, or schema-facing behavior.

## Evidence Paths

- `internal/app/gateway/internal/store/**`
- `internal/app/user/internal/store/**`
- `internal/platform/database/mysql/**`
- `internal/app/<service>/service/**`
- `internal/app/<service>/controller/**`
- `api/proto/**`
- `atlas/migrations/<service>/**`

## File Organization

Organize each service store by durable responsibility:

- shared DB/provider files: service DB aggregate, transaction helpers, provider set, migration model list.
- `interface_<domain>.go`: service-facing repository interface.
- `<domain>.go`: constants, model struct, `TableName`, ID hook, conversion methods, repository struct, constructor, and core methods.
- `scope_<domain>.go`: reusable filters for list/search/data permission/time ranges.
- tests: repository, transaction, conversion, and query behavior.

Keep a domain together until there is a real reason to split.

## Repository Interfaces

Rules:

- Every method takes `context.Context` first.
- Return store model types, option structs, IDs, counts, or errors.
- Do not return protobuf messages or frontend view models.
- Keep write/read methods explicit.
- Use option structs for list methods with several filters.
- Add comments for side effects such as association changes.

Do not put service decisions in repository interfaces. Authorization and business decisions belong in service.

## Repository Struct And Provider Wiring

- Repositories receive the service DB/platform dependencies through provider sets.
- Constructors should return interfaces when that is the local pattern.
- Do not call `egorm.Load(...).Build()` inside repositories, services, or controllers.
- Add store providers to the owning service provider graph.
- Regenerate Wire with `make gen`.

## Model Rules

- Implement `TableName()` for every persistent model.
- Use current project base model conventions from the owning store/platform database package.
- Implement ID hooks only where the model follows project-generated IDs.
- Keep business constants near the model.
- Use `time.Time` for DB time/date values and conversion methods for protobuf timestamp output.
- Use pointer fields only when null semantics are intentionally needed.
- Use explicit join table structs for many-to-many relations that need stable table names or custom keys.
- Put computed response fields on conversion methods, not stored DB fields.

## GORM Tag Rules

- Choose MySQL types explicitly.
- Define `not null` and defaults for stable zero behavior.
- Add indexes only for query patterns or uniqueness rules.
- Add service-level duplicate handling for user-facing unique indexes.
- Keep comments meaningful and aligned with proto comments.

Before adding a field, answer:

- Is it stored, computed, or request-only?
- Is zero value valid or should it be nullable?
- Does it need an index?
- Does proto need copier tags or conversion methods?
- Does frontend validation match proto and DB length?
- Does the migration need a default for existing rows?
- Does this require e2e because it changes a user-visible workflow?

## Migration Hooks

- Add new models to the owning service migration model list.
- Add join tables to the owning service join table setup.
- Generate with `make migrate.new SERVICE=<service> NAME=<change_name>`.
- Commit SQL and service `atlas.sum` together.

## Validation

- `go test -race ./internal/app/<service>/internal/store/...`
- `make gen`
- `make migrate.new SERVICE=<service> NAME=<change_name>`
- `rg -n "validate:|label:|copier:" api/gen/go`
- `make e2e E2E_TIMEOUT=20m` when the model affects gateway-facing behavior.
