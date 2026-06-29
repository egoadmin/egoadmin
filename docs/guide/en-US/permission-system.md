# Permission System

EgoAdmin uses a closed backend/frontend permission chain. Hiding frontend buttons is not considered authorization by itself.

## Chain

```text
authsession Bearer token
  -> API classification
  -> Casbin service/method permission
  -> DataScope query filtering
  -> API manifest
  -> routeMenu.ts
  -> permission-contract.json
```

## API Classification

| Type | Login Required | Casbin Required | Example |
|------|----------------|-----------------|---------|
| public | no | no | login, captcha, login crypto challenge |
| login-only | yes | no | menus, logout, heartbeat, personal center |
| protected | yes | yes | user, role and department management |

## Permission ID

Protected APIs use gRPC identity:

```text
USER.V1.USERSERVICE/ADDUSER
USER.V1.ROLESERVICE/UPDATEROLE
```

Do not use HTTP paths, frontend route paths or menu IDs as backend permission IDs.

### Casbin Model Definition

EgoAdmin uses an RBAC model with resource roles described by a PERM (Policy, Effect, Request, Matchers) model:

```conf
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && (r.act == p.act || p.act == "*")
```

Explanation:

- `r = sub, obj, act` -- request triple: subject (user role), object (API resource ID), action (method name).
- `p = sub, obj, act` -- policy definition matching the request.
- `g = _, _` -- user-role inheritance (RBAC).
- `g2 = _, _` -- role-resource-group mapping for DataScope scenarios.
- `keyMatch2` -- supports path parameter matching such as `/user.v1.UserService/:method`.
- `e = some(where (p.eft == allow))` -- a single allow policy is sufficient.

Gateway Casbin enforcer initialization:

```go
// internal/app/gateway/adapter/casbin/enforcer.go
func NewEnforcer(adapter persist.Adapter) (*casbin.Enforcer, error) {
  modelText := `...` // embedded model text above
  m, err := model.NewModelFromString(modelText)
  if err != nil {
    return nil, err
  }
  e, err := casbin.NewEnforcer(m, adapter)
  if err != nil {
    return nil, err
  }
  e.AddFunction("keyMatch2", util.KeyMatch2Func)
  return e, nil
}
```

::: tip
Casbin policies are stored in the `casbin_rule` table of the `egoadmin_user` database and read/written via `gorm-adapter`.
:::

## API Manifest

Gateway generates an API manifest at startup from registered gRPC methods. The frontend consumes it as typed constants.

### Proto-Driven Manifest

Each RPC in the proto file needs OpenAPI extension annotations:

```protobuf
// proto/user/v1/user_service.proto
service UserService {
  rpc GetUserList(GetUserListRequest) returns (GetUserListResponse) {
    option (google.api.http) = {
      get: "/user.v1.UserService/GetUserList"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      security: {
        security_requirement: {
          key: "BearerAuth";
          value: {}
        }
      }
    };
  }

  rpc AddUser(AddUserRequest) returns (AddUserResponse) {
    option (google.api.http) = {
      post: "/user.v1.UserService/AddUser"
      body: "*"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      security: {
        security_requirement: {
          key: "BearerAuth";
          value: {}
        }
      }
    };
  }
}
```

After running `make gen`, the gateway registers all gRPC methods. Building the frontend produces:

```ts
// web/src/api/api-manifest.ts (auto-generated build artifact, do not edit manually)
export const APIs = {
  user: {
    v1: {
      UserService: {
        GetUserList: '/user.v1.UserService/GetUserList',
        GetUser: '/user.v1.UserService/GetUser',
        AddUser: '/user.v1.UserService/AddUser',
        UpdateUser: '/user.v1.UserService/UpdateUser',
        DeleteUser: '/user.v1.UserService/DeleteUser',
      },
      RoleService: {
        GetRoleList: '/user.v1.RoleService/GetRoleList',
        AddRole: '/user.v1.RoleService/AddRole',
        UpdateRole: '/user.v1.RoleService/UpdateRole',
        DeleteRole: '/user.v1.RoleService/DeleteRole',
      },
      DeptService: {
        GetDeptTree: '/user.v1.DeptService/GetDeptTree',
        AddDept: '/user.v1.DeptService/AddDept',
        UpdateDept: '/user.v1.DeptService/UpdateDept',
        DeleteDept: '/user.v1.DeptService/DeleteDept',
      },
    },
  },
} as const
```

routeMenu references these constants:

```ts
apis: [
  APIs.user.v1.UserService.GetUserList,
  APIs.user.v1.UserService.AddUser,
]
```

::: warning
`api-manifest.ts` is a build artifact. Do not edit manually. Run `make gen` then `cd web && pnpm run build` after every proto change.
:::

## routeMenu

`web/src/config/routeMenu.ts` is the single source of truth for menus, pages, and button permissions.

```ts
// web/src/config/routeMenu.ts
import { MenuType } from '@/types/menu'
import { APIs } from '@/api/api-manifest'

{
  id: 30200,
  type: MenuType.Catalog,
  name: 'route.user',
  path: '/user',
  title: 'User Management',
  locale: 'menu.user',
  icon: 'ep:setting',
  children: [
    {
      id: 30201,
      parentId: 30200,
      type: MenuType.Page,
      name: 'route.user',
      path: '/user/user',
      title: 'User Management',
      locale: 'menu.user.user',
      icon: 'ep:user',
      apis: [APIs.user.v1.UserService.GetUserList],
      children: [
        {
          id: 30202,
          type: MenuType.Button,
          title: 'Add User',
          apis: [APIs.user.v1.UserService.AddUser],
        },
        {
          id: 30203,
          type: MenuType.Button,
          title: 'Edit User',
          apis: [APIs.user.v1.UserService.UpdateUser],
        },
        {
          id: 30204,
          type: MenuType.Button,
          title: 'Delete User',
          apis: [APIs.user.v1.UserService.DeleteUser],
        },
      ],
    },
    {
      id: 30301,
      parentId: 30200,
      type: MenuType.Page,
      name: 'route.role',
      path: '/user/role',
      title: 'Role Management',
      locale: 'menu.user.role',
      icon: 'ep:avatar',
      apis: [APIs.user.v1.RoleService.GetRoleList],
      children: [
        {
          id: 30302,
          type: MenuType.Button,
          title: 'Add Role',
          apis: [APIs.user.v1.RoleService.AddRole],
        },
        {
          id: 30303,
          type: MenuType.Button,
          title: 'Edit Role',
          apis: [APIs.user.v1.RoleService.UpdateRole],
        },
      ],
    },
  ],
}
```

### routeMenu Data Structure

| Field | Type | Description |
|-------|------|-------------|
| `id` | number | Globally unique menu/button ID |
| `parentId` | number | Parent ID; omit or use `0` for top-level |
| `type` | MenuType | `Catalog` (directory), `Page` (page), `Button` (button) |
| `name` | string | Route name for `router.push({ name })` |
| `path` | string | Route path |
| `title` | string | Display title |
| `locale` | string | i18n key |
| `icon` | string | Element Plus icon name |
| `apis` | string[] | Bound API permission identifiers |
| `children` | RouteMenu[] | Child menus/buttons |

Use permissions in Vue:

```ts
const userStore = useUserStore()
const canAdd = computed(() => userStore.VA(['30202']))
```

```vue
<el-button v-if="canAdd" type="primary" @click="openAddDialog">
  Add User
</el-button>
```

::: tip
Button hiding is a UX convenience. Real access control must be enforced by backend authsession + Casbin.
:::

## Permission Contract

```bash
cd web
pnpm run build
```

This generates `web/dist/permission-contract.json`.

### Contract Contents

```json
{
  "version": "1.0.0",
  "generatedAt": "2026-06-29T12:00:00Z",
  "menuMap": {
    "30201": {
      "title": "User Management",
      "apis": ["/user.v1.UserService/GetUserList"]
    },
    "30202": {
      "title": "Add User",
      "apis": ["/user.v1.UserService/AddUser"]
    },
    "30203": {
      "title": "Edit User",
      "apis": ["/user.v1.UserService/UpdateUser"]
    }
  }
}
```

### Contract Validation Flow

1. Frontend build walks `routeMenu.ts` and writes each menu/button ID with its bound APIs into `permission-contract.json`.
2. User service reads the contract when adding or editing a role.
3. The request carries the `menuIDs` the role should be granted.
4. Backend collects all API identifiers from those menuIDs via the contract, producing `allowedAPIs`.
5. The request also carries the `apis` to grant to the role.
6. Backend verifies `apis` is a subset of `allowedAPIs`; rejects if not.
7. On success, writes role-menu and role-API associations to the database.

```go
// internal/app/user/application/usecase/role_usecase.go
func (uc *RoleUseCase) AddRole(ctx context.Context, cmd AddRoleCommand) error {
  // 1. Collect allowedAPIs from contract based on menuIDs
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)

  // 2. Verify cmd.APIs is a subset of allowedAPIs
  if !isSubset(cmd.APIs, allowedAPIs) {
    return errors.New("role APIs exceed menu-allowed scope")
  }

  // 3. Persist to database
  return uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    role, err := uc.roles.Create(txCtx, cmd.Role)
    if err != nil {
      return err
    }
    if err := uc.permissions.ReplaceRoleMenus(txCtx, role.ID, cmd.MenuIDs); err != nil {
      return err
    }
    return uc.permissions.ReplaceRoleAPIs(txCtx, role.ID, cmd.APIs)
  })
}
```

You can temporarily skip validation during local development:

```toml
[app.service]
skipPermissionContractCheck = true
```

Do not skip in production.

## DataScope

DataScope restricts ordinary users' visible rows. Admin/root users bypass data permission checks.

| Scope | Meaning | Query Behavior |
|-------|---------|----------------|
| self | own records only | `owner_user_id = current_user` |
| department | current department | `dept_id = current_dept` |
| department tree | current department and children | `dept_id IN (...)` |
| all | no filter | admin/root or authorized role |
| custom | selected departments | `dept_id IN (selected_depts)` |

### DataScope Implementation

Struct definition:

```go
// internal/app/user/domain/permission/datascope.go
type DataScope struct {
  Scope     DataScopeType
  UserID    uint64
  DeptID    uint64
  DeptIDs   []uint64   // custom department set
  SkipCheck bool       // admin/root bypass
}
```

Reusable query filter:

```go
// internal/app/user/adapter/persistence/mysql/datascope.go
func applyDataScope(db *gorm.DB, scope permission.DataScope, tableAlias ...string) *gorm.DB {
  if scope.SkipCheck {
    return db // admin/root, no filter
  }

  alias := ""
  if len(tableAlias) > 0 {
    alias = tableAlias[0] + "."
  }

  switch scope.Scope {
  case permission.DataScopeSelf:
    return db.Where(alias+"owner_user_id = ?", scope.UserID)
  case permission.DataScopeDept:
    return db.Where(alias+"dept_id = ?", scope.DeptID)
  case permission.DataScopeDeptTree:
    deptIDs := getDeptTree(scope.DeptID) // recursive child department IDs
    return db.Where(alias+"dept_id IN ?", deptIDs)
  case permission.DataScopeCustom:
    return db.Where(alias+"dept_id IN ?", scope.DeptIDs)
  case permission.DataScopeAll:
    return db // no filter
  default:
    return db.Where("1 = 0") // unknown scope, deny access
  }
}
```

Repository usage:

```go
func (r *UserRepository) List(ctx context.Context, q user.Query, scope permission.DataScope) ([]*user.User, int64, error) {
  db := r.db.WithContext(ctx).Model(&UserModel{})
  db = applyUserFilters(db, q)
  db = applyDataScope(db, scope)

  var total int64
  if err := db.Count(&total).Error; err != nil {
    return nil, 0, err
  }

  var rows []UserModel
  if err := db.Offset(int((q.Page - 1) * q.Limit)).Limit(int(q.Limit)).Find(&rows).Error; err != nil {
    return nil, 0, err
  }

  return toUsers(rows), total, nil
}
```

### Admin/Root Bypass

```go
// internal/app/user/application/usecase/user_query.go
func (uc *UserUseCase) GetUserList(ctx context.Context, q user.Query) (*user.ListResult, error) {
  auth := permission.FromContext(ctx)

  scope := permission.DataScope{
    Scope:     auth.DataScope,
    UserID:    auth.UserID,
    DeptID:    auth.DeptID,
    DeptIDs:   auth.DeptIDs,
    SkipCheck: auth.IsAdmin || auth.IsRoot,
  }

  return uc.users.List(ctx, q, scope)
}
```

::: warning
`SkipCheck` should only be determined at the auth layer. Never hard-code role names inside repositories.
:::

## Role CRUD with Permission Assignment

### Role Data Model

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
type RoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name      string    `gorm:"column:name;type:varchar(64);not null;uniqueIndex"`
  Code      string    `gorm:"column:code;type:varchar(64);not null;uniqueIndex"`
  DataScope int32     `gorm:"column:data_scope;type:tinyint;not null;default:1"`
  Sort      int32     `gorm:"column:sort;not null;default:0"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1"`
  Remark    string    `gorm:"column:remark;type:varchar(255);default:''"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
```

### Association Tables

```go
// Role-menu association (permission grants)
type RoleMenuModel struct {
  ID     uint64 `gorm:"primaryKey;autoIncrement"`
  RoleID uint64 `gorm:"column:role_id;not null;index"`
  MenuID uint64 `gorm:"column:menu_id;not null;index"`
}

func (RoleMenuModel) TableName() string { return "role_menu" }

// User-role association
type UserRoleModel struct {
  ID     uint64 `gorm:"primaryKey;autoIncrement"`
  UserID uint64 `gorm:"column:user_id;not null;index"`
  RoleID uint64 `gorm:"column:role_id;not null;index"`
}

func (UserRoleModel) TableName() string { return "user_role" }
```

### Add Role with Permissions

```go
func (uc *RoleUseCase) AddRole(ctx context.Context, cmd AddRoleCommand) (*role.Role, error) {
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)
  if !isSubset(cmd.APIs, allowedAPIs) {
    return nil, errors.New("role APIs exceed menu-allowed scope")
  }

  var result *role.Role
  err := uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    r := &role.Role{
      Name:      cmd.Name,
      Code:      cmd.Code,
      DataScope: cmd.DataScope,
      Sort:      cmd.Sort,
      Status:    cmd.Status,
    }
    created, err := uc.roles.Create(txCtx, r)
    if err != nil {
      return err
    }
    result = created

    if err := uc.permissions.ReplaceRoleMenus(txCtx, created.ID, cmd.MenuIDs); err != nil {
      return err
    }
    for _, api := range cmd.APIs {
      if err := uc.enforcer.AddPolicy(created.Code, api, "*"); err != nil {
        return err
      }
    }
    return nil
  })
  return result, err
}
```

### Update Role Permissions

```go
func (uc *RoleUseCase) UpdateRole(ctx context.Context, cmd UpdateRoleCommand) error {
  allowedAPIs := uc.contract.CollectAPIs(cmd.MenuIDs)
  if !isSubset(cmd.APIs, allowedAPIs) {
    return errors.New("role APIs exceed menu-allowed scope")
  }

  return uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    if err := uc.roles.Update(txCtx, cmd.Role); err != nil {
      return err
    }

    // Remove old Casbin policies first
    oldAPIs := uc.permissions.GetRoleAPIs(txCtx, cmd.Role.ID)
    for _, api := range oldAPIs {
      uc.enforcer.RemovePolicy(cmd.Role.Code, api, "*")
    }

    if err := uc.permissions.ReplaceRoleMenus(txCtx, cmd.Role.ID, cmd.MenuIDs); err != nil {
      return err
    }

    // Write new Casbin policies
    for _, api := range cmd.APIs {
      if _, err := uc.enforcer.AddPolicy(cmd.Role.Code, api, "*"); err != nil {
        return err
      }
    }
    return uc.enforcer.SavePolicy()
  })
}
```

## Common Permission Issues

| Problem | Possible Cause | Solution |
|---------|---------------|----------|
| User accesses unauthorized page | Frontend route not guarded; wrong menu ID | Check `routeMenu` `apis` binding |
| Normal user calls protected API successfully | Casbin policy missing or enforcer not loaded | Check `casbin_rule` table; restart gateway |
| admin user blocked | admin flag not injected into AuthContext | Check `authsession` admin detection |
| Role API assignment rejected | API not in permission-contract scope | Add menu/button to `routeMenu`, rebuild contract |
| DataScope not applied | Repository missing `applyDataScope` call | Verify query method passes scope |
| Casbin policies inconsistent | Old policies not removed before update | RemovePolicy then AddPolicy on role edit |
| Multi-login token invalid | `maxLoginClient` too small | Adjust config or use token refresh |

### Debugging Tips

1. **Inspect Casbin policies**:

```bash
mysql -e "SELECT * FROM egoadmin_user.casbin_rule;"
```

2. **Temporarily disable contract check**:

```toml
[app.service]
skipPermissionContractCheck = true
```

3. **Fetch current user menus**:

```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/user.v1.UserService/GetUserMenus
```

4. **Check route guard**: verify `web/src/router/guard.ts` permission guard logic is not bypassed.

## Checklist for Adding a New Protected API

1. Proto RPC has complete OpenAPI auth annotations.
2. RPC is not in openPack / justLoginPack, defaults to protected.
3. Run `make gen`.
4. Server registers the generated gRPC service.
5. Gateway can generate api-manifest.
6. `routeMenu.ts` uses `APIs` constants for page/button binding.
7. `cd web && pnpm run build` generates `permission-contract.json`.
8. Normal user denied on unauthorized access.
9. admin or authorized role succeeds.
10. e2e covers both success and rejection paths.

## Validation

```bash
make gen
cd web && pnpm run build
go test -race ./internal/app/user/...
make e2e E2E_TIMEOUT=20m
```
