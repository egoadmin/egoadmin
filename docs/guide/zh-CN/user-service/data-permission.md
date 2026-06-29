# 数据权限

数据权限（DataScope）是 EgoAdmin 行级数据访问控制机制，基于用户角色的数据权限等级，自动为 GORM 查询注入 WHERE 条件，确保用户只能访问其权限范围内的数据。

## 概述

DataScope 解决"用户能看到哪些数据"的问题。与 API 权限（Casbin 校验"能不能访问这个接口"）不同，DataScope 控制"返回哪些行"。

每个角色携带一个数据权限等级（1-全部、2-本部门及子部门、3-本部门、4-仅本人）。系统根据用户所有角色中最大的权限等级，计算最终的数据权限范围。

内置管理员用户（admin/root）自动绕过所有数据权限过滤。

## 核心用法

### DataScope 模型

```go
type DataScopeLevel int32

const (
    DataScopeAll        DataScopeLevel = 1 // 全部数据
    DataScopeDeptAndSub DataScopeLevel = 2 // 本部门及子部门
    DataScopeDeptSelf   DataScopeLevel = 3 // 本部门
    DataScopeSelf       DataScopeLevel = 4 // 仅本人
)

type DataScope struct {
    UserID  uint64         `json:"userID"`
    DeptID  uint64         `json:"deptID"`  // 当前用户所属部门
    Level   DataScopeLevel `json:"level"`   // 最大的数据权限等级
    DeptIDs []uint64       `json:"deptIDs"` // 可见的部门 ID 列表
    IsAdmin bool           `json:"isAdmin"` // 是否内置管理员
}
```

### 权限等级解析

系统从用户所有角色中选取最宽泛的权限等级：

```go
func widestDataScope(roles []store.RoleModel) DataScopeLevel {
    if len(roles) == 0 {
        return DataScopeSelf // 无角色默认仅本人
    }
    best := DataScopeSelf
    for _, role := range roles {
        level := DataScopeLevel(role.DataPerm)
        switch level {
        case DataScopeAll:
            return DataScopeAll // 全部是最大值，直接返回
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

等级排序：`全部(1) > 本部门及子部门(2) > 本部门(3) > 仅本人(4)`

### 权限解析流程

`resolveDataScope` 是所有数据权限计算的统一入口：

```go
func resolveDataScope(ctx context.Context, user store.UserInterface,
    role store.RoleInterface, dept store.DeptInterface, cache dataScopeCache) (DataScope, error) {

    auth, ok := authsession.FromContext(ctx)
    if !ok || auth.UserID == 0 {
        return DataScope{}, platformi18n.ErrorNotLogin(ctx, "AuthMissingToken", nil)
    }
    // 内置管理员直接返回全部权限
    if auth.IsBuiltinAdmin {
        return DataScope{
            UserID:  auth.UserID,
            Level:   DataScopeAll,
            IsAdmin: true,
        }, nil
    }
    // 尝试从缓存读取
    key := dataScopeCacheKey(auth.UserID)
    if cache != nil {
        var cached DataScope
        if err := cache.Get(ctx, key, &cached); err == nil && cached.UserID == auth.UserID {
            return cached, nil
        }
    }
    // 从数据库加载并缓存
    scope, err := loadDataScope(ctx, auth, user, dept)
    if err != nil {
        return DataScope{}, err
    }
    if cache != nil {
        cache.Set(ctx, key, scope, dataScopeTTL) // TTL: 2 分钟
    }
    return scope, nil
}
```

`loadDataScope` 根据等级计算可见部门列表：

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
        return scope, nil // 无需计算部门列表
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

### SQL 注入

DataScope 为不同实体提供独立的 GORM scope 方法：

**UserScope** — 用户表查询：

```go
func (s DataScope) UserScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin || s.Level == DataScopeAll {
            return db // 不加过滤
        }
        switch s.Level {
        case DataScopeDeptAndSub, DataScopeDeptSelf:
            if len(s.DeptIDs) == 0 {
                return dataScopeDeny(db) // 无可见部门，拒绝全部
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

**LogScope** — 日志表查询：

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

**RoleScope** — 角色表查询（更复杂，需要排除内置角色并按归属过滤）：

```go
func (s DataScope) RoleScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin {
            return db // 管理员看全部
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

### 权限校验方法

DataScope 提供多个 Enforce 方法用于显式权限校验：

```go
// 校验是否可以访问该用户
func (s DataScope) EnforceUser(ctx context.Context, user *store.UserModel) error

// 校验是否可以访问该部门
func (s DataScope) EnforceDeptID(ctx context.Context, id uint64) error

// 校验是否可以修改该部门
func (s DataScope) EnforceDeptMutableID(ctx context.Context, id uint64) error

// 校验是否可以访问该角色
func (s DataScope) EnforceRole(ctx context.Context, role *store.RoleModel) error

// 校验是否可以修改该角色
func (s DataScope) EnforceRoleMutable(ctx context.Context, role *store.RoleModel) error

// 校验是否可以分配该数据权限等级
func (s DataScope) EnforceAssignableDataPerm(ctx context.Context, level int32) error
```

校验失败时返回 `ErrorAccessDenied`：

```go
func (s DataScope) EnforceUser(ctx context.Context, user *store.UserModel) error {
    if s.AllowsUser(user) {
        return nil
    }
    return platformi18n.ErrorAccessDenied(ctx, "NoAccessUserData", nil)
}
```

### 缓存机制

DataScope 缓存在 Redis 中，TTL 为 2 分钟：

```go
const dataScopeTTL = 2 * time.Minute

func dataScopeCacheKey(userID uint64) string {
    return fmt.Sprintf("%s:auth:data_scope:%d", defaults.RedisKeyPrefix, userID)
}
```

缓存失效场景：

| 操作 | 失效方式 |
|------|----------|
| 用户角色变更 | `deleteDataScopeCache(ctx, cache, userID)` |
| 用户部门变更 | `deleteDataScopeCache(ctx, cache, userID)` |
| 部门结构变更 | 遍历部门关联用户批量清除 |
| 角色删除 | 获取角色下所有用户，批量清除 |
| 角色数据权限等级变更 | 获取角色下所有用户，批量清除 |

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
部门结构变更后必须清除关联用户的数据权限缓存，否则用户在 2 分钟 TTL 内仍使用旧的部门可见范围。
:::

### Admin/Root 绕过

内置管理员在 `resolveDataScope` 入口直接返回全部权限，不查询数据库：

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

`UserModelUsernameRoot = "root"` 和 `UserModelUsernameAdmin = "admin"` 为内置管理员账号。

## 配置示例

DataScope 缓存依赖 Redis（通过 JetCache 组件）：

```toml
[component.jetcache]
[component.jetcache.default]
remote = "redis"
prefix = "egoadmin"
```

::: warning
DataScope 的 TTL 是代码常量 `2 * time.Minute`，不是配置项。如果需要调整，需修改 `internal/app/user/service/data_scope.go`。
:::

## 实际示例

### 在用户列表中注入数据权限

```go
func (s *UserService) getUserList(ctx context.Context, opt store.UserModelGetListOption,
    scopes ...func(*gorm.DB) *gorm.DB) (users []*store.UserModel, total int64, err error) {

    dataScope, err := s.resolveDataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    // 注入数据权限 scope — 最终 SQL 会自动添加 WHERE 条件
    scopes = append(scopes, dataScope.UserScope())
    return s.User.GetList(ctx, opt, scopes...)
}
```

### 在部门查询中使用可见性过滤

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
    // 校验父部门可见性
    if er = scope.EnforceDeptID(ctx, parentID); er != nil {
        return nil, er
    }
    // 过滤子部门
    filtered := make([]*store.DeptModel, 0, len(depts))
    for _, dept := range depts {
        if dept != nil && scope.AllowsDeptID(dept.ID) {
            filtered = append(filtered, dept)
        }
    }
    return filtered, nil
}
```

### 角色可分配性校验

```go
func (s DataScope) AllowsRole(role *store.RoleModel) bool {
    if s.IsAdmin {
        return true
    }
    if role == nil || role.BuiltIn == store.RoleModelBuiltIn {
        return false // 不能操作内置角色
    }
    if !dataPermAssignable(s.Level, DataScopeLevel(role.DataPerm)) {
        return false // 不能分配高于自身等级的权限
    }
    if role.OwnerUserID == s.UserID {
        return true // 自己创建的角色
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

## 工作原理

```text
请求进入 Service 层
  -> 调用 resolveDataScope(ctx)
    -> 检查是否内置管理员 → 直接返回 DataScopeAll
    -> 查询 Redis 缓存 → 命中则返回
    -> 缓存未命中：
      -> 查询用户信息和角色列表
      -> widestDataScope() 选取最大权限等级
      -> 根据等级计算 DeptIDs（子树查询 / 当前部门 / 仅本人）
      -> 写入缓存，TTL 2 分钟
  -> 使用 DataScope 的方法：
    -> EnforceUser / EnforceDeptID — 显式校验单个实体
    -> UserScope / LogScope / RoleScope — 注入 GORM WHERE 子句
    -> AllowsUser / AllowsDeptID — 代码中的布尔判断
```

## 常见问题

::: danger 数据权限未注入
确保每次数据库查询前调用 `resolveDataScope(ctx)` 并将对应的 scope 注入 GORM 查询。遗漏此步骤将导致普通用户看到超出权限范围的数据。
:::

::: danger 部门变更后权限不生效
部门结构变更后必须调用 `deleteDataScopeCacheByDeptIDs` 清除关联用户的缓存。TTL 为 2 分钟，在此期间用户可能仍使用旧的权限范围。
:::

::: warning 无部门用户
如果用户没有分配部门（`DeptID == 0`），`resolveDataScope` 会返回 `ErrorAccessDenied`。所有用户必须关联到有效部门。
:::

::: tip 数据权限等级不可降级分配
普通管理员只能分配等于或低于自身等级的数据权限。`dataPermAssignable` 通过 `int32(target) >= int32(actor)` 实现此约束。数值越小等级越高。
:::

## 参考链接

- [权限系统](/guide/zh-CN/permission-system) — API 分类、Casbin 校验、routeMenu
- [用户与部门管理](/guide/zh-CN/user-service/user-dept) — 用户 CRUD 与部门树
- [角色与权限](/guide/zh-CN/user-service/role-permission) — 角色 CRUD 与权限分配
- [审计日志](/guide/zh-CN/user-service/audit-log) — 操作审计与合规
