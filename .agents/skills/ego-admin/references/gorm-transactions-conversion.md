# GORM Transactions And Conversion

Read this before changing transaction boundaries, service/store cooperation, Redis-lock-protected writes, error behavior, zero-result behavior, DB-to-RPC conversion methods, copier tags, or controller response assembly.

## Evidence Paths

- `internal/app/<service>/service/**`
- `internal/app/<service>/internal/store/**`
- `internal/platform/database/mysql/**`
- `api/proto/**`

## Transaction Boundaries

Service owns business transactions:

- Acquire locks before transactions when concurrent writes can violate invariants.
- Run multi-step DB changes in service-level transaction helpers.
- Pass transaction context to every store method inside the transaction.
- Keep non-DB side effects after commit unless they must be part of rollback behavior.
- Do not open unrelated transactions inside store methods when the service already owns the boundary.

Store methods must be transaction-aware and use DB handles derived from context.

## Error Behavior

- Return generated proto/domain errors for reusable business failures.
- Return generic outward errors for sensitive auth failures.
- Preserve not-found semantics required by API contracts.
- Do not expose raw SQL errors to users when a friendly business error is expected.

## Conversion Rules

- Proto fields use copier tags for durable mapping differences.
- Store model methods perform time conversion, ID conversion, masking, computed values, and association projection.
- Controllers may assemble endpoint-only presentation fields.
- Services should not return protobuf response messages.
- Repositories should not import proto packages for response construction.

## Association Conversion

If a response field depends on associations:

- Proto field has a copier tag pointing at a conversion method, or controller assigns it explicitly.
- Repository preloads the needed association.
- Tests or e2e exercise a non-empty association path.

## Validation

- `go test -race ./internal/app/<service>/service/...`
- `go test -race ./internal/app/<service>/internal/store/...`
- `make gen` for copier/proto/provider changes.
- `make e2e E2E_TIMEOUT=20m` for complete user-visible write/read workflows.
