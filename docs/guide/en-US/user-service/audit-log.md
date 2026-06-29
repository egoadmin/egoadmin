# Audit Log

The audit log is EgoAdmin's compliance and debugging infrastructure, recording the executor, timing, parameters, and outcome of all critical operations. It supports queries constrained by data permission scope and automatically cleans up expired records.

## Overview

EgoAdmin's audit log system uses an asynchronous write pattern. At the controller layer, `defer` statements capture successful operations, serialize them asynchronously, and persist them to the `sys_log` table. Log parameters are automatically desensitized (sensitive fields like passwords and tokens are replaced with `***`), and queries are constrained by DataScope row-level permissions.

Log lifecycle is managed by a scheduled job: records older than 2 years are automatically purged.

## Core Usage

### Log Model

Logs are stored in the `sys_log` table:

```go
type LogModel struct {
    xorm.Model
    UserID     string // User ID (string, legacy compatibility)
    UserIDU64  uint64 // User ID (numeric column, for indexing and permission filtering)
    Username   string // Username
    DeptID     string // Department ID (string)
    DeptIDU64  uint64 // Department ID (numeric column)
    DeptName   string // Full department name
    Typ        string // Operation type
    ModuleName string // Module name
    Title      string // Title (e.g. "Create User")
    URL        string // Access URL
    Method     string // Request method (gRPC method)
    ClientIP   string // Client IP address
    Params     string // Request parameters (JSON, desensitized)
    Remark     string // Remark
}
```

::: tip
`UserIDU64` and `DeptIDU64` are numeric index columns optimized for data permission filtering. DataScope's `LogScope()` uses these columns for `WHERE` clause injection.
:::

### Audit Log Adapter

The `auditlog` package provides a unified logging interface:

```go
type Loger interface {
    Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string)
}
```

Parameter descriptions:

| Parameter | Meaning | Example |
|-----------|---------|---------|
| `fnName` | Function/module name | "用户管理-用户", "系统管理-角色管理" |
| `typ` | Operation type | "新增", "编辑", "删除", "登录", "登出", "强退" |
| `action` | Request name | "新增用户", "编辑角色", "登出" |
| `req` | Request parameters | Protobuf request struct pointer |
| `remarks` | Remark | Optional additional notes |

Internal implementation:

```go
func (o *options) Save(ctx context.Context, fnName string, typ string, action string, req any, remarks ...string) {
    ctx = context.WithoutCancel(ctx) // Async write survives request cancellation

    // Serialize and desensitize parameters
    if req != nil {
        reqBytes, err = json.Marshal(req)
        reqBytes = maskJSONSecrets(reqBytes)
    }

    // Extract user info from auth context
    auth, ok := authsession.FromContext(ctx)
    if ok {
        detail.Username = auth.Username
        detail.UserID = auth.UserIDString()
        detail.DeptID = auth.DeptIDString()
    }

    // Extract URL and client IP from gRPC metadata
    md := metadata.ExtractIncoming(ctx)
    detail.URL = md.Get("url")
    detail.OriginIP = md.Get("x-forwarded-for")
    detail.GrpcMethod = grpc.FromContext(ctx)

    // Async database write
    go func(ctx context.Context, detail AccessLogDetail) {
        if err := o.sf(ctx, detail); err != nil {
            elog.Error("日志记录失败", elog.FieldErr(err))
        }
    }(ctx, detail)
}
```

### Parameter Desensitization

Automatically identifies and replaces sensitive fields:

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

Desensitization example:

```json
{
  "username": "zhangsan",
  "password": "***",
  "passwordCipher": "***",
  "phone": "13800138000"
}
```

::: danger
All field values containing keywords like `password`, `token`, or `captcha` are replaced with `***`. If custom requests include other sensitive fields, ensure their names do not collide with the list above, or extend `isSecretLogKey`.
:::

### Controller Integration

Audit logs are recorded at the controller layer via the `defer` pattern, only for successful operations:

```go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    out = &userv1.AddUserResponse{}
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
        }
    }()
    // ... business logic
    return
}
```

Audit log patterns for various operations:

```go
// User creation
s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)

// User update
s.logger.Save(ctx, "用户管理-用户", "编辑", "编辑用户", in)

// User deletion
s.logger.Save(ctx, "用户管理-用户", "删除", "删除用户", in)

// Password reset
s.logger.Save(ctx, "用户管理-用户", "编辑", "重置密码", in)

// Force offline user
s.logger.Save(ctx, "用户管理-在线用户", "强退", "强退用户", in)

// Login
s.logger.Save(authsession.NewContext(ctx, resp.Auth), "用户-登录退出", "登录", "登录", in)

// Logout
s.logger.Save(ctx, "用户-登录退出", "登出", "登出", in)

// Department creation
s.logger.Save(ctx, "用户管理-组织管理", "新增", "新增组织", in)

// Role creation
s.logger.Save(ctx, "系统管理-角色管理", "新增", "新增角色", in)
```

::: tip
The `defer` + `err == nil` guard is the standard pattern: audit logs are only recorded for successful operations. This avoids creating misleading audit records for failed attempts.
:::

### Log Query API

Log queries support pagination, sorting, username search, event title search, and time range filtering:

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
        Username:  in.GetUsername(),  // Username fuzzy search
        Event:     in.GetEvent(),     // Event title fuzzy search
        StartTime: xtime.Ts2Time(in.GetTimeRange().GetStart()),
        EndTime:   xtime.Ts2Time(in.GetTimeRange().GetEnd()),
    }
    logs, total, err := s.log.GetList(ctx, opt)
    // ...
}
```

Underlying query implementation:

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
        opt.Pgopt.Order = desc // Default: descending by creation time
    }
    // Count first, then paginate
    err = db.Scopes(scopes...).Model(&LogModel{}).Count(&total).Error
    if err != nil {
        return
    }
    scopes = append(scopes, xorm.WithScopePaginate(opt.Pgopt)...)
    err = db.Scopes(scopes...).Find(&logs).Error
    return
}
```

### Data Permission

Log queries are constrained by DataScope through `scope.LogScope()`:

```go
func (s *LogService) GetList(ctx context.Context, opt store.LogModelGetListOption) (logs []*store.LogModel, total int64, err error) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    return s.Log.GetList(ctx, opt, scope.LogScope())
}
```

`LogScope` uses the `user_id_u64` and `dept_id_u64` numeric columns for filtering:

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

### Log Cleanup

A scheduled job cleans up logs older than 2 years:

```go
func (s *LogService) CleanLog(ctx context.Context) (err error) {
    beforeDate := time.Now().AddDate(-2, 0, -1) // Two years ago
    err = s.Log.DeleteLogBeforeDate(ctx, beforeDate)
    return
}
```

The underlying implementation performs a hard delete:

```go
func (m *Log) DeleteLogBeforeDate(ctx context.Context, date time.Time) (err error) {
    db := mysql.DBWithContext(ctx, m.cc)
    err = db.Unscoped().Where("created_at < ?", date).Delete(&LogModel{}).Error
    return
}
```

::: warning
Log cleanup uses `Unscoped()` for permanent deletion -- records cannot be recovered. The 2-year retention period is a code constant, not a configurable value. Modify the `CleanLog` method to adjust.
:::

## Configuration Examples

Audit logs depend on MySQL for storage and Redis (for user data permission resolution):

```toml
[component.jetcache]
[component.jetcache.default]
remote = "redis"
prefix = "egoadmin"
```

Audit log adapter initialization:

```go
logger := auditlog.New(func(ctx context.Context, alog auditlog.AccessLogDetail) error {
    // Convert AccessLogDetail to LogModel and persist
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

## Real-World Examples

### Login Audit Log

Login is special because the request context does not yet contain authentication info (the token has not been issued). The auth context from the login result must be wrapped manually:

```go
func (s *UserGRPC) Login(ctx context.Context, in *userv1.LoginRequest) (out *userv1.LoginResponse, err error) {
    // ... login logic
    resp, err := s.userUseCase.Login(ctx, application.LoginCommand{...})
    if err != nil {
        return
    }
    // Record log only for non-refresh logins
    if in.GetToken() == "" {
        s.logger.Save(authsession.NewContext(ctx, resp.Auth), "用户-登录退出", "登录", "登录", in)
    }
    // ...
}
```

::: tip
During login, the request context has no auth info (the token is not yet issued). Use `authsession.NewContext(ctx, resp.Auth)` to wrap the authentication context from the login result.
:::

### Department Management Audit Log

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

### Role Management Audit Log

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

## How It Works

```text
gRPC Controller method executes
  -> defer registers log recording callback
  -> Business logic runs
  -> Method returns:
    -> If err == nil (success):
      -> Call logger.Save(ctx, module, type, title, request)
      -> context.WithoutCancel prevents request cancellation from affecting log write
      -> JSON serialize request parameters
      -> maskJSONSecrets desensitizes sensitive fields
      -> Extract user/dept from authsession
      -> Extract URL, client IP from gRPC metadata
      -> goroutine async write to database
    -> If err != nil (failure):
      -> Skip log recording
```

Log query DataScope filtering chain:

```text
GetLogList request
  -> LogService.GetList
    -> resolveDataScope(ctx) gets current user's data permissions
    -> scope.LogScope() generates WHERE clauses
    -> Injected into GORM query
    -> Returns filtered log list
```

## Common Issues

::: danger Audit log missing
Ensure the Controller `defer` uses the `err == nil` guard. If the guard condition is reversed, successful operations will not be logged. If `logger.Save` is commented out or the adapter is not properly initialized, logs will not be written.
:::

::: danger Log query returns empty results
Regular users are constrained by DataScope and can only see logs from their own department or operations they performed. With `DataScopeSelf` permission, users can only see their own operation records.
:::

::: warning Async write data consistency
Logs use `go func()` for async writes. In theory, a write failure could occur after the request has already returned. Audit logs do not block business operations. If strict log completeness is required, additional compensation mechanisms are needed.
:::

::: tip Extending desensitization rules
If the business introduces new sensitive fields (e.g. bank card numbers, national IDs), add the corresponding keywords to `isSecretLogKey` to ensure plaintext sensitive information is not recorded in logs.
:::

## Reference Links

- [Permission System](/guide/en-US/permission-system) -- API classification, Casbin checks, routeMenu
- [User & Department Management](/guide/en-US/user-service/user-dept) -- User CRUD and department tree
- [Roles & Permissions](/guide/en-US/user-service/role-permission) -- Role CRUD and permission assignment
- [Data Permissions](/guide/en-US/user-service/data-permission) -- DataScope row-level access control
