# 审计日志

审计日志是 EgoAdmin 用户服务的合规与调试基础设施，记录所有关键操作的执行者、时间、参数和结果，支持按数据权限范围查询，并自动清理过期记录。

## 概述

EgoAdmin 的审计日志系统采用异步写入模式，在 Controller 层通过 `defer` 语句捕获成功操作，异步序列化并持久化到 `sys_log` 表。日志参数自动脱敏（密码、token 等敏感字段替换为 `***`），查询受 DataScope 行级权限约束。

日志生命周期由定时任务管理：超过 2 年的记录自动清理。

## 核心用法

### 日志模型

日志存储在 `sys_log` 表中：

```go
type LogModel struct {
    xorm.Model
    UserID     string // 用户 ID（字符串，兼容旧数据）
    UserIDU64  uint64 // 用户 ID（数值列，用于索引和权限过滤）
    Username   string // 用户名
    DeptID     string // 部门 ID（字符串）
    DeptIDU64  uint64 // 部门 ID（数值列）
    DeptName   string // 部门全称
    Typ        string // 操作类型
    ModuleName string // 模块名
    Title      string // 标题（如"创建用户"）
    URL        string // 访问链接
    Method     string // 请求方法（gRPC method）
    ClientIP   string // 客户端 IP
    Params     string // 请求参数（JSON，已脱敏）
    Remark     string // 备注
}
```

::: tip
`UserIDU64` 和 `DeptIDU64` 是为数据权限过滤优化的数值索引列。数据权限的 `LogScope()` 使用这两个列进行 `WHERE` 条件注入。
:::

### 审计日志适配器

`auditlog` 包提供统一的日志记录接口：

```go
type Loger interface {
    Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string)
}
```

参数说明：

| 参数 | 含义 | 示例 |
|------|------|------|
| `fnName` | 功能名称 | "用户管理-用户"、"系统管理-角色管理" |
| `typ` | 操作类型 | "新增"、"编辑"、"删除"、"登录"、"登出"、"强退" |
| `action` | 请求名称 | "新增用户"、"编辑角色"、"登出" |
| `req` | 请求参数 | protobuf 请求结构体指针 |
| `remarks` | 备注 | 可选的额外说明 |

内部实现：

```go
func (o *options) Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string) {
    ctx = context.WithoutCancel(ctx) // 异步写入不随请求取消

    // 序列化并脱敏参数
    if req != nil {
        reqBytes, err = json.Marshal(req)
        reqBytes = maskJSONSecrets(reqBytes)
    }

    // 从认证上下文提取用户信息
    auth, ok := authsession.FromContext(ctx)
    if ok {
        detail.Username = auth.Username
        detail.UserID = auth.UserIDString()
        detail.DeptID = auth.DeptIDString()
    }

    // 从 gRPC metadata 提取 URL 和客户端 IP
    md := metadata.ExtractIncoming(ctx)
    detail.URL = md.Get("url")
    detail.OriginIP = md.Get("x-forwarded-for")
    detail.GrpcMethod = grpc.FromContext(ctx)

    // 异步写入数据库
    go func(ctx context.Context, detail AccessLogDetail) {
        if err := o.sf(ctx, detail); err != nil {
            elog.Error("日志记录失败", elog.FieldErr(err))
        }
    }(ctx, detail)
}
```

### 参数脱敏

自动识别并替换敏感字段：

```go
func isSecretLogKey(key string) bool {
    switch strings.ToLower(key) {
    case "password", "oldpassword", "old_password", "newpassword", "new_password",
        "passwordcipher", "password_cipher", "privatekey", "private_key",
        "keyid", "key_id", "challengeid", "challenge_id", "nonce",
        "captchacode", "captcha_code", "token", "refreshtoken", "refresh_token",
        "authorization":
        return true
    default:
        return false
    }
}
```

脱敏效果示例：

```json
{
  "username": "zhangsan",
  "password": "***",
  "passwordCipher": "***",
  "phone": "13800138000"
}
```

::: danger
所有包含 `password`、`token`、`captcha` 等关键词的字段值都会被替换为 `***`。如果自定义请求中包含其他敏感字段，请确保字段名不与上述列表冲突，或扩展 `isSecretLogKey`。
:::

### Controller 层集成

审计日志在 Controller 层通过 `defer` 模式记录，仅在操作成功时写入：

```go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    out = &userv1.AddUserResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
        }
    }()
    // ... 业务逻辑
    return
}
```

各类操作的审计日志写法：

```go
// 用户新增
s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)

// 用户编辑
s.logger.Save(ctx, "用户管理-用户", "编辑", "编辑用户", in)

// 用户删除
s.logger.Save(ctx, "用户管理-用户", "删除", "删除用户", in)

// 重置密码
s.logger.Save(ctx, "用户管理-用户", "编辑", "重置密码", in)

// 强退用户
s.logger.Save(ctx, "用户管理-在线用户", "强退", "强退用户", in)

// 登录
s.logger.Save(authsession.NewContext(ctx, resp.Auth), "用户-登录退出", "登录", "登录", in)

// 登出
s.logger.Save(ctx, "用户-登录退出", "登出", "登出", in)

// 部门新增
s.logger.Save(ctx, "用户管理-组织管理", "新增", "新增组织", in)

// 角色新增
s.logger.Save(ctx, "系统管理-角色管理", "新增", "新增角色", in)
```

::: tip
`defer` 加 `err == nil` 守卫是标准模式：只有操作成功时才记录审计日志。避免记录失败操作产生误导性审计记录。
:::

### 日志查询 API

日志查询支持分页、排序、用户名搜索、事件标题搜索和时间范围过滤：

```go
func (s *LogGRPC) GetLogList(ctx context.Context, in *userv1.GetLogListRequest) (out *userv1.GetLogListResponse, err error) {
    out = &userv1.GetLogListResponse{
        Logs: make([]*userv1.SysLog, 0),
    }
    opt := store.LogModelGetListOption{
        Pgopt: xorm.PaginateOption{
            Page:  int(in.GetPage()),
            Limit: int(in.GetLimit()),
            Sort:  in.GetSort(),
            Order: in.GetOrder(),
        },
        Username:  in.GetUsername(),  // 用户名模糊搜索
        Event:     in.GetEvent(),     // 事件标题模糊搜索
        StartTime: xtime.Ts2Time(in.GetTimeRange().GetStart()),
        EndTime:   xtime.Ts2Time(in.GetTimeRange().GetEnd()),
    }
    logs, total, err := s.log.GetList(ctx, opt)
    // ...
}
```

底层查询实现：

```go
func (m *Log) GetList(ctx context.Context, opt LogModelGetListOption,
    scopes ...func(*gorm.DB) *gorm.DB) (logs []*LogModel, total int64, err error) {

    db := mysql.DBWithContext(ctx, m.cc)
    scopes = append(scopes,
        logScopeUsernameLike(opt.Username),   // username LIKE '%name%'
        logScopeTitleLike(opt.Event),         // title LIKE '%event%'
        logScopeCratedAtRange(opt.StartTime, opt.EndTime), // created_at BETWEEN
    )
    if opt.Pgopt.Sort == "" {
        opt.Pgopt.Sort = createAt
        opt.Pgopt.Order = desc // 默认按创建时间倒序
    }
    // 先 count，再分页
    err = db.Scopes(scopes...).Model(&LogModel{}).Count(&total).Error
    if err != nil {
        return
    }
    scopes = append(scopes, xorm.WithScopePaginate(opt.Pgopt)...)
    err = db.Scopes(scopes...).Find(&logs).Error
    return
}
```

### 数据权限

日志查询受 DataScope 约束，通过 `scope.LogScope()` 注入：

```go
func (s *LogService) GetList(ctx context.Context, opt store.LogModelGetListOption) (logs []*store.LogModel, total int64, err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    return s.Log.GetList(ctx, opt, scope.LogScope())
}
```

`LogScope` 使用 `user_id_u64` 和 `dept_id_u64` 数值列进行过滤：

```go
func (s DataScope) LogScope() func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if s.IsAdmin || s.Level == DataScopeAll {
            return db
        }
        switch s.Level {
        case DataScopeDeptAndSub, DataScopeDeptSelf:
            return db.Where(uint64ColumnIn("dept_id_u64", s.DeptIDs))
        case DataScopeSelf:
            return db.Where(clause.Eq{Column: "user_id_u64", Value: s.UserID})
        default:
            return dataScopeDeny(db)
        }
    }
}
```

### 日志清理

定时任务清理超过 2 年的日志：

```go
func (s *LogService) CleanLog(ctx context.Context) (err error) {
    beforeDate := time.Now().AddDate(-2, 0, -1) // 两年前
    err = s.Log.DeleteLogBeforeDate(ctx, beforeDate)
    return
}
```

底层实现直接物理删除：

```go
func (m *Log) DeleteLogBeforeDate(ctx context.Context, date time.Time) (err error) {
    db := mysql.DBWithContext(ctx, m.cc)
    err = db.Unscoped().Where("created_at < ?", date).Delete(&LogModel{}).Error
    return
}
```

::: warning
日志清理使用 `Unscoped()` 物理删除，不可恢复。清理周期为代码硬编码的 2 年，不是配置项。如需调整，修改 `CleanLog` 方法。
:::

## 配置示例

审计日志依赖 MySQL 存储和 Redis（用于用户数据权限解析）：

```toml
[component.jetcache]
[component.jetcache.default]
remote = "redis"
prefix = "egoadmin"
```

日志适配器的初始化：

```go
logger := auditlog.New(func(ctx context.Context, alog auditlog.AccessLogDetail) error {
    // 将 AccessLogDetail 转换为 LogModel 并保存
    return logStore.Save(ctx, &store.LogModel{
        UserID:     alog.UserID,
        UserIDU64:  parseUint64(alog.UserID),
        Username:   alog.Username,
        DeptID:     alog.DeptID,
        DeptIDU64:  parseUint64(alog.DeptID),
        Title:      alog.Action,
        URL:        alog.URL,
        Method:     alog.GrpcMethod,
        ClientIP:   alog.OriginIP,
        Params:     alog.Params,
        Remark:     alog.Remark,
    })
})
```

## 实际示例

### 登录审计日志

登录操作比较特殊，需要手动构建包含认证信息的 context：

```go
func (s *UserGRPC) Login(ctx context.Context, in *userv1.LoginRequest) (out *userv1.LoginResponse, err error) {
    // ... 登录逻辑
    resp, err := s.userUseCase.Login(ctx, application.LoginCommand{...})
    if err != nil {
        return
    }
    // 非续期登录时记录日志
    if in.GetToken() == "" {
        s.logger.Save(authsession.NewContext(ctx, resp.Auth), "用户-登录退出", "登录", "登录", in)
    }
    // ...
}
```

::: tip
登录时请求 context 中还没有认证信息（因为 token 尚未签发），所以需要用 `authsession.NewContext(ctx, resp.Auth)` 包装登录结果中的认证上下文。
:::

### 部门管理审计日志

```go
func (s *DeptGRPC) AddDept(ctx context.Context, in *userv1.AddDeptRequest) (out *userv1.AddDeptResponse, err error) {
    out = &userv1.AddDeptResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-组织管理", "新增", "新增组织", in)
        }
    }()
    dept := &store.DeptModel{
        ParentID: in.GetParentId(),
        DeptName: in.GetDept().GetDeptName(),
        Leader:   in.GetDept().GetLeader(),
    }
    err = s.dept.AddDept(ctx, dept)
    out.Id = dept.ID
    return
}
```

### 角色管理审计日志

```go
func (s *RoleGRPC) UpdateRole(ctx context.Context, in *userv1.UpdateRoleRequest) (out *userv1.UpdateRoleResponse, err error) {
    out = &userv1.UpdateRoleResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "系统管理-角色管理", "编辑", "编辑角色", in)
        }
    }()
    err = mapRoleError(ctx, s.role.UpdateRole(ctx, in.GetId(), &store.RoleModel{...}))
    return
}
```

## 工作原理

```text
gRPC Controller 方法执行
  -> defer 注册日志记录回调
  -> 执行业务逻辑
  -> 方法返回时：
    -> 如果 err == nil（操作成功）：
      -> 调用 logger.Save(ctx, module, type, title, request)
      -> context.WithoutCancel 防止请求取消影响日志写入
      -> JSON 序列化请求参数
      -> maskJSONSecrets 脱敏敏感字段
      -> 从 authsession 提取 user/dept 信息
      -> 从 gRPC metadata 提取 URL、客户端 IP
      -> goroutine 异步写入数据库
    -> 如果 err != nil（操作失败）：
      -> 跳过日志记录
```

日志查询的 DataScope 过滤链路：

```text
GetLogList 请求
  -> LogService.GetList
    -> resolveDataScope(ctx) 获取当前用户数据权限
    -> scope.LogScope() 生成 WHERE 条件
    -> 注入 GORM 查询
    -> 返回过滤后的日志列表
```

## 常见问题

::: danger 审计日志缺失
确保在 Controller 的 `defer` 中使用 `err == nil` 守卫。如果 guard 条件写反，成功操作不会记录日志。如果 `logger.Save` 被注释掉或适配器未正确初始化，也不会写入。
:::

::: danger 日志查询返回空结果
普通用户受 DataScope 约束，只能看到与自己同部门或由自己操作的日志。如果数据权限为"仅本人"（`DataScopeSelf`），只能看到自己的操作记录。
:::

::: warning 异步写入的数据一致性
日志使用 `go func()` 异步写入，理论上存在写入失败但请求已返回的情况。审计日志不阻塞业务操作。如果对日志完整性有严格要求，需要额外的补偿机制。
:::

::: tip 脱敏规则扩展
如果业务中新增了敏感字段（如银行卡号、身份证号），需要在 `isSecretLogKey` 中添加对应的关键词，确保日志中不记录明文敏感信息。
:::

## 参考链接

- [权限系统](/guide/zh-CN/permission-system) — API 分类、Casbin 校验、routeMenu
- [用户与部门管理](/guide/zh-CN/user-service/user-dept) — 用户 CRUD 与部门树
- [角色与权限](/guide/zh-CN/user-service/role-permission) — 角色 CRUD 与权限分配
- [数据权限](/guide/zh-CN/user-service/data-permission) — DataScope 行级权限详解
