# Data Permission

Read this before changing role data permissions, user/role/dept/log list visibility, ownership fields, authorization scopes, or queries that should be filtered by the current user's organization or identity.

## Evidence Paths

- `internal/app/user/service/data_scope.go`
- `internal/app/user/service/{user,role,dept,log}_service.go`
- `internal/app/user/internal/store/scope_*.go`
- `internal/app/user/internal/store/{user,role,dept,log}.go`
- `internal/app/user/adapter/persistence/mysql/*role*`
- `test/e2e/gateway/*`

## Permission Model

Role `data_perm` levels are ordered from widest to narrowest:

- `1`: all data.
- `2`: current organization and descendants.
- `3`: current organization only.
- `4`: self only.

Built-in admin/root users are represented by `authsession.AuthContext.IsBuiltinAdmin` and must bypass data permission filters. Non-admin users must never bypass data permission only because they have Casbin API permission.

## Service Pattern

Resolve scope in service/application code, not in controllers:

```go
scope, err := s.DataScope(ctx)
if err != nil {
    return err
}
```

Use `DataScope` helpers for durable authorization decisions:

- List users through `scope.UserScope()`.
- List logs through `scope.LogScope()`.
- List roles through `scope.RoleScope()`.
- Filter department trees through `scope.FilterDeptTree(...)`.
- Check detail or mutation with `EnforceUser`, `EnforceDeptID`, `EnforceDeptMutableID`, `EnforceRole`, and `EnforceRoleMutable`.
- Before creating/updating a role, call `EnforceAssignableDataPerm`; a user can only assign a data permission level within their own scope.

Return localized access-denied errors with `platformi18n.ErrorAccessDenied(ctx, "...", nil)`; do not return raw English/Chinese strings.

## Query Rules

- Prefer GORM scopes and `gorm.io/gorm/clause` expressions over hand-written SQL.
- Keep list and count filters identical.
- Use deny-by-default scopes for invalid/empty scopes.
- Add stable indexes for columns used by data permission filters such as `dept_id`, `dept_id_u64`, `user_id_u64`, `owner_user_id`, `owner_dept_id`, and `data_perm`.
- Avoid loading all rows and filtering in memory except for tree assembly where the full tree is intentionally needed and then filtered.

## Role Ownership

Non-admin-created roles must record `OwnerUserID` and `OwnerDeptID`. Built-in roles must not be visible or mutable to ordinary users. Role visibility combines:

- role is not built-in.
- role `data_perm` is not wider than current user's data permission.
- role owner is the current user or an allowed owner department.

## Gateway And Error Propagation

Gateway-facing role/user/dept/log APIs should preserve user-service business errors. `internal/client/userclient` must normalize remote EGO gRPC errors back to `*eerrors.EgoError` so `ERROR_ACCESS_DENIED`, `ERROR_NOT_LOGIN`, and localized messages do not become `ERROR_FAILED` or `内部错误`.

## Testing

Data permission changes need both focused service tests and gateway e2e when visible through HTTP compatibility.

Cover:

- admin/root bypass.
- each scope level for list/detail/mutation.
- denial for out-of-scope user, role, dept, and log access.
- role creation/update rejecting `data_perm` wider than actor scope.
- gateway response reason stays `ecode.v1.ERROR_ACCESS_DENIED`, not internal failure.

## Validation

- `go test -race ./internal/app/user/service`
- `go test -race ./internal/client/userclient` when cross-service error propagation changes.
- `make e2e E2E_TIMEOUT=20m` for gateway-visible permission behavior.
