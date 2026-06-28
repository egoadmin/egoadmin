# Backend API Flow

Read this before implementing controller methods, service methods, service store queries/models, request conversion, response conversion, copier mappings, or operation logs.

## Evidence Paths

- `api/proto/**`, `api/proto-internal/**`
- `api/gen/go/**`
- `internal/app/<service>/controller/**`
- `internal/app/<service>/service/**`
- `internal/app/<service>/internal/store/**`
- `internal/client/**`

## Standard Flow

For new APIs:

1. Define service/rpc/request/response/entity/errors in proto.
2. Run `make gen`.
3. Implement the generated controller interface.
4. Convert request values into service/store inputs.
5. Call service.
6. Convert outputs into protobuf response with copier tags, store conversion methods, or endpoint-specific assembly.
7. Update permission/frontend when gateway-facing.
8. Add tests and evaluate e2e.

For persisted features:

1. Proto defines public contract and validation.
2. Store defines models, repositories, scopes, conversion helpers, and migration model list.
3. Service defines business rules, locks, transactions, permission/data rules, and side effects.
4. Controller maps request/response and records logs.
5. Gateway/frontend bind generated method names to routeMenu and permission contract when user-facing.

## Controller Rules

- Initialize `out` before work.
- Use `in.GetXxx()` accessors, especially for nested messages.
- Let validator middleware enforce generated tags.
- Build service/store option structs at the request edge.
- Keep durable validation in service.
- Use deferred success-only logging where current controller methods do so.
- Check list totals before assigning to narrower proto fields.
- Use copier and conversion methods for durable response contract mapping.

Do not put direct GORM queries in controller.

## Service Rules

- Own transactions, locks, duplicate checks, delete guards, status transitions, permission boundaries, cache/session invalidation, and cross-service orchestration.
- Use owned store interfaces.
- Use `internal/client/*` wrappers for remote calls.
- Keep side effects that should happen only after commit outside DB transactions when appropriate.
- Return store models or service DTOs, not protobuf response messages.

## Store Rules

Service store packages own:

- GORM model definitions.
- Repository interfaces and implementations.
- Query options and scopes.
- Transaction-aware DB helpers.
- Migration model registration.
- Conversion methods used by copier.

Do not reuse another service's store. Cross-service access belongs behind gRPC clients.

## Copier Rules

- Use proto `copier` tags when Go field names differ (`ID`, `ParentID`, `DeptID`, `URL`).
- Use store model methods for time conversion, masking, computed fields, associations, and ID conversion.
- Confirm repositories preload associations before relying on association-derived conversion methods.
- Use controller assembly only for endpoint-specific presentation.

## e2e Rule

If the API is user-visible, changes auth/permission/menu behavior, crosses services, or affects gateway HTTP compatibility, add or update gateway e2e unless existing e2e already proves the behavior. Mention the e2e path in handoff.

## Validation

- `make gen`
- `go test -race ./internal/app/<service>/...`
- `make service.check SERVICE=<service>` for service wiring changes.
- `cd web && pnpm run build` for frontend/permission changes.
- `make e2e E2E_TIMEOUT=20m` for complete gateway-facing workflows.
