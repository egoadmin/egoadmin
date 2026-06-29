# 用户与部门管理

用户与部门管理是 EgoAdmin 用户服务的核心功能，涵盖用户增删改查、密码管理以及组织树的维护。

## 概述

EgoAdmin 的用户服务管理平台用户和组织架构。用户通过 `UserModel` 表示，关联到部门和角色。部门采用 `parent_id + code` 路径编码方案实现层级树结构，支持递归子树查询。

所有用户和部门操作都受数据权限（DataScope）约束，确保操作者只能访问其权限范围内的数据。

## 核心用法

### 用户模型

用户模型存储在 `user` 表中，核心字段如下：

```go
type UserModel struct {
    xorm.Model
    Username   string      // 用户名，唯一索引
    Password   string      // bcrypt 存储的密码哈希
    Name       string      // 姓名
    Phone      *string     // 手机号，唯一索引
    Gender     int8        // 性别：0-保密, 1-男, 2-女
    UserStatus int32       // 状态：1-有效, 2-无效
    DeptID     uint64      // 所属部门 ID
    Avatar     string      // 头像引用 ID
    Roles      []RoleModel // 多对多角色关联
    Remark     string      // 备注
}
```

关联表 `user_role` 维护用户与角色的多对多关系：

```go
type UserRole struct {
    UserModelID uint64 `gorm:"primaryKey"`
    RoleModelID uint64 `gorm:"primaryKey"`
}
```

### 新增用户

新增用户的完整流程：分布式锁 -> 数据权限校验 -> 事务内唯一性检查 -> 生成默认密码 -> bcrypt 哈希 -> 写入数据库 -> 重新加载 Casbin 角色。

```go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    // 校验操作者有权管理目标部门
    if user.DeptID != 0 {
        if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
            return err
        }
    }
    // 校验操作者有权分配目标角色
    if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
        return err
    }
    // ...
}
```

事务内的关键步骤：

```go
addlock := s.UserRedis.LockAdd()
if err = addlock.Lock(ctx, time.Second*5); err != nil {
    return
}
defer func() { _ = addlock.Unlock(ctx) }()

if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
    if er := s.addCheck(txCtx, user); er != nil { // 唯一性检查
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
密码由手机号自动生成，遵循 `DefaultPasswordFromPhone` 规则。前端不允许提交自定义密码（`application.ErrSubmittedPassword`）。管理员应引导用户在首次登录后修改密码。
:::

### 修改用户

修改用户时先读取旧记录，校验数据权限，再更新：

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
    // 校验操作者有权访问当前用户
    if err = scope.EnforceUser(ctx, savedUser); err != nil {
        return err
    }
    // 校验操作者有权管理目标部门
    if user.DeptID != 0 {
        if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
            return err
        }
    }
    if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
        return err
    }
    // 委托给 use-case 执行实际更新
    if err = s.UserUseCase.UpdateUser(ctx, application.UpdateUserCommand{...}); err != nil {
        return err
    }
    // 清除数据权限缓存，下次查询重新计算
    return deleteDataScopeCache(ctx, s.DataScopeCache(), id)
}
```

### 删除用户

删除操作在事务内完成，外层清理缓存和会话：

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
            if er = scope.EnforceUser(ctx, user); er != nil { // 逐个校验权限
                return er
            }
        }
        return s.User.Delete(txCtx, ids)
    }); err != nil {
        return
    }
    // 事务提交后清理
    for _, user := range users {
        s.Options.Casbin.DeleteRolesForUser(user.Username) // 清 Casbin
        deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), user.ID) // 清认证缓存
        deleteDataScopeCache(ctx, s.DataScopeCache(), user.ID) // 清数据权限缓存
        s.Auth.RevokeUser(ctx, user.ID, authsession.StatusRevoked) // 撤销会话
    }
    return firstCacheErr
}
```

### 修改密码与重置密码

修改密码需要验证旧密码，重置密码不需要：

```go
// 修改密码 — 需要旧密码验证
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
    // 清缓存并撤销会话
    deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), id)
    err = s.Auth.RevokeUser(ctx, id, authsession.StatusRevoked)
    return
}

// 重置密码 — 不需要旧密码，受数据权限约束
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

### 部门模型与树结构

部门使用 `parent_id` 和 `code` 路径编码实现层级树。`code` 字段存储从根到当前节点的路径 ID 用 `-` 拼接：

```go
type DeptModel struct {
    xorm.Model
    Code     string      // 路径编码，如 "1-5-12"
    ParentID uint64      // 上级部门 ID，顶级为 0
    DeptName string      // 部门名称
    Leader   string      // 负责人
    Phone    string      // 联系电话
    Email    string      // 邮箱
    Priority int32       // 排序值
    Status   int32       // 状态：1-正常, 2-禁用
    Level    int32       // 层级，最大 5
    Childs   []DeptModel // 子部门
}
```

::: tip
`code` 路径编码允许通过 `WHERE code LIKE '1-5-%'` 高效查询整棵子树，无需递归 CTE。
:::

查询子树 ID：

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

查询祖先链路：

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

### 用户列表查询

用户列表支持分页、排序、部门范围、角色筛选和姓名/用户名模糊搜索：

```go
func (s *UserService) getUserList(ctx context.Context, opt store.UserModelGetListOption,
    scopes ...func(*gorm.DB) *gorm.DB) (users []*store.UserModel, total int64, err error) {

    dataScope, err := s.resolveDataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    scopes = append(scopes, dataScope.UserScope()) // 注入数据权限

    if len(opt.Deptids) != 0 {
        ids, er := s.Dept.GetSubtreeIDs(ctx, opt.Deptids[0])
        if er != nil {
            return nil, 0, er
        }
        opt.Deptids = ids // 展开为完整子树
    }
    return s.User.GetList(ctx, opt, scopes...)
}
```

调用方选择不同的搜索方式：

```go
// 按姓名/用户名模糊搜索
users, total, err := s.GetUserListNameOrUserName(ctx, opt, name)

// 仅按用户名模糊搜索
users, total, err := s.GetUserListUserName(ctx, opt, name)
```

## 配置示例

用户服务配置位于 `configs/user/config.toml`：

```toml
[app.user]
adminPassword = "123456"           # admin 初始密码
jwtExpire = 604800                 # JWT 过期时间（秒）
heartbeatOfflineSeconds = 660      # 心跳超时下线阈值
multiLoginEnabled = true           # 是否允许多端登录
maxLoginClient = 2                 # 最多同时登录设备数
```

::: warning
生产环境必须通过 `EGOADMIN_` 前缀环境变量覆盖敏感配置，禁止在代码仓库中提交真实密码或密钥。
:::

## 实际示例

### 新增用户的 Controller 调用

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

### 在线用户管理

```go
// 心跳 — 只刷新在线状态，不撤销会话
func (s *UserGRPC) HeartBeatUser(ctx context.Context, in *userv1.HeartBeatUserRequest) (*userv1.HeartBeatUserResponse, error) {
    auth, ok := authsession.FromContext(ctx)
    if !ok {
        return nil, platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)
    }
    return &userv1.HeartBeatUserResponse{}, s.userUseCase.MarkUserOnline(ctx, auth.UserID)
}
```

## 工作原理

1. **数据权限前置**：所有读写操作调用 `resolveDataScope(ctx)` 获取当前用户的数据权限范围。
2. **分布式锁**：新增和修改操作使用 Redis 分布式锁防止并发冲突。
3. **事务保护**：涉及多表写入（用户 + 角色关联）在事务内完成。
4. **缓存一致性**：修改用户、部门或角色后清除相关数据权限缓存和认证快照缓存。
5. **会话管理**：删除用户或修改密码时撤销该用户的所有活跃会话。

```text
请求进入 gRPC Controller
  -> 解析认证上下文（authsession）
  -> 调用 resolveDataScope 获取数据权限
  -> DataScope.EnforceUser / EnforceDeptID 校验
  -> 获取分布式锁
  -> 开启事务
    -> 唯一性检查
    -> 业务操作
  -> 提交事务
  -> 清理缓存 + 重新加载 Casbin
```

## 常见问题

::: danger 数据权限未生效
确保每次查询前调用 `resolveDataScope(ctx)` 并将 `scope.UserScope()` 作为 GORM scope 注入。遗漏此步骤会导致普通用户看到超出权限的数据。
:::

::: danger 部门删除失败
删除部门前检查 `CountByDeptIds`，如果部门下仍有用户则无法删除。前端应调用 `CheckDeleteDept` 接口预检查。
:::

::: tip 密码重置后的缓存失效
重置密码后需要清除 `AuthSnapshotCache` 和 `DataScopeCache`，并撤销用户会话。否则用户仍可使用旧的 access token 访问系统。
:::

::: danger Casbin 角色不同步
新增用户后必须调用 `reloadCasbinRolesForUser`，否则 Casbin 中没有该用户的角色信息，protected API 将返回 403。
:::

## 参考链接

- [权限系统](/guide/zh-CN/permission-system) — API 分类、Casbin 校验、routeMenu
- [角色与权限](/guide/zh-CN/user-service/role-permission) — 角色 CRUD 与权限分配
- [数据权限](/guide/zh-CN/user-service/data-permission) — DataScope 行级权限详解
- [审计日志](/guide/zh-CN/user-service/audit-log) — 操作审计与合规
