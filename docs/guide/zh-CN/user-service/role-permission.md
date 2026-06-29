# 角色与权限

角色与权限管理是 EgoAdmin RBAC 体系的核心，通过角色定义数据权限范围和 API 访问权限，并与 Casbin 策略引擎集成实现动态权限校验。

## 概述

EgoAdmin 采用 RBAC（基于角色的访问控制）模型。角色（RoleModel）是权限的载体，每个角色绑定数据权限等级（DataScope）和 API 权限策略（Casbin policies）。用户通过多对多关系关联角色，登录后聚合所有角色的菜单和权限。

权限链路贯穿前后端：后端通过 Casbin 校验 API 访问权限，前端通过 routeMenu 控制菜单和按钮可见性。

## 核心用法

### 角色模型

角色存储在 `role` 表中：

```go
type RoleModel struct {
    xorm.Model
    Name        string                 // 角色名称
    Typ         int32                  // 类型：1-平台角色
    BuiltIn     int32                  // 1-内置角色, 2-普通角色
    DataPerm    int32                  // 数据权限：1-全部, 2-本部门及子部门, 3-本部门, 4-仅本人
    OwnerUserID uint64                 // 创建者用户 ID
    OwnerDeptID uint64                 // 归属部门 ID
    Uses        string                 // 功能权限，逗号分隔
    ViewMenus   string                 // 前端菜单 ID，逗号分隔
    Desc        string                 // 描述
    Policies    []RolePermissionPolicy // API 权限策略
}
```

数据权限等级定义：

| 值 | 常量 | 含义 |
|----|------|------|
| 1 | `RoleModelDataPermAll` | 全部数据 |
| 2 | `RoleModelDataPermUserDeptAndSubDept` | 本部门及子部门 |
| 3 | `RoleModelDataPermUserDeptSelf` | 本部门 |
| 4 | `RoleModelUserSelf` | 仅本人 |

### API 权限策略

每个角色绑定一组 API 权限策略，存储在 `role_permission_policy` 表中，映射 Casbin 的 subject/object/act：

```go
type RolePermissionPolicy struct {
    RoleModelID uint64 // 角色 ID（联合主键）
    Service     string // gRPC 服务名，Casbin object
    Method      string // gRPC 方法名，Casbin act
}
```

Casbin 规则示例：

```text
p, role:1001, USER.V1.USERSERVICE/ADDUSER, *
p, role:1001, USER.V1.ROLESERVICE/UPDATEROLE, *
```

::: tip
权限标识使用 gRPC service/method 身份（如 `USER.V1.USERSERVICE/ADDUSER`），不使用 HTTP path 或前端路由 path。
:::

### 新增角色

新增角色时校验数据权限等级可分配性，并记录创建者信息：

```go
func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) (err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return err
    }
    // 校验操作者有权分配该数据权限等级
    if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
        return err
    }
    // 非管理员记录创建者归属
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

### 修改角色

修改角色时校验可变性（Mutable）和数据权限等级：

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
    // 校验操作者有权修改该角色
    if err = scope.EnforceRoleMutable(ctx, savedRole); err != nil {
        return err
    }
    // 校验操作者有权分配新的数据权限等级
    if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
        return err
    }
    // 保留原始归属信息
    role.OwnerUserID = savedRole.OwnerUserID
    role.OwnerDeptID = savedRole.OwnerDeptID
    // 获取受影响的用户列表，用于后续清除缓存
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

### 删除角色

删除前校验权限，删除后清除受影响用户的数据权限缓存：

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
内置角色（`BuiltIn == 1`）不可删除、不可修改。普通管理员无法操作其他管理员创建的角色，除非拥有足够的数据权限范围。
:::

### 角色查询与列表

角色列表和查询受数据权限约束，通过 `scope.RoleScope()` 注入 GORM scope：

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

### 用户角色分配

用户与角色通过 `user_role` 中间表实现多对多关联。GORM 在创建用户时自动维护关联：

```go
// 新增用户时，Roles 字段携带角色列表
user := &store.UserModel{
    Username: in.GetUser().GetUsername(),
    Roles:    userRolesFromIDs(in.GetRoleIds()),
}
s.User.Add(ctx, user) // GORM 自动写入 user_role
```

更新用户角色时使用 Replace：

```go
func (m *User) Update(ctx context.Context, id uint64, user *UserModel) error {
    return mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
        // 替换角色关联
        if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).
            Association("Roles").Replace(user.Roles); er != nil {
            return er
        }
        return tx.Omit("Roles").Model(&UserModel{Model: xorm.Model{ID: id}}).Updates(user).Error
    })
}
```

### 角色使用计数

删除角色前检查是否有用户仍绑定该角色：

```go
func (m *User) CountByRole(ctx context.Context, roleId uint64) (count int64, err error) {
    db := mysql.DBWithContext(ctx, m.cc)
    err = db.Model(&UserRole{}).Where("role_model_id = ?", roleId).Count(&count).Error
    return
}
```

### Casbin 同步

角色变更后需要重新加载受影响用户的 Casbin 策略。新增用户后同样需要同步：

```go
// 新增用户后重新加载 Casbin
err = s.reloadCasbinRolesForUser(ctx, user.ID)
```

::: danger
如果角色更新了 `Policies` 字段但没有触发 Casbin reload，已登录用户的权限不会实时生效，直到会话刷新或下次登录。
:::

### 菜单系统

角色的 `ViewMenus` 字段存储逗号分隔的菜单 ID。登录时聚合所有角色的菜单：

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

前端 routeMenu 使用常量绑定菜单和按钮权限：

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

## 配置示例

```toml
[app.user]
adminPassword = "123456"

# permission-contract 校验开关（仅调试用，生产环境不可开启）
[app.service]
skipPermissionContractCheck = false
```

`permission-contract.json` 在前端构建时生成，确保角色只能被授予已注册的 API：

```bash
cd web
pnpm run build
# 输出: web/dist/permission-contract.json
```

## 实际示例

### Controller 层的角色审计日志

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

### 错误映射

领域错误在 Controller 层映射为 i18n 错误消息：

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

## 工作原理

```text
角色 CRUD 请求
  -> resolveDataScope 获取操作者数据权限
  -> EnforceRoleMutable / EnforceAssignableDataPerm 校验
  -> Use Case 层执行业务逻辑（唯一性检查、使用计数）
  -> 写入 role 表 + role_permission_policy 表
  -> 清除受影响用户的数据权限缓存
  -> Casbin 策略在下次访问时重新加载
```

Casbin 与 DataScope 的协作关系：

```text
用户登录
  -> authsession 签发 token
  -> 请求到达 gateway
  -> Casbin 检查该用户角色是否有权限访问目标 API
  -> Controller 内 DataScope 过滤数据范围
```

## 常见问题

::: danger 角色删除失败
检查 `CountByRole`，如果仍有用户绑定该角色则无法删除。前端应在删除前调用 `CheckDeleteRole` 接口预检查。
:::

::: danger 数据权限等级越权
普通管理员只能分配等于或低于自身数据权限等级的角色。例如只有"本部门"权限的管理员不能创建"全部数据"的角色。
:::

::: tip 内置角色保护
`BuiltIn == 1` 的角色（如系统管理员）不允许修改或删除，即使数据权限允许。`EnforceRoleMutable` 会自动拒绝。
:::

::: danger Casbin 策略未同步
角色权限策略变更后，受影响用户需要在下次请求时重新加载 Casbin。如果需要立即生效，管理员应要求用户重新登录。
:::

## 参考链接

- [权限系统](/guide/zh-CN/permission-system) — API 分类、Casbin 校验、routeMenu
- [用户与部门管理](/guide/zh-CN/user-service/user-dept) — 用户 CRUD 与部门树
- [数据权限](/guide/zh-CN/user-service/data-permission) — DataScope 行级权限详解
- [审计日志](/guide/zh-CN/user-service/audit-log) — 操作审计与合规
