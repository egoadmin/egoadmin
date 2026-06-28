# GORM Queries And Scopes

Read this before writing repository CRUD methods, list/count methods, pagination, sorting, scopes, association replacement, duplicate checks, existence checks, or data permission filters.

## Evidence Paths

- `internal/app/<service>/internal/store/**`
- `internal/platform/database/mysql/**`
- `internal/app/<service>/service/**`

## Query Ownership

Reusable persistence behavior belongs in the owning service store package. Services should call named repository methods and scopes instead of building raw GORM chains.

Rules:

- Every repository method accepts `context.Context`.
- Repository methods use transaction-aware DB helpers.
- List and count use the same filters.
- Add stable ordering, usually requested order plus ID tie-breaker.
- Return empty slices, not nil slices, when practical.
- Distinguish not-found behavior deliberately: return a typed/domain error when the API contract depends on it.

## Scopes

Use scopes for reusable filters:

- status filters.
- keyword/name filters.
- ID sets.
- time ranges.
- data permission filters.
- association preloads when they are reusable and safe.

Keep service-specific authorization decisions in service. A scope may express a persistence filter, but service decides when it applies.

## Associations

- Preload associations needed by response conversion methods.
- Use explicit association replacement methods for many-to-many changes.
- Keep association updates inside transactions when they are part of one business write.
- Avoid hidden N+1 queries in list responses.

## Duplicate And Existence Checks

- Store can expose count/existence helpers.
- Service owns deciding whether duplicates are business errors.
- User-facing uniqueness needs both DB constraints where appropriate and service-level friendly errors.

## Pagination And Sorting

- Validate user-facing sort fields through an allowlist.
- Add deterministic tie-break ordering.
- Keep count/list filters aligned.
- Guard total overflow when converting to narrower proto fields.

## Validation

- `go test -race ./internal/app/<service>/internal/store/...`
- service tests for duplicate/delete/status rules.
- e2e when query behavior affects list/detail pages or permission-visible data.
