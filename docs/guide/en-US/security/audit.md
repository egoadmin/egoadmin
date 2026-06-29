# Security Audit

EgoAdmin provides a built-in audit logging system that records user actions, supports querying by module, type, and time range, and meets security compliance and incident investigation needs.

## Overview

Audit logs cover all key operations: login/logout, user management, role management, department management, and more. Logs are written asynchronously to avoid impacting business API performance. The query API enforces data permission controls so users can only view audit records within their authorized scope.

```text
Business operation occurs
  -> Controller defer block records audit log
  -> Asynchronous database write (non-blocking)
  -> Query applies DataScope data permissions
  -> Scheduled cleanup of expired logs (2-year retention)
```

## Core Usage

### Audit Log Model

Each audit log entry contains the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `user_id` | int64 | Operating user ID |
| `username` | string | Operating username |
| `dept` | string | Department |
| `module` | string | Operation module (e.g., user, role, dept) |
| `type` | string | Operation type (e.g., create, update, delete, login, logout) |
| `title` | string | Operation description |
| `url` | string | Request API path |
| `method` | string | HTTP/gRPC method name |
| `ip` | string | Client IP |
| `params` | string | Request parameters (desensitized) |
| `created_at` | time | Operation time |

### Controller Integration

Use a defer block in controller methods to record audit logs:

```go
func (c *UserController) AddUser(ctx context.Context, req *pb.AddUserRequest) (*pb.AddUserResponse, error) {
    // Get operating user info from context
    currentUser := authsession.GetUserFromContext(ctx)

    defer func() {
        c.logger.Save(ctx, "user", "create", "Add User", req)
    }()

    // Business logic
    result, err := c.userApp.AddUser(ctx, req)
    if err != nil {
        return nil, err
    }

    return result, nil
}
```

::: tip
Using `defer` ensures the audit log is recorded regardless of whether the operation succeeds or fails. Audit logs record "attempted operations," not just "successful operations."
:::

### Async Writing

Audit logs are written to the database asynchronously, not blocking business requests:

```go
// internal/component/auditlog/logger.go
func (l *AuditLogger) Save(ctx context.Context, module, typ, title string, params any) {
    // Extract user info from context
    user := authsession.GetUserFromContext(ctx)
    ip := extractClientIP(ctx)

    // Desensitize parameters
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

    // Async write, does not block caller
    go func() {
        if err := l.db.Create(log).Error; err != nil {
            slog.Error("failed to save audit log", "error", err)
        }
    }()
}
```

::: warning
Async writing means logs may be lost on abnormal service exit. For critical security operations (e.g., password resets, permission changes), consider synchronous writing or using a message queue for guaranteed delivery.
:::

### Parameter Desensitization

Sensitive fields are automatically masked before logging:

```go
func desensitize(params any) string {
    data, _ := json.Marshal(params)
    m := make(map[string]any)
    json.Unmarshal(data, &m)

    // Mask sensitive fields
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

Before and after desensitization:

```text
Original parameters:
{"username":"admin","password":"123456","phone":"13800138000"}

After desensitization:
{"username":"admin","password":"***","phone":"13800138000"}
```

::: danger
Passwords, tokens, secrets, and other sensitive fields must be desensitized before writing to audit logs. Logging plaintext passwords is a critical security vulnerability.
:::

### Data Permission Control

Audit log queries follow DataScope data permission rules:

```go
func (r *AuditLogRepo) List(ctx context.Context, q Query, scope permission.DataScope) ([]*AuditLog, int64, error) {
    db := r.db.WithContext(ctx).Model(&AuditLogModel{})

    // Apply query conditions
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

    // Apply data permissions
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

Data permission scope mapping:

| Scope | Visible Audit Logs |
|-------|--------------------|
| self | Own operation logs only |
| department | All users in current department |
| department tree | All users in current department and sub-departments |
| all | All users' operation logs (admin/root) |
| custom | Users in selected departments |

### Log Retention Policy

Expired logs are cleaned up by a scheduled job:

```text
Retention policy:
  -> Default retention: 2 years
  -> Cron job runs daily at 2 AM
  -> Deletes records with created_at older than retention period
  -> Configurable retention duration
```

Cleanup job example:

```go
// Register scheduled job
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

### Query API

The `GetLogList` endpoint supports multi-condition filtering:

```protobuf
// Proto definition
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

Query examples:

```text
Query login logs from the last 7 days:
  module = "" (no module filter)
  type = "login"
  start_time = timestamp from 7 days ago
  end_time = current timestamp
  page = 1
  page_size = 20

Query user management module operations:
  module = "user"
  type = "" (no type filter)
  page = 1
  page_size = 20
```

### IP Tracking

Client real IP is obtained from the `x-forwarded-for` header:

```go
func extractClientIP(ctx context.Context) string {
    // Prefer x-forwarded-for from gRPC metadata
    md, _ := metadata.FromIncomingContext(ctx)
    if forwarded := md.Get("x-forwarded-for"); len(forwarded) > 0 {
        // Take the first IP (client real IP)
        parts := strings.Split(forwarded[0], ",")
        return strings.TrimSpace(parts[0])
    }

    // Fall back to peer address
    p, ok := peer.FromContext(ctx)
    if ok {
        return p.Addr.String()
    }

    return "unknown"
}
```

::: tip
When requests pass through a reverse proxy like Nginx, the `x-forwarded-for` header carries the client's real IP. Ensure Nginx is configured with `proxy_set_header X-Forwarded-For $remote_addr`.
:::

### Login/Logout Audit

Authentication events are automatically logged:

```go
// Internal wrapper, automatically logs on successful login
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
    resp, err := s.doLogin(ctx, req)

    defer func() {
        if err == nil {
            s.auditLogger.Save(ctx, "auth", "login", "User Login", map[string]any{
                "username": req.Username,
                "ip":       extractClientIP(ctx),
            })
        } else {
            s.auditLogger.Save(ctx, "auth", "login_failed", "Login Failed", map[string]any{
                "username": req.Username,
                "reason":   err.Error(),
            })
        }
    }())

    return resp, err
}

// Log on logout
func (s *AuthService) Logout(ctx context.Context) error {
    defer func() {
        s.auditLogger.Save(ctx, "auth", "logout", "User Logout", nil)
    }()

    return s.sessionMgr.Revoke(ctx)
}
```

### Admin Action Audit

All CRUD operations are logged using the unified defer pattern:

```go
// User management
func (c *UserController) AddUser(ctx context.Context, req *pb.AddUserRequest) (*pb.AddUserResponse, error) {
    defer c.logger.Save(ctx, "user", "create", "Add User", req)
    return c.userApp.AddUser(ctx, req)
}

func (c *UserController) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
    defer c.logger.Save(ctx, "user", "update", "Edit User", req)
    return c.userApp.UpdateUser(ctx, req)
}

func (c *UserController) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
    defer c.logger.Save(ctx, "user", "delete", "Delete User", req)
    return c.userApp.DeleteUser(ctx, req)
}

// Role management
func (c *RoleController) AddRole(ctx context.Context, req *pb.AddRoleRequest) (*pb.AddRoleResponse, error) {
    defer c.logger.Save(ctx, "role", "create", "Add Role", req)
    return c.roleApp.AddRole(ctx, req)
}

// Department management
func (c *DeptController) AddDept(ctx context.Context, req *pb.AddDeptRequest) (*pb.AddDeptResponse, error) {
    defer c.logger.Save(ctx, "dept", "create", "Add Department", req)
    return c.deptApp.AddDept(ctx, req)
}
```

## Configuration Examples

### Audit Log Module Configuration

```toml
[component.auditlog]
# Log retention days
retentionDays = 730
# Enable async writing
asyncWrite = true
# Desensitized field list
sensitiveFields = ["password", "passwordCipher", "token", "secret"]
```

### Database Table Schema

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

Index explanation:

| Index | Purpose |
|-------|---------|
| `idx_user_id` | Query operation logs by user |
| `idx_module_type` | Filter by module and type |
| `idx_created_at` | Time range queries and cleanup |

## Real-World Examples

### Example 1: Security Incident Investigation

```text
Scenario: Suspicious data modification detected
  1. Query the user's operation logs: username = "xxx"
  2. Filter modification operations: type = "update"
  3. Review detailed operation records within the time range
  4. Verify operation source via the IP field
  5. Review specific changes via the params field
```

### Example 2: Login Security Monitoring

```text
Query all failed login records:
  module = "auth"
  type = "login_failed"
  start_time = "2024-01-01"
  end_time = "2024-01-02"

Analysis:
  -> Same IP with many failures: suspected brute force
  -> Same user logging in from multiple locations: account may be compromised
  -> Login during unusual hours: needs attention
```

### Example 3: Compliance Audit Report

```text
Generate monthly audit report:
  1. Query operation statistics across all modules
  2. Count operations by user
  3. Compile login/logout records
  4. Export as CSV/Excel for compliance review
```

## How It Works

### Audit Log Write Flow

```text
1. Controller method executes business logic
2. Defer block triggers before method returns
3. Extract operating user and IP info from context
4. Desensitize parameters
5. Construct AuditLog struct
6. Async goroutine writes to database
7. Write failure logs an error (does not affect business)
```

### Data Permissions in Audit Queries

```text
1. User requests audit log list
2. Extract current user info from context
3. Query current user's data permission scope
4. Build SQL conditions:
   - self: WHERE user_id = ?
   - department: WHERE dept = ?
   - department tree: WHERE dept IN (...)
   - all: no filter
5. Execute query and return results
```

## Common Issues

### Does audit logging impact performance?

With async writing, audit logging has minimal impact on business API performance (only the time to construct the log object). Database writes complete in a separate goroutine, not blocking business requests.

### How to prevent audit log tampering?

In the current implementation, audit logs share the database with business data. For tamper-proofing:

1. Use a separate audit database with restricted write permissions
2. Compute a hash chain over log records
3. Periodically archive logs to read-only storage

### What if log volume is too large?

1. Ensure the `idx_created_at` index exists
2. Shorten the retention period (e.g., to 1 year)
3. Regularly archive historical logs to cold storage
4. Shard by module or type (for very high volumes)

### How to view detailed parameters for a specific operation?

Query via the `GetLogList` API. The `params` field contains the request parameters as a JSON string (desensitized). For scenarios requiring full parameters, desensitization can be temporarily disabled (development environment only).

### How do data permissions affect audit queries?

Regular users can only view audit logs within their permission scope. For example, a user with department-level access can only see operations from users in their department. Admin/root users can view all logs.

## Reference Links

- [Authentication and Session Security](/guide/en-US/security/auth-security) -- Login/logout audit related
- [Attack Protection](/guide/en-US/security/attack-protection) -- Security protection strategies
- [Permission System](/guide/en-US/permission-system) -- DataScope data permission documentation
- `internal/component/auditlog` -- Audit log component source code
- `atlas/migrations/user` -- Database migration files
