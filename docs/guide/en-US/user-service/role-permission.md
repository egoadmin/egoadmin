# Roles & Permissions

Role and permission management is the core of EgoAdmin's RBAC system. Roles define data permission scope and API access rights, integrated with the Casbin policy engine for dynamic permission enforcement.

## Overview

EgoAdmin uses a Role-Based Access Control (RBAC) model. A role (RoleModel) is the carrier of permissions, binding a data permission level (DataScope) and API permission policies (Casbin policies). Users are associated with roles through a many-to-many relationship. Upon login, menus and permissions are aggregated from all assigned roles.

The permission pipeline spans both backend and frontend: the backend uses Casbin to verify API access, while the frontend uses routeMenu to control menu and button visibility.

## Core Usage

### Role Model

Roles are stored in the `role` table:

```go
type RoleModel struct {
    xorm.Model
    Name        string                 // Role name
    Typ         int32                  // Type: 1-platform role
    BuiltIn     int32                  // 1-built-in, 2-normal
    DataPerm    int32                  // Data scope: 1-all, 2-dept+children, 3-dept, 4-self
    OwnerUserID uint64                 // Creator user ID
    OwnerDeptID uint64                 // Owning department ID
    Uses        string                 // Feature permissions, comma-separated
    ViewMenus   string                 // Frontend menu IDs, comma-separated
    Desc        string                 // Description
    Policies    []RolePermissionPolicy // API permission policies
}
```

Data permission level definitions:

| Value | Constant | Meaning |
|-------|----------|---------|
| 1 | `RoleModelDataPermAll` | All data |
| 2 | `RoleModelDataPermUserDeptAndSubDept` | Own department and children |
| 3 | `RoleModelDataPermUserDeptSelf` | Own department only |
| 4 | `RoleModelUserSelf` | Self only |

### API Permission Policies

Each role binds a set of API permission policies stored in the `role_permission_policy` table, mapping to Casbin's subject/object/act:

```go
type RolePermissionPolicy struct {
    RoleModelID uint64 // Role ID (composite primary key)
    Service     string // gRPC service name, Casbin object
    Method      string // gRPC method name, Casbin act
}
```

Casbin rule example:

```text
p, role:1001, USER.V1.USERSERVICE/ADDUSER, *
p, role:1001, USER.V1.ROLESERVICE/UPDATEROLE, *
```

::: tip
Permission identifiers use gRPC service/method identity (e.g. `USER.V1.USERSERVICE/ADDUSER`), not HTTP path or frontend route path.
:::

### Creating a Role

When creating a role, the system validates data permission level assignability and records creator information:

```go
func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) (err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return err
    }
    // Verify operator can assign this data permission level
    if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
        return err
    }
    // Non-admin users get ownership recorded
    if !scope.IsAdmin {
        role.OwnerUserID = scope.UserID
        role.OwnerDeptID = scope.DeptID
    }
    result, err := s.RoleUseCase.CreateRole(ctx, roleCommandFromStore(role))
    if err != nil {
        return mapRoleDomainError(ctx, err)
    }
    role.ID = result.ID
    return
}
```

### Updating a Role

Updating a role checks mutability (Mutable) and data permission level:

```go
func (s *RoleService) UpdateRole(ctx context.Context, roleID uint64, role *store.RoleModel) (err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return err
    }
    savedRole, err := s.Role.Get(ctx, roleID)
    if err != nil {
        return err
    }
    // Verify operator can modify this role
    if err = scope.EnforceRoleMutable(ctx, savedRole); err != nil {
        return err
    }
    // Verify operator can assign the new data permission level
    if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
        return err
    }
    // Preserve original ownership
    role.OwnerUserID = savedRole.OwnerUserID
    role.OwnerDeptID = savedRole.OwnerDeptID
    // Get affected users for cache invalidation
    affectedUserIDs, err := s.userIDsByRole(ctx, roleID)
    if err != nil {
        return err
    }
    if err = mapRoleDomainError(ctx, s.RoleUseCase.UpdateRole(ctx, roleID, roleCommandFromStore(role))); err != nil {
        return err
    }
    return deleteDataScopeCache(ctx, s.DataScopeCache(), affectedUserIDs...)
}
```

### Deleting a Role

Before deletion, the system checks permissions and clears affected users' data scope caches:

```go
func (s *RoleService) DeleteRole(ctx context.Context, id uint64) (err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return err
    }
    role, err := s.Role.Get(ctx, id)
    if err != nil {
        return err
    }
    if err = scope.EnforceRoleMutable(ctx, role); err != nil {
        return err
    }
    affectedUserIDs, err := s.userIDsByRole(ctx, id)
    if err != nil {
        return err
    }
    if err = mapRoleDomainError(ctx, s.RoleUseCase.DeleteRole(ctx, id)); err != nil {
        return err
    }
    return deleteDataScopeCache(ctx, s.DataScopeCache(), affectedUserIDs...)
}
```

::: warning
Built-in roles (`BuiltIn == 1`) cannot be deleted or modified. Regular admins cannot operate on roles created by other admins unless they have sufficient data scope.
:::

### Role Listing and Queries

Role listing and queries are constrained by data scope through `scope.RoleScope()`:

```go
func (s *RoleService) GetRoleList(ctx context.Context, name string, pgopt xorm.PaginateOption) (roles []*store.RoleModel, total int64, err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    roles, total, err = s.Role.GetList(ctx, name, pgopt, scope.RoleScope())
    return
}

func (s *RoleService) GetRoleAll(ctx context.Context) (roles []*store.RoleModel, err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return nil, err
    }
    roles, err = s.Role.GetAll(ctx, scope.RoleScope())
    return
}
```

### User-Role Assignment

Users and roles have a many-to-many relationship through the `user_role` join table. GORM manages associations automatically during user creation:

```go
// When creating a user, the Roles field carries the role list
user := &store.UserModel{
    Username: in.GetUser().GetUsername(),
    Roles:    userRolesFromIDs(in.GetRoleIds()),
}
s.User.Add(ctx, user) // GORM auto-inserts into user_role
```

When updating user roles, GORM uses Replace:

```go
func (m *User) Update(ctx context.Context, id uint64, user *UserModel) error {
    return mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
        // Replace role associations
        if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).
            Association("Roles").Replace(user.Roles); er != nil {
            return er
        }
        return tx.Omit("Roles").Model(&UserModel{Model: xorm.Model{ID: id}}).Updates(user).Error
    })
}
```

### Role Usage Count

Before deleting a role, check if any users still reference it:

```go
func (m *User) CountByRole(ctx context.Context, roleId uint64) (count int64, err error) {
    db := mysql.DBWithContext(ctx, m.cc)
    err = db.Model(&UserRole{}).Where("role_model_id = ?", roleId).Count(&count).Error
    return
}
```

### Casbin Synchronization

After role changes, affected users' Casbin policies need to be reloaded. The same applies after creating a new user:

```go
// Reload Casbin after user creation
err = s.reloadCasbinRolesForUser(ctx, user.ID)
```

::: danger
If a role's `Policies` field is updated without triggering a Casbin reload, logged-in users will not see the changes in real time until session refresh or next login.
:::

### Menu System

The role's `ViewMenus` field stores comma-separated menu IDs. During login, menus are aggregated from all roles:

```go
func (s *UserService) GetMenus(ctx context.Context, id uint64) (menus string, err error) {
    user, err := s.User.Get(ctx, id)
    if err != nil {
        return
    }
    menuArr := make([]string, 0)
    for _, role := range user.Roles {
        menuArr = append(menuArr, strings.Split(role.ViewMenus, ",")...)
    }
    menuArr = lo.Uniq(menuArr)
    menus = strings.Join(menuArr, ",")
    return
}
```

The frontend routeMenu binds menus and button permissions using constants:

```ts
{
  id: 30201,
  parentId: 30200,
  type: MenuType.Page,
  path: '/user/user',
  title: '用户管理',
  apis: [APIs.user.v1.UserService.GetUserList],
  children: [
    {
      id: 30202,
      type: MenuType.Button,
      title: '新增用户',
      apis: [APIs.user.v1.UserService.AddUser],
    },
  ],
}
```

## Configuration Examples

```toml
[app.user]
adminPassword = "123456"

# permission-contract check toggle (debug only, must not be enabled in production)
[app.service]
skipPermissionContractCheck = false
```

`permission-contract.json` is generated during frontend build, ensuring roles can only be assigned registered APIs:

```bash
cd web
pnpm run build
# Output: web/dist/permission-contract.json
```

## Real-World Examples

### Controller Audit Logging for Roles

```go
func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (out *userv1.AddRoleResponse, err error) {
    out = &userv1.AddRoleResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "系统管理-角色管理", "新增", "新增角色", in)
        }
    }()
    role := &store.RoleModel{
        Name:      in.GetRole().GetName(),
        DataPerm:  in.GetRole().GetDataPerm(),
        ViewMenus: in.GetRole().GetViewMenus(),
        Policies:  roleStorePermissionPolicies(in.GetRole().GetPolicies()),
    }
    err = s.role.AddRole(ctx, role)
    if err = mapRoleError(ctx, err); err != nil {
        return
    }
    out.Id = role.ID
    return
}
```

### Error Mapping

Domain errors are mapped to i18n error messages at the controller layer:

```go
func mapRoleError(ctx context.Context, err error) error {
    var inUse roledomain.InUseError
    switch {
    case err == nil:
        return nil
    case errors.Is(err, roledomain.ErrNameExists):
        return platformi18n.ErrorFailed(ctx, "RoleNameExists", nil)
    case errors.As(err, &inUse):
        return platformi18n.ErrorFailed(ctx, "RoleInUseCount", map[string]any{"Count": inUse.Count})
    case errors.Is(err, roledomain.ErrInUse):
        return platformi18n.ErrorFailed(ctx, "RoleInUse", nil)
    default:
        return err
    }
}
```

## How It Works

```text
Role CRUD request
  -> resolveDataScope to get operator's data permissions
  -> EnforceRoleMutable / EnforceAssignableDataPerm validation
  -> Use Case layer executes business logic (uniqueness, usage count)
  -> Persist to role table + role_permission_policy table
  -> Clear affected users' data scope cache
  -> Casbin policies reload on next request
```

Casbin and DataScope collaboration:

```text
User login
  -> authsession issues token
  -> Request reaches gateway
  -> Casbin checks if user's role permits access to the target API
  -> Controller applies DataScope for data filtering
```

## Common Issues

::: danger Role deletion fails
Check `CountByRole` -- if users still reference the role, deletion is rejected. The frontend should call `CheckDeleteRole` for pre-validation.
:::

::: danger Data permission level escalation
Regular admins can only assign roles with a data permission level equal to or lower than their own. For example, an admin with "own department" scope cannot create an "all data" role.
:::

::: tip Built-in role protection
Roles with `BuiltIn == 1` (such as system admin) cannot be modified or deleted, even if data scope allows it. `EnforceRoleMutable` automatically rejects these operations.
:::

::: danger Casbin policy not synced
After changing role permission policies, affected users need to reload Casbin on their next request. To apply immediately, ask users to re-login.
:::

## Reference Links

- [Permission System](/guide/en-US/permission-system) -- API classification, Casbin checks, routeMenu
- [User & Department Management](/guide/en-US/user-service/user-dept) -- User CRUD and department tree
- [Data Permissions](/guide/en-US/user-service/data-permission) -- DataScope row-level access control
- [Audit Log](/guide/en-US/user-service/audit-log) -- Operation auditing and compliance
