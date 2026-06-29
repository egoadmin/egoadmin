# User & Department Management

User and department management is the core functionality of the EgoAdmin user service, covering user CRUD, password management, and organizational tree maintenance.

## Overview

The EgoAdmin user service manages platform users and organizational structure. Users are represented by `UserModel`, linked to departments and roles. Departments use a `parent_id + code` path-encoding scheme to implement a hierarchical tree structure, supporting efficient recursive subtree queries.

All user and department operations are constrained by DataScope (row-level data permissions), ensuring operators can only access data within their authorized scope.

## Core Usage

### User Model

The user model is stored in the `user` table with the following key fields:

```go
type UserModel struct {
    xorm.Model
    Username   string      // Username, unique index
    Password   string      // Password hash stored with bcrypt
    Name       string      // Full name
    Phone      *string     // Phone number, unique index
    Gender     int8        // Gender: 0-hidden, 1-male, 2-female
    UserStatus int32       // Status: 1-valid, 2-invalid
    DeptID     uint64      // Department ID
    Avatar     string      // Avatar reference ID
    Roles      []RoleModel // Many-to-many role association
    Remark     string      // Remark
}
```

The `user_role` join table maintains the many-to-many relationship:

```go
type UserRole struct {
    UserModelID uint64 `gorm:"primaryKey"`
    RoleModelID uint64 `gorm:"primaryKey"`
}
```

### Creating a User

The full create flow: distributed lock -> data scope validation -> uniqueness check in transaction -> generate default password -> bcrypt hash -> persist -> reload Casbin roles.

```go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    // Verify operator can manage the target department
    if user.DeptID != 0 {
        if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
            return err
        }
    }
    // Verify operator can assign the target roles
    if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
        return err
    }
    // ...
}
```

Key steps within the transaction:

```go
addlock := s.UserRedis.LockAdd()
if err = addlock.Lock(ctx, time.Second*5); err != nil {
    return
}
defer func() { _ = addlock.Unlock(ctx) }()

if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
    if er := s.addCheck(txCtx, user); er != nil { // Uniqueness check
        return er
    }
    defaultPassword, er := userdomain.DefaultPasswordFromPhone(stringValue(user.Phone))
    if er != nil {
        return er
    }
    hashPass, er := xbcrypt.HashAndSalt(defaultPassword)
    if er != nil {
        return er
    }
    user.Password = hashPass
    return s.User.Add(txCtx, user)
}); err != nil {
    return
}
err = s.reloadCasbinRolesForUser(ctx, user.ID)
```

::: warning
The password is auto-generated from the phone number following `DefaultPasswordFromPhone` rules. The frontend is not allowed to submit a custom password (`application.ErrSubmittedPassword`). Admins should guide users to change their password after first login.
:::

### Updating a User

Update reads the existing record, validates data scope, then applies changes:

```go
func (s *UserService) UpdateUser(ctx context.Context, id uint64, user *store.UserModel) (err error) {
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    savedUser, err := s.User.Get(ctx, id)
    if err != nil {
        return err
    }
    // Verify operator has access to the current user
    if err = scope.EnforceUser(ctx, savedUser); err != nil {
        return err
    }
    // Verify operator can manage the target department
    if user.DeptID != 0 {
        if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
            return err
        }
    }
    if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
        return err
    }
    // Delegate to use-case for actual persistence
    if err = s.UserUseCase.UpdateUser(ctx, application.UpdateUserCommand{...}); err != nil {
        return err
    }
    // Clear data scope cache so next query recalculates
    return deleteDataScopeCache(ctx, s.DataScopeCache(), id)
}
```

### Deleting a User

Deletion completes inside a transaction, then cleans up caches and sessions outside:

```go
func (s *UserService) DeleteUser(ctx context.Context, ids []uint64) (err error) {
    var users []*store.UserModel
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
        users, er = s.User.GetByIds(txCtx, ids)
        if er != nil {
            return er
        }
        for _, user := range users {
            if er = scope.EnforceUser(ctx, user); er != nil { // Per-user check
                return er
            }
        }
        return s.User.Delete(txCtx, ids)
    }); err != nil {
        return
    }
    // Post-transaction cleanup
    for _, user := range users {
        s.Options.Casbin.DeleteRolesForUser(user.Username)       // Clear Casbin
        deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), user.ID) // Clear auth cache
        deleteDataScopeCache(ctx, s.DataScopeCache(), user.ID)         // Clear data scope cache
        s.Auth.RevokeUser(ctx, user.ID, authsession.StatusRevoked)     // Revoke sessions
    }
    return firstCacheErr
}
```

### Password Change and Reset

Password change requires old password verification; reset does not:

```go
// Change password — requires old password
func (s *UserService) UpdateUserPassword(ctx context.Context, id uint64, oldpass, newpass string) (err error) {
    user, err := s.GetSelfUser(ctx, id)
    if err != nil {
        return
    }
    if oldpass != "" && !s.comparePassword(user.Password, oldpass) {
        err = platformi18n.ErrorFailed(ctx, "OldPasswordIncorrect", nil)
        return
    }
    hashPass, err := xbcrypt.HashAndSalt(newpass)
    if err != nil {
        return
    }
    if err = s.User.UpdatePass(ctx, id, hashPass); err != nil {
        return
    }
    deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), id)
    err = s.Auth.RevokeUser(ctx, id, authsession.StatusRevoked)
    return
}

// Reset password — no old password needed, constrained by data scope
func (s *UserService) ResetUserPassword(ctx context.Context, id uint64) (err error) {
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    user, err := s.User.Get(ctx, id)
    if err != nil {
        return err
    }
    if err = scope.EnforceUser(ctx, user); err != nil {
        return err
    }
    return s.UserUseCase.ResetUserPassword(ctx, id)
}
```

### Department Model and Tree Structure

Departments use `parent_id` and a `code` path-encoding to implement a hierarchy. The `code` field stores path IDs from root to the current node, joined by `-`:

```go
type DeptModel struct {
    xorm.Model
    Code     string      // Path encoding, e.g. "1-5-12"
    ParentID uint64      // Parent department ID, 0 for top-level
    DeptName string      // Department name
    Leader   string      // Department leader
    Phone    string      // Contact phone
    Email    string      // Email
    Priority int32       // Sort order
    Status   int32       // Status: 1-active, 2-disabled
    Level    int32       // Nesting level, max 5
    Childs   []DeptModel // Child departments
}
```

::: tip
The `code` path-encoding enables efficient subtree queries with `WHERE code LIKE '1-5-%'`, avoiding recursive CTEs entirely.
:::

Querying subtree IDs:

```go
func (m *Dept) GetSubtreeIDs(ctx context.Context, id uint64) (ids []uint64, err error) {
    dept, err := m.GetSelf(ctx, id)
    if err != nil {
        return nil, err
    }
    db := mysql.DBWithContext(ctx, m.cc)
    err = db.Model(&DeptModel{}).
        Where("code LIKE ?", dept.Code+"%").
        Pluck("id", &ids).Error
    return ids, err
}
```

Querying ancestor chain:

```go
func (m *Dept) GetAncestorIDs(ctx context.Context, id uint64) (ids []uint64, err error) {
    dept, err := m.GetSelf(ctx, id)
    if err != nil {
        return nil, err
    }
    parts := strings.Split(dept.Code, "-")
    ids = make([]uint64, 0, len(parts))
    for _, part := range parts {
        ancestorID, err := strconv.ParseUint(part, 10, 64)
        if err != nil {
            return nil, err
        }
        ids = append(ids, ancestorID)
    }
    return ids, nil
}
```

### User Listing

User listing supports pagination, sorting, department scoping, role filtering, and name/username search:

```go
func (s *UserService) getUserList(ctx context.Context, opt store.UserModelGetListOption,
    scopes ...func(*gorm.DB) *gorm.DB) (users []*store.UserModel, total int64, err error) {

    dataScope, err := s.resolveDataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    scopes = append(scopes, dataScope.UserScope()) // Inject data permission

    if len(opt.Deptids) != 0 {
        ids, er := s.Dept.GetSubtreeIDs(ctx, opt.Deptids[0])
        if er != nil {
            return nil, 0, er
        }
        opt.Deptids = ids // Expand to full subtree
    }
    return s.User.GetList(ctx, opt, scopes...)
}
```

Callers choose the search variant:

```go
// Search by name or username
users, total, err := s.GetUserListNameOrUserName(ctx, opt, name)

// Search by username only
users, total, err := s.GetUserListUserName(ctx, opt, name)
```

## Configuration Examples

User service configuration lives in `configs/user/config.toml`:

```toml
[app.user]
adminPassword = "123456"           # Admin initial password
jwtExpire = 604800                 # JWT expiry in seconds
heartbeatOfflineSeconds = 660      # Heartbeat timeout threshold
multiLoginEnabled = true           # Allow multi-device login
maxLoginClient = 2                 # Max concurrent login devices
```

::: warning
Production environments must override sensitive configuration via `EGOADMIN_`-prefixed environment variables. Never commit real passwords or keys to the repository.
:::

## Real-World Examples

### Controller Call for Creating a User

```go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    out = &userv1.AddUserResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
        }
    }()

    gender, ok := store.UserModelGenderToInt8(in.GetUser().GetGender())
    if !ok {
        err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
        return
    }
    user := &store.UserModel{
        Username:   in.GetUser().GetUsername(),
        Name:       in.GetUser().GetName(),
        Phone:      lo.ToPtr(in.GetUser().GetPhone()),
        Gender:     gender,
        UserStatus: in.GetUser().GetUserStatus(),
        DeptID:     in.GetUser().GetDeptId(),
        Roles:      userRolesFromIDs(in.GetRoleIds()),
    }
    err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
    if err != nil {
        return
    }
    out.Id = user.ID
    return
}
```

### Online User Management

```go
// Heartbeat — only refreshes online status, does not revoke sessions
func (s *UserGRPC) HeartBeatUser(ctx context.Context, in *userv1.HeartBeatUserRequest) (*userv1.HeartBeatUserResponse, error) {
    auth, ok := authsession.FromContext(ctx)
    if !ok {
        return nil, platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)
    }
    return &userv1.HeartBeatUserResponse{}, s.userUseCase.MarkUserOnline(ctx, auth.UserID)
}
```

## How It Works

1. **Data scope enforcement**: Every read/write calls `resolveDataScope(ctx)` to get the current user's data permission scope.
2. **Distributed locking**: Create and update operations use Redis distributed locks to prevent concurrency conflicts.
3. **Transaction protection**: Multi-table writes (user + role associations) complete inside a single transaction.
4. **Cache coherence**: Modifying a user, department, or role clears related data scope and auth snapshot caches.
5. **Session management**: Deleting a user or changing a password revokes all active sessions for that user.

```text
Request enters gRPC Controller
  -> Parse auth context (authsession)
  -> Call resolveDataScope for data permissions
  -> DataScope.EnforceUser / EnforceDeptID check
  -> Acquire distributed lock
  -> Begin transaction
    -> Uniqueness check
    -> Business operation
  -> Commit transaction
  -> Clear caches + reload Casbin
```

## Common Issues

::: danger Data scope not enforced
Make sure every query calls `resolveDataScope(ctx)` and injects `scope.UserScope()` as a GORM scope. Missing this step allows regular users to see data beyond their permission.
:::

::: danger Department delete fails
Check `CountByDeptIds` before deleting a department. If users are still assigned to it, deletion is rejected. The frontend should call `CheckDeleteDept` for pre-validation.
:::

::: tip Cache invalidation after password reset
After resetting a password, clear both `AuthSnapshotCache` and `DataScopeCache` and revoke user sessions. Otherwise, the user can still access the system with their old access token.
:::

::: danger Casbin roles not synced
After creating a user, call `reloadCasbinRolesForUser`. Without this, Casbin has no role information for the user, and protected APIs will return 403.
:::

## Reference Links

- [Permission System](/guide/en-US/permission-system) -- API classification, Casbin checks, routeMenu
- [Roles & Permissions](/guide/en-US/user-service/role-permission) -- Role CRUD and permission assignment
- [Data Permissions](/guide/en-US/user-service/data-permission) -- DataScope row-level access control
- [Audit Log](/guide/en-US/user-service/audit-log) -- Operation auditing and compliance
