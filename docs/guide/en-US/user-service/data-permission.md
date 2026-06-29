# Data Permissions

Data permissions (DataScope) is EgoAdmin's row-level data access control mechanism. Based on the user's role data permission levels, it automatically injects WHERE clauses into GORM queries, ensuring users can only access data within their authorized scope.

## Overview

DataScope answers the question "which data can this user see?" Unlike API permissions (Casbin checks "can this user access this endpoint"), DataScope controls "which rows are returned."

Each role carries a data permission level (1-all, 2-department+children, 3-department, 4-self). The system calculates the final data scope based on the widest permission level across all of the user's roles.

Built-in admin users (admin/root) automatically bypass all data permission filtering.

## Core Usage

### DataScope Model

```go
type DataScopeLevel int32

const (
    DataScopeAll        DataScopeLevel = 1 // All data
    DataScopeDeptAndSub DataScopeLevel = 2 // Own department and children
    DataScopeDeptSelf   DataScopeLevel = 3 // Own department
    DataScopeSelf       DataScopeLevel = 4 // Self only
)

type DataScope struct {
    UserID  uint64         `json:"userID"`
    DeptID  uint64         `json:"deptID"`  // Current user's department
    Level   DataScopeLevel `json:"level"`   // Widest data permission level
    DeptIDs []uint64       `json:"deptIDs"` // Visible department ID list
    IsAdmin bool           `json:"isAdmin"` // Whether built-in admin
}
```

### Permission Level Resolution

The system selects the widest permission level from all user roles:

```go
func widestDataScope(roles []store.RoleModel) DataScopeLevel {
    if len(roles) == 0 {
        return DataScopeSelf // No roles defaults to self-only
    }
    best := DataScopeSelf
    for _, role := range roles {
        level := DataScopeLevel(role.DataPerm)
        switch level {
        case DataScopeAll:
            return DataScopeAll // All is the maximum, return immediately
        case DataScopeDeptAndSub:
            if best == DataScopeDeptSelf || best == DataScopeSelf {
                best = DataScopeDeptAndSub
            }
        case DataScopeDeptSelf:
            if best == DataScopeSelf {
                best = DataScopeDeptSelf
            }
        case DataScopeSelf:
        default:
        }
    }
    return best
}
```

Level ranking: `All(1) > Dept+Children(2) > Dept(3) > Self(4)`

### Resolution Flow

`resolveDataScope` is the unified entry point for all data permission calculations:

```go
func resolveDataScope(ctx context.Context, user store.UserInterface,
    role store.RoleInterface, dept store.DeptInterface, cache dataScopeCache) (DataScope, error) {

    auth, ok := authsession.FromContext(ctx)
    if !ok || auth.UserID == 0 {
        return DataScope{}, platformi18n.ErrorNotLogin(ctx, "AuthMissingToken", nil)
    }
    // Built-in admin bypasses all filtering
    if auth.IsBuiltinAdmin {
        return DataScope{
            UserID:  auth.UserID,
            Level:   DataScopeAll,
            IsAdmin: true,
        }, nil
    }
    // Try cache
    key := dataScopeCacheKey(auth.UserID)
    if cache != nil {
        var cached DataScope
        if err := cache.Get(ctx, key, &cached); err == nil && cached.UserID == auth.UserID {
            return cached, nil
        }
    }
    // Load from database and cache
    scope, err := loadDataScope(ctx, auth, user, dept)
    if err != nil {
        return DataScope{}, err
    }
    if cache != nil {
        cache.Set(ctx, key, scope, dataScopeTTL) // TTL: 2 minutes
    }
    return scope, nil
}
```

`loadDataScope` computes the visible department list based on the permission level:

```go
func loadDataScope(ctx context.Context, auth *authsession.AuthContext,
    user store.UserInterface, dept store.DeptInterface) (DataScope, error) {

    savedUser, err := user.Get(ctx, auth.UserID)
    if err != nil {
        return DataScope{}, err
    }
    if savedUser.DeptID == 0 {
        return DataScope{}, platformi18n.ErrorAccessDenied(ctx, "CurrentUserNoDept", nil)
    }
    level := widestDataScope(savedUser.Roles)
    scope := DataScope{
        UserID: auth.UserID,
        DeptID: savedUser.DeptID,
        Level:  level,
    }
    switch level {
    case DataScopeAll:
        return scope, nil // No department list needed
    case DataScopeDeptAndSub:
        ids, er := dept.GetSubtreeIDs(ctx, savedUser.DeptID)
        if er != nil {
            return DataScope{}, er
        }
        scope.DeptIDs = ids
    case DataScopeDeptSelf:
        scope.DeptIDs = []uint64{savedUser.DeptID}
    case DataScopeSelf:
        scope.DeptIDs = []uint64{savedUser.DeptID}
    default:
        scope.Level = DataScopeSelf
        scope.DeptIDs = []uint64{savedUser.DeptID}
    }
    return scope, nil
}
```

### SQL Injection

DataScope provides separate GORM scope methods for different entities:

**UserScope** -- User table queries:

```go
func (s DataScope) UserScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin || s.Level == DataScopeAll {
            return db // No filter
        }
        switch s.Level {
        case DataScopeDeptAndSub, DataScopeDeptSelf:
            if len(s.DeptIDs) == 0 {
                return dataScopeDeny(db) // No visible departments, deny all
            }
            return db.Where(uint64ColumnIn("dept_id", s.DeptIDs))
        case DataScopeSelf:
            return db.Where(clause.Eq{
                Column: clause.Column{Table: clause.CurrentTable, Name: "id"},
                Value:  s.UserID,
            })
        default:
            return dataScopeDeny(db)
        }
    }
}
```

**LogScope** -- Log table queries:

```go
func (s DataScope) LogScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin || s.Level == DataScopeAll {
            return db
        }
        switch s.Level {
        case DataScopeDeptAndSub, DataScopeDeptSelf:
            if len(s.DeptIDs) == 0 {
                return dataScopeDeny(db)
            }
            return db.Where(uint64ColumnIn("dept_id_u64", s.DeptIDs))
        case DataScopeSelf:
            return db.Where(clause.Eq{
                Column: clause.Column{Table: clause.CurrentTable, Name: "user_id_u64"},
                Value:  s.UserID,
            })
        default:
            return dataScopeDeny(db)
        }
    }
}
```

**RoleScope** -- Role table queries (more complex, excludes built-in roles and filters by ownership):

```go
func (s DataScope) RoleScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin {
            return db // Admin sees all
        }
        owner := make([]clause.Expression, 0, 3)
        switch s.Level {
        case DataScopeAll:
            owner = append(owner,
                clause.Neq{Column: "owner_user_id", Value: uint64(0)},
                clause.Neq{Column: "owner_dept_id", Value: uint64(0)},
            )
        case DataScopeDeptAndSub, DataScopeDeptSelf:
            if s.UserID != 0 {
                owner = append(owner, clause.Eq{Column: "owner_user_id", Value: s.UserID})
            }
            if len(s.DeptIDs) != 0 {
                owner = append(owner, uint64ColumnIn("owner_dept_id", s.DeptIDs))
            }
        case DataScopeSelf:
            owner = append(owner, clause.Eq{Column: "owner_user_id", Value: s.UserID})
        default:
            return dataScopeDeny(db)
        }
        return db.Where(clause.AndConditions{Exprs: []clause.Expression{
            clause.Neq{Column: "built_in", Value: store.RoleModelBuiltIn},
            clause.Gte{Column: "data_perm", Value: int32(s.Level)},
            clause.OrConditions{Exprs: owner},
        }})
    }
}
```

### Enforcement Methods

DataScope provides multiple Enforce methods for explicit permission checks:

```go
// Verify access to a specific user
func (s DataScope) EnforceUser(ctx context.Context, user *store.UserModel) error

// Verify access to a specific department
func (s DataScope) EnforceDeptID(ctx context.Context, id uint64) error

// Verify permission to modify a department
func (s DataScope) EnforceDeptMutableID(ctx context.Context, id uint64) error

// Verify access to a specific role
func (s DataScope) EnforceRole(ctx context.Context, role *store.RoleModel) error

// Verify permission to modify a role
func (s DataScope) EnforceRoleMutable(ctx context.Context, role *store.RoleModel) error

// Verify permission to assign a data permission level
func (s DataScope) EnforceAssignableDataPerm(ctx context.Context, level int32) error
```

Failed checks return `ErrorAccessDenied`:

```go
func (s DataScope) EnforceUser(ctx context.Context, user *store.UserModel) error {
    if s.AllowsUser(user) {
        return nil
    }
    return platformi18n.ErrorAccessDenied(ctx, "NoAccessUserData", nil)
}
```

### Caching Mechanism

DataScope is cached in Redis with a 2-minute TTL:

```go
const dataScopeTTL = 2 * time.Minute

func dataScopeCacheKey(userID uint64) string {
    return fmt.Sprintf("%s:auth:data_scope:%d", defaults.RedisKeyPrefix, userID)
}
```

Cache invalidation scenarios:

| Operation | Invalidation Method |
|-----------|-------------------|
| User role change | `deleteDataScopeCache(ctx, cache, userID)` |
| User department change | `deleteDataScopeCache(ctx, cache, userID)` |
| Department structure change | Bulk clear for all associated users |
| Role deletion | Get all users of the role, bulk clear |
| Role data permission level change | Get all users of the role, bulk clear |

```go
func deleteDataScopeCache(ctx context.Context, cache dataScopeCache, userIDs ...uint64) error {
    if cache == nil {
        return nil
    }
    for _, userID := range userIDs {
        if userID == 0 {
            continue
        }
        if err := cache.Delete(ctx, dataScopeCacheKey(userID)); err != nil {
            return err
        }
    }
    return nil
}
```

::: danger
After department structure changes, you must clear data scope caches for all associated users. Otherwise, users will continue using the old department visibility scope within the 2-minute TTL window.
:::

### Admin/Root Bypass

Built-in admins get full permissions at the `resolveDataScope` entry point without querying the database:

```go
if auth.IsBuiltinAdmin {
    return DataScope{
        UserID:  auth.UserID,
        DeptID:  auth.DeptID,
        Level:   DataScopeAll,
        IsAdmin: true,
    }, nil
}
```

`UserModelUsernameRoot = "root"` and `UserModelUsernameAdmin = "admin"` are the built-in admin accounts.

## Configuration Examples

DataScope caching depends on Redis (via the JetCache component):

```toml
[component.jetcache]
[component.jetcache.default]
remote = "redis"
prefix = "egoadmin"
```

::: warning
DataScope TTL is a code constant `2 * time.Minute`, not a configurable value. To adjust it, modify `internal/app/user/service/data_scope.go`.
:::

## Real-World Examples

### Injecting Data Scope into User Listing

```go
func (s *UserService) getUserList(ctx context.Context, opt store.UserModelGetListOption,
    scopes ...func(*gorm.DB) *gorm.DB) (users []*store.UserModel, total int64, err error) {

    dataScope, err := s.resolveDataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    // Inject data scope -- final SQL automatically adds WHERE clauses
    scopes = append(scopes, dataScope.UserScope())
    return s.User.GetList(ctx, opt, scopes...)
}
```

### Visibility Filtering in Department Queries

```go
func (s *DeptService) GetDeptChilds(ctx context.Context, parentID uint64) (depts []*store.DeptModel, err error) {
    depts, err = s.Dept.GetChilds(ctx, parentID)
    if err != nil {
        return nil, err
    }
    scope, er := s.DataScope(ctx)
    if er != nil {
        return nil, er
    }
    // Verify parent department visibility
    if er = scope.EnforceDeptID(ctx, parentID); er != nil {
        return nil, er
    }
    // Filter child departments
    filtered := make([]*store.DeptModel, 0, len(depts))
    for _, dept := range depts {
        if dept != nil && scope.AllowsDeptID(dept.ID) {
            filtered = append(filtered, dept)
        }
    }
    return filtered, nil
}
```

### Role Assignability Check

```go
func (s DataScope) AllowsRole(role *store.RoleModel) bool {
    if s.IsAdmin {
        return true
    }
    if role == nil || role.BuiltIn == store.RoleModelBuiltIn {
        return false // Cannot operate on built-in roles
    }
    if !dataPermAssignable(s.Level, DataScopeLevel(role.DataPerm)) {
        return false // Cannot assign permissions higher than own level
    }
    if role.OwnerUserID == s.UserID {
        return true // Own role
    }
    switch s.Level {
    case DataScopeAll:
        return role.OwnerUserID != 0 || role.OwnerDeptID != 0
    case DataScopeDeptAndSub, DataScopeDeptSelf:
        return role.OwnerDeptID != 0 && s.AllowsDeptID(role.OwnerDeptID)
    case DataScopeSelf:
        return false
    default:
        return false
    }
}
```

## How It Works

```text
Request enters Service layer
  -> Call resolveDataScope(ctx)
    -> Check if built-in admin -> return DataScopeAll
    -> Check Redis cache -> return on hit
    -> Cache miss:
      -> Query user info and role list
      -> widestDataScope() selects the widest permission level
      -> Compute DeptIDs based on level (subtree query / current dept / self only)
      -> Write to cache with 2-minute TTL
  -> Use DataScope methods:
    -> EnforceUser / EnforceDeptID -- explicit single-entity checks
    -> UserScope / LogScope / RoleScope -- inject GORM WHERE clauses
    -> AllowsUser / AllowsDeptID -- boolean checks in application code
```

## Common Issues

::: danger Data scope not injected
Ensure every database query calls `resolveDataScope(ctx)` and injects the appropriate scope into the GORM query. Missing this step allows regular users to see data beyond their authorized scope.
:::

::: danger Permission changes not effective after department change
After department structure changes, call `deleteDataScopeCacheByDeptIDs` to clear caches for associated users. Within the 2-minute TTL window, users may still use the old permission scope.
:::

::: warning Users without a department
If a user has no department assigned (`DeptID == 0`), `resolveDataScope` returns `ErrorAccessDenied`. All users must be associated with a valid department.
:::

::: tip Data permission level cannot be escalated
Regular admins can only assign data permission levels equal to or lower than their own. `dataPermAssignable` enforces this with `int32(target) >= int32(actor)`. Lower numeric values represent higher permission levels.
:::

## Reference Links

- [Permission System](/guide/en-US/permission-system) -- API classification, Casbin checks, routeMenu
- [User & Department Management](/guide/en-US/user-service/user-dept) -- User CRUD and department tree
- [Roles & Permissions](/guide/en-US/user-service/role-permission) -- Role CRUD and permission assignment
- [Audit Log](/guide/en-US/user-service/audit-log) -- Operation auditing and compliance
