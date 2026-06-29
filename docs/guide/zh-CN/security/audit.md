# 安全审计

EgoAdmin 提供内置的审计日志系统，记录用户操作行为，支持按模块、类型、时间范围查询，满足安全合规和问题追溯需求。

## 概述

审计日志覆盖所有关键操作：登录/登出、用户管理、角色管理、部门管理等。日志采用异步写入，不影响业务接口性能。查询接口支持数据权限控制，确保用户只能查看权限范围内的审计记录。

```text
业务操作发生
  -> Controller defer 块记录审计日志
  -> 异步写入数据库（非阻塞）
  -> 查询时应用 DataScope 数据权限
  -> 定时清理过期日志（2 年保留期）
```

## 核心用法

### 审计日志模型

每条审计日志包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | int64 | 操作用户 ID |
| `username` | string | 操作用户名 |
| `dept` | string | 所属部门 |
| `module` | string | 操作模块（如 user、role、dept） |
| `type` | string | 操作类型（如 create、update、delete、login、logout） |
| `title` | string | 操作描述 |
| `url` | string | 请求 API 路径 |
| `method` | string | HTTP/gRPC 方法名 |
| `ip` | string | 客户端 IP |
| `params` | string | 请求参数（脱敏后） |
| `created_at` | time | 操作时间 |

### Controller 集成

在 controller 方法中使用 defer 块记录审计日志：

```go
func (c *UserController) AddUser(ctx context.Context, req *pb.AddUserRequest) (*pb.AddUserResponse, error) {
    // 从 context 获取操作用户信息
    currentUser := authsession.GetUserFromContext(ctx)

    defer func() {
        c.logger.Save(ctx, "user", "create", "新增用户", req)
    }()

    // 业务逻辑
    result, err := c.userApp.AddUser(ctx, req)
    if err != nil {
        return nil, err
    }

    return result, nil
}
```

::: tip
使用 `defer` 确保无论业务成功还是失败，审计日志都会被记录。审计日志记录的是"尝试操作"，而非仅"成功操作"。
:::

### 异步写入

审计日志采用异步方式写入数据库，不阻塞业务请求：

```go
// internal/component/auditlog/logger.go
func (l *AuditLogger) Save(ctx context.Context, module, typ, title string, params any) {
    // 从 context 提取用户信息
    user := authsession.GetUserFromContext(ctx)
    ip := extractClientIP(ctx)

    // 参数脱敏
    sanitizedParams := desensitize(params)

    log := &AuditLog{
        UserID:    user.ID,
        Username:  user.Username,
        Dept:      user.DeptName,
        Module:    module,
        Type:      typ,
        Title:     title,
        URL:       extractURL(ctx),
        Method:    extractMethod(ctx),
        IP:        ip,
        Params:    sanitizedParams,
        CreatedAt: time.Now(),
    }

    // 异步写入，不阻塞调用方
    go func() {
        if err := l.db.Create(log).Error; err != nil {
            slog.Error("failed to save audit log", "error", err)
        }
    }()
}
```

::: warning
异步写入意味着日志可能在服务异常退出时丢失。对于关键安全操作（如密码重置、权限变更），可以考虑同步写入或使用消息队列保证投递。
:::

### 参数脱敏

敏感字段在记录前自动脱敏：

```go
func desensitize(params any) string {
    data, _ := json.Marshal(params)
    m := make(map[string]any)
    json.Unmarshal(data, &m)

    // 脱敏敏感字段
    sensitiveFields := []string{
        "password", "passwordCipher", "oldPassword", "newPassword",
        "token", "refreshToken", "secret", "captchaCode",
    }

    for _, field := range sensitiveFields {
        if _, ok := m[field]; ok {
            m[field] = "***"
        }
    }

    result, _ := json.Marshal(m)
    return string(result)
}
```

脱敏前后对比：

```text
原始参数：
{"username":"admin","password":"123456","phone":"13800138000"}

脱敏后：
{"username":"admin","password":"***","phone":"13800138000"}
```

::: danger
密码、token、密钥等敏感字段必须脱敏后才能写入审计日志。直接记录明文密码是严重的安全漏洞。
:::

### 数据权限控制

审计日志查询遵循 DataScope 数据权限规则：

```go
func (r *AuditLogRepo) List(ctx context.Context, q Query, scope permission.DataScope) ([]*AuditLog, int64, error) {
    db := r.db.WithContext(ctx).Model(&AuditLogModel{})

    // 应用查询条件
    if q.Module != "" {
        db = db.Where("module = ?", q.Module)
    }
    if q.Type != "" {
        db = db.Where("type = ?", q.Type)
    }
    if !q.StartTime.IsZero() {
        db = db.Where("created_at >= ?", q.StartTime)
    }
    if !q.EndTime.IsZero() {
        db = db.Where("created_at <= ?", q.EndTime)
    }

    // 应用数据权限
    db = applyDataScope(db, scope)

    var total int64
    db.Count(&total)

    var logs []*AuditLogModel
    db.Order("created_at DESC").
        Offset((q.Page - 1) * q.PageSize).
        Limit(q.PageSize).
        Find(&logs)

    return toAuditLogs(logs), total, nil
}
```

数据权限范围说明：

| 范围 | 可查看的审计日志 |
|------|----------------|
| 本人 | 仅自己的操作日志 |
| 本部门 | 当前部门所有用户的操作日志 |
| 本部门及子部门 | 当前部门树下所有用户的操作日志 |
| 全部 | 所有用户的操作日志（admin/root） |
| 自定义 | 指定部门集合内用户的操作日志 |

### 日志保留策略

超过保留期的日志由定时任务清理：

```text
保留策略：
  -> 默认保留 2 年
  -> 定时 cron job 每天凌晨执行
  -> 删除 created_at 超过保留期的记录
  -> 支持配置保留时长
```

清理任务示例：

```go
// 定时任务注册
func RegisterCleanupJob(db *gorm.DB, retentionDays int) {
    cron.AddFunc("0 2 * * *", func() {
        cutoff := time.Now().AddDate(0, 0, -retentionDays)
        result := db.Where("created_at < ?", cutoff).Delete(&AuditLogModel{})
        if result.Error != nil {
            slog.Error("audit log cleanup failed", "error", result.Error)
        } else {
            slog.Info("audit log cleanup completed", "deleted", result.RowsAffected)
        }
    })
}
```

### 查询 API

`GetLogList` 接口支持多条件筛选：

```protobuf
// proto 定义
rpc GetLogList(GetLogListRequest) returns (GetLogListResponse);

message GetLogListRequest {
  string module = 1;
  string type = 2;
  string username = 3;
  int64 start_time = 4;
  int64 end_time = 5;
  int64 page = 6;
  int64 page_size = 7;
}
```

查询示例：

```text
查询最近 7 天的登录日志：
  module = ""（不筛选模块）
  type = "login"
  start_time = 7天前的时间戳
  end_time = 当前时间戳
  page = 1
  page_size = 20

查询用户管理模块的操作：
  module = "user"
  type = ""（不筛选类型）
  page = 1
  page_size = 20
```

### IP 追踪

通过 `x-forwarded-for` header 获取客户端真实 IP：

```go
func extractClientIP(ctx context.Context) string {
    // 优先从 gRPC metadata 获取 x-forwarded-for
    md, _ := metadata.FromIncomingContext(ctx)
    if forwarded := md.Get("x-forwarded-for"); len(forwarded) > 0 {
        // 取第一个 IP（客户端真实 IP）
        parts := strings.Split(forwarded[0], ",")
        return strings.TrimSpace(parts[0])
    }

    // 回退到 peer address
    p, ok := peer.FromContext(ctx)
    if ok {
        return p.Addr.String()
    }

    return "unknown"
}
```

::: tip
当请求经过 Nginx 等反向代理时，`x-forwarded-for` 头部携带客户端真实 IP。确保 Nginx 配置了 `proxy_set_header X-Forwarded-For $remote_addr`。
:::

### 登录/登出审计

认证事件自动记录审计日志：

```go
// 内部封装，登录成功时自动记录
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
    resp, err := s.doLogin(ctx, req)

    defer func() {
        if err == nil {
            s.auditLogger.Save(ctx, "auth", "login", "用户登录", map[string]any{
                "username": req.Username,
                "ip":       extractClientIP(ctx),
            })
        } else {
            s.auditLogger.Save(ctx, "auth", "login_failed", "登录失败", map[string]any{
                "username": req.Username,
                "reason":   err.Error(),
            })
        }
    }())

    return resp, err
}

// 退出登录时记录
func (s *AuthService) Logout(ctx context.Context) error {
    defer func() {
        s.auditLogger.Save(ctx, "auth", "logout", "用户登出", nil)
    }()

    return s.sessionMgr.Revoke(ctx)
}
```

### 管理操作审计

所有 CRUD 操作通过统一的 defer 模式记录：

```go
// 用户管理
func (c *UserController) AddUser(ctx context.Context, req *pb.AddUserRequest) (*pb.AddUserResponse, error) {
    defer c.logger.Save(ctx, "user", "create", "新增用户", req)
    return c.userApp.AddUser(ctx, req)
}

func (c *UserController) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
    defer c.logger.Save(ctx, "user", "update", "编辑用户", req)
    return c.userApp.UpdateUser(ctx, req)
}

func (c *UserController) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
    defer c.logger.Save(ctx, "user", "delete", "删除用户", req)
    return c.userApp.DeleteUser(ctx, req)
}

// 角色管理
func (c *RoleController) AddRole(ctx context.Context, req *pb.AddRoleRequest) (*pb.AddRoleResponse, error) {
    defer c.logger.Save(ctx, "role", "create", "新增角色", req)
    return c.roleApp.AddRole(ctx, req)
}

// 部门管理
func (c *DeptController) AddDept(ctx context.Context, req *pb.AddDeptRequest) (*pb.AddDeptResponse, error) {
    defer c.logger.Save(ctx, "dept", "create", "新增部门", req)
    return c.deptApp.AddDept(ctx, req)
}
```

## 配置示例

### 审计日志模块配置

```toml
[component.auditlog]
# 日志保留天数
retentionDays = 730
# 是否启用异步写入
asyncWrite = true
# 脱敏字段列表
sensitiveFields = ["password", "passwordCipher", "token", "secret"]
```

### 数据库表结构

```sql
CREATE TABLE `audit_log` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL DEFAULT 0,
  `username` varchar(64) NOT NULL DEFAULT '',
  `dept` varchar(128) NOT NULL DEFAULT '',
  `module` varchar(32) NOT NULL DEFAULT '',
  `type` varchar(32) NOT NULL DEFAULT '',
  `title` varchar(128) NOT NULL DEFAULT '',
  `url` varchar(255) NOT NULL DEFAULT '',
  `method` varchar(32) NOT NULL DEFAULT '',
  `ip` varchar(64) NOT NULL DEFAULT '',
  `params` text,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_module_type` (`module`, `type`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

索引说明：

| 索引 | 用途 |
|------|------|
| `idx_user_id` | 按用户查询操作日志 |
| `idx_module_type` | 按模块和类型筛选 |
| `idx_created_at` | 按时间范围查询和清理 |

## 实战场景

### 场景一：安全事件追溯

```text
场景：发现某用户数据异常修改
  1. 查询该用户的操作日志：username = "xxx"
  2. 筛选修改操作：type = "update"
  3. 查看时间范围内的详细操作记录
  4. 通过 IP 字段确认操作来源
  5. 通过 params 字段查看具体修改内容
```

### 场景二：登录安全监控

```text
查询所有登录失败记录：
  module = "auth"
  type = "login_failed"
  start_time = "2024-01-01"
  end_time = "2024-01-02"

分析结果：
  -> 同一 IP 大量失败：疑似暴力破解
  -> 同一用户多地登录：账号可能被盗
  -> 异常时间段登录：需要关注
```

### 场景三：合规审计报告

```text
生成月度审计报告：
  1. 查询所有模块的操作统计
  2. 按用户统计操作频次
  3. 统计登录/登出记录
  4. 导出为 CSV/Excel 供合规审查
```

## 工作原理

### 审计日志写入流程

```text
1. Controller 方法执行业务逻辑
2. defer 块在方法返回前触发
3. 从 context 提取操作用户和 IP 信息
4. 对参数进行脱敏处理
5. 构造 AuditLog 结构体
6. 异步 goroutine 写入数据库
7. 写入失败记录错误日志（不影响业务）
```

### 数据权限在审计查询中的应用

```text
1. 用户请求审计日志列表
2. 从 context 获取当前用户信息
3. 查询当前用户的数据权限范围
4. 拼接 SQL 条件：
   - 本人：WHERE user_id = ?
   - 本部门：WHERE dept = ?
   - 本部门及子部门：WHERE dept IN (...)
   - 全部：不加条件
5. 执行查询并返回结果
```

## 常见问题

### 审计日志对性能有影响吗？

异步写入模式下，审计日志对业务接口的性能影响极小（仅构造日志对象的时间）。数据库写入在独立的 goroutine 中完成，不阻塞业务请求。

### 如何保证审计日志不被篡改？

当前实现中审计日志与业务数据在同一数据库。如需防篡改，可以：

1. 使用独立的审计数据库，限制写入权限
2. 对日志记录计算哈希链
3. 定期将日志归档到只读存储

### 日志量太大怎么办？

1. 确保 `idx_created_at` 索引存在
2. 缩短保留期（如改为 1 年）
3. 定期归档历史日志到冷存储
4. 按模块或类型分表（如日志量特别大）

### 如何查看某次操作的详细参数？

通过 `GetLogList` 接口查询，`params` 字段包含请求参数的 JSON 字符串（已脱敏）。对于需要完整参数的场景，可以临时关闭脱敏（仅限开发环境）。

### 数据权限对审计查询有什么影响？

普通用户只能查看自己权限范围内的审计日志。例如，本部门权限的用户只能看到本部门用户的操作记录。admin/root 用户可以查看所有日志。

## 参考链接

- [认证与会话安全](/guide/zh-CN/security/auth-security) -- 登录/登出审计相关
- [攻击防护](/guide/zh-CN/security/attack-protection) -- 安全防护策略
- [权限系统](/guide/zh-CN/permission-system) -- DataScope 数据权限说明
- `internal/component/auditlog` -- 审计日志组件源码
- `atlas/migrations/user` -- 数据库迁移文件
