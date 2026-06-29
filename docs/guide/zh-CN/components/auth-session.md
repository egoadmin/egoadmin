# AuthSession 认证会话

AuthSession 是 EgoAdmin 的核心认证组件，提供基于 JWT 的会话管理能力，涵盖令牌签发、刷新、校验、撤销、多设备登录控制、心跳离线检测和强制下线。

## 概述

AuthSession 位于 `internal/component/authsession`，是所有受保护 API 的认证基础。它不关心业务逻辑，只负责：

- **签发令牌**：登录时创建 access token（JWT）和 refresh token（不透明令牌）。
- **刷新令牌**：access token 过期后通过 refresh token 获取新令牌对，旧 refresh token 立即作废（rotation）。
- **校验请求**：gRPC 中间件从 metadata 提取 Bearer token，校验后注入 `AuthContext`。
- **撤销会话**：支持单会话撤销、按用户撤销、按工作区撤销。
- **多设备登录**：控制同一用户可同时持有的会话数量和同设备策略。
- **心跳离线**：前端定期发送心跳，超时后标记用户离线，可选撤销会话。

组件依赖 Redis 存储会话索引、JetCache 缓存会话记录、IDGen 生成会话 ID 和令牌 ID。通过 Wire 注入，业务层只需依赖 `Interface` 接口。

```text
internal/component/authsession/
├── interface.go       # Interface 定义
├── component.go       # 核心实现（Issue/Refresh/Validate/Logout/Revoke）
├── config.go          # Config 和策略枚举
├── container.go       # Container + Build 生命周期
├── claims.go          # JWT Claims 和 AuthContext
├── middleware.go       # gRPC 中间件（Server / ServerStream）
├── records.go         # SessionRecord / AccessRecord / RefreshRecord
├── errors.go          # 错误码和 ecode 映射
├── cache.go           # recordCache（JetCache 适配）
├── store.go           # indexStore（Redis 索引）
├── keys.go            # Redis key 构建
├── id.go              # IDGenerator 接口
├── provider.go        # Wire ProviderSet
└── options.go         # 功能选项
```

## 核心用法

### 接口定义

```go
type Interface interface {
    Issue(ctx context.Context, req IssueRequest) (*IssueResult, error)
    Refresh(ctx context.Context, req RefreshRequest) (*IssueResult, error)
    ValidateAccessToken(ctx context.Context, rawToken string) (*AuthContext, error)
    Logout(ctx context.Context, auth *AuthContext) error
    RevokeSession(ctx context.Context, sessionID string, reason Status) error
    RevokeUser(ctx context.Context, userID uint64, reason Status) error
    RevokeUserWorkspace(ctx context.Context, userID uint64, workspaceID uint64, reason Status) error
    ExpiresAt() time.Time
}
```

### 登录签发令牌

用户验证凭据后调用 `Issue` 创建会话：

```go
issued, err := s.Auth.Issue(ctx, authsession.IssueRequest{
    UserID:   user.ID,
    Username: user.Username,
    UserType: resp.UserTyp,
    UA:       ua,
    IP:       loginip,
})
if err != nil {
    return nil, err
}
// issued.AccessToken  -> 返回给前端的 JWT
// issued.RefreshToken -> 返回给前端的不透明刷新令牌
// issued.ExpiresAt    -> access token 展示过期时间
// issued.Auth         -> 本次签发的 AuthContext
```

`Issue` 内部流程：

1. 校验请求字段（UserID、Username、UA 不能为空）。
2. 调用 `ContextValidator` 验证（如果已注册，可用于检查用户是否被禁用）。
3. 调用 `prepareSessionSlot` 处理多设备登录策略。
4. 通过 IDGen 生成 sessionID、tokenID 和不透明 refresh token。
5. 使用 HS256 签名 JWT，Claims 包含 uid、username、typ、ua、sid、jti。
6. 保存 `SessionRecord`、`AccessRecord`、`RefreshRecord` 到 JetCache。
7. 在 Redis 中维护用户会话索引和设备会话索引。
8. 触发 `EventRecorder`（如果已注册）记录登录事件。

### 刷新令牌

access token 过期后，前端携带 refresh token 调用刷新接口：

```go
issued, err := s.Auth.Refresh(ctx, authsession.RefreshRequest{
    RefreshToken: refreshToken,
    IP:           loginip,
})
```

Refresh 执行 **令牌轮换（rotation）**：

1. 校验 refresh token 对应的 `RefreshRecord` 存在且状态为 active。
2. 检查 `SessionRecord.CurrentRefreshHash` 是否匹配（防止已轮换的旧 refresh token 被重放）。
3. 旧 access token 标记为 `StatusRotated`，旧 refresh token 也标记为 `StatusRotated`。
4. 签发新 access token 和新 refresh token，更新 `SessionRecord`。
5. 如果旧 refresh token 被重用（replay），整个会话将被撤销（`StatusRefreshReused`）。

::: warning 令牌安全
Refresh token 采用单次使用（one-time-use）设计。使用后立即轮换，旧值作废。如果检测到旧 refresh token 被重用，整个会话将被撤销，所有关联令牌失效。这是防止 refresh token 泄露的安全措施。
:::

### 请求校验（中间件）

gRPC 中间件自动完成令牌提取和校验，业务代码通过 `FromContext` 获取认证信息：

```go
auth, ok := authsession.FromContext(ctx)
if !ok {
    return platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)
}
// auth.UserID      -> 用户 ID
// auth.Username    -> 用户名
// auth.UserType    -> 用户类型
// auth.SessionID   -> 会话 ID
// auth.TokenID     -> 当前 access token ID
// auth.WorkspaceID -> 工作区 ID
```

中间件校验流程：

1. 从 gRPC metadata 的 `authorization` 字段提取 `Bearer <token>`。
2. 解析 JWT Claims，校验签名和过期时间。
3. 从 JetCache 查询 `AccessRecord`，验证状态为 active、未过期、token hash 匹配。
4. 从 JetCache 查询 `SessionRecord`，验证状态为 active、未过期、当前 access ID 匹配。
5. 运行 `ContextValidator`（如果已注册）。
6. 触摸（touch）`SessionRecord.LastActiveAt`（如果距上次触摸超过 `TouchInterval`）。
7. 将 `AuthContext` 注入 context。

### 退出登录

```go
err = s.Auth.Logout(ctx, auth)
```

Logout 调用 `RevokeSession`，将当前会话及其关联的 access token 和 refresh token 标记为 `StatusLogout`。

### 强制下线

管理员可以强制撤销指定用户的所有会话：

```go
// 撤销用户所有会话
err = s.Auth.RevokeUser(ctx, userID, authsession.StatusRevoked)

// 只撤销指定工作区的会话
err = s.Auth.RevokeUserWorkspace(ctx, userID, workspaceID, authsession.StatusRevoked)
```

撤销状态枚举：

| 状态 | 含义 | 触发场景 |
|------|------|----------|
| `StatusActive` | 正常 | 会话存活 |
| `StatusLogout` | 已退出 | 用户主动退出 |
| `StatusExpired` | 已过期 | refresh token 过期 |
| `StatusKicked` | 被踢出 | 多设备登录超限，最旧会话被驱逐 |
| `StatusReplaced` | 被替换 | 同设备重新登录 |
| `StatusRevoked` | 已撤销 | 管理员强制下线 |
| `StatusRotated` | 已轮换 | 令牌刷新后旧令牌标记 |
| `StatusRefreshReused` | 刷新令牌重放 | 检测到 refresh token 重放攻击 |

## 配置参考

### 服务层配置（configs/user/config.toml）

```toml
[app.user]
adminPassword = "123456"               # 初始管理员密码（仅初始化）
jwtExpire = 604800                     # JWT 有效期（秒），7 天
refreshTokenExpire = 2592000           # Refresh token 有效期（秒），30 天
jwtSignKey = "local-egoadmin-jwt-sign-key"  # JWT HS256 签名密钥
useCaptcha = false                     # 是否启用登录验证码
multiLoginEnabled = true               # 是否允许多设备登录
maxLoginClient = 2                     # 最大并发会话数
heartbeatOfflineEnabled = true         # 是否启用心跳离线检测
heartbeatOfflineSeconds = 660          # 心跳超时秒数（11 分钟）
revokeSessionOnHeartbeatOffline = false # 离线时是否撤销会话
```

服务层将这些配置转换为组件层 `Config`：

```go
// 内部转换逻辑
jwtExpire (秒)             -> AccessTokenTTL (Duration)
refreshTokenExpire (秒)    -> RefreshTokenTTL (Duration)
multiLoginEnabled          -> MultiLoginEnabled
maxLoginClient             -> MaxSessions
```

### 组件层配置（component.authsession）

直接配置组件时使用 Duration 格式：

```toml
[component.authsession]
name = "default"
keyPrefix = ""
jwtSignKey = "local-egoadmin-jwt-sign-key"
accessTokenTTL = "7h"                  # access token 有效期
accessTokenDisplaySkew = "30m"         # 返回给前端的过期时间提前量
refreshTokenTTL = "720h"               # refresh token 有效期（30 天）
revokedRecordTTL = "24h"               # 已撤销记录保留时间
touchInterval = "1m"                   # SessionRecord 触摸间隔
multiLoginEnabled = true               # 多设备登录开关
maxSessions = 2                        # 最大并发会话数（0 = 不限制）
sameDeviceStrategy = "replace"         # 同设备策略：replace / reject / allow
overflowStrategy = "revoke_oldest"     # 超限策略：revoke_oldest / reject
```

### 配置字段详解

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `name` | `"default"` | 组件实例名称 |
| `keyPrefix` | `""` | Redis key 前缀，多实例共享 Redis 时用于隔离 |
| `jwtSignKey` | `""`（必填） | HS256 签名密钥，**不可为空** |
| `accessTokenTTL` | `"2h"` | JWT 有效期 |
| `accessTokenDisplaySkew` | `"30m"` | `ExpiresAt()` 返回值比实际过期时间提前的量，给前端预留刷新窗口 |
| `refreshTokenTTL` | `"720h"` | Refresh token 有效期，同时也是会话的最大生命周期 |
| `revokedRecordTTL` | `"24h"` | 已撤销记录在缓存中的保留时间，用于返回精确的撤销原因 |
| `touchInterval` | `"1m"` | `LastActiveAt` 更新的最小间隔，避免高频写入 |
| `multiLoginEnabled` | `true` | `false` 时每次登录撤销用户所有旧会话 |
| `maxSessions` | `0` | 最大并发会话数，`0` 表示不限制 |
| `sameDeviceStrategy` | `"replace"` | 同一设备（相同 UA hash）的会话处理策略 |
| `overflowStrategy` | `"revoke_oldest"` | 超过 `maxSessions` 时的处理策略 |

### 同设备策略（SameDeviceStrategy）

| 策略 | 行为 |
|------|------|
| `replace`（默认） | 同设备重新登录时撤销旧会话 |
| `reject` | 同设备已有会话时拒绝登录，返回 `ErrSessionExists` |
| `allow` | 不做同设备检查，允许同设备多会话 |

### 超限策略（OverflowStrategy）

| 策略 | 行为 |
|------|------|
| `revoke_oldest`（默认） | 超过 `maxSessions` 时撤销最旧的会话 |
| `reject` | 超过 `maxSessions` 时拒绝登录，返回 `ErrTooManySessions` |

## 实际场景

### 场景一：标准登录流程

```text
1. 前端调用 GetLoginCrypto 获取 RSA 公钥和 challenge
2. 前端用 Web Crypto RSA-OAEP/SHA-256 加密密码
3. 前端调用 Login 提交加密后的凭据
4. 后端验证凭据 → Issue() 创建会话
5. 返回 AccessToken + RefreshToken + ExpiresAt
6. 前端存储令牌，后续请求在 metadata 中携带 Bearer token
```

### 场景二：令牌自动刷新

```text
1. 前端检测到 response 中 LoginExpired 错误码
2. 使用 RefreshToken 调用 Refresh 接口
3. 后端执行令牌轮换，返回新的令牌对
4. 前端更新存储的令牌，重试原请求
5. 如果 Refresh 也失败（refresh token 过期），跳转登录页
```

::: tip 提示
`ExpiresAt()` 返回值比 JWT 实际过期时间提前 `accessTokenDisplaySkew`（默认 30 分钟）。前端可以在此时间点主动刷新，避免用户在操作中途遇到令牌过期。
:::

### 场景三：多设备登录控制

```text
配置：multiLoginEnabled = true, maxLoginClient = 2

用户在设备 A 登录 → 创建会话 1
用户在设备 B 登录 → 创建会话 2（未超限）
用户在设备 C 登录 → 撤销会话 1（最旧），创建会话 3

配置：multiLoginEnabled = false

用户在设备 B 登录 → 撤销所有旧会话，创建新会话
```

### 场景四：心跳离线检测

```text
1. 前端定期（例如每 5 分钟）发送心跳请求
2. 后端更新用户在线状态和心跳时间戳
3. 定时任务扫描心跳超时的用户
4. 超过 heartbeatOfflineSeconds（默认 660 秒）未收到心跳
5. 标记用户离线
6. 如果 revokeSessionOnHeartbeatOffline = true，同时撤销会话
```

心跳接口调用示例：

```go
// 前端发送心跳
err = s.userUseCase.MarkUserOnline(ctx, auth.UserID)
```

定时任务检查超时：

```go
// Cron 定时任务
err = s.UserUseCase.OfflineExpiredUsers(ctx, application.OfflineExpiredCommand{
    Enabled:       conf.HeartbeatOfflineEnabled,
    Seconds:       conf.HeartbeatOfflineSeconds,
    RevokeSession: conf.RevokeSessionOnHeartbeatOffline,
})
```

### 场景五：管理员强制下线

```text
1. 管理员在后台选择目标用户
2. 调用 RevokeUser（或 RevokeUserWorkspace）
3. 组件遍历用户所有活跃会话，逐一撤销
4. 撤销后客户端下次请求将收到 NotLogin 错误
5. 前端提示"您的账号已在其他地方登录"或"已被管理员下线"
```

### 场景六：API 权限分类

gateway 对 API 进行三类分类，AuthSession 中间件按分类应用：

| 分类 | 认证 | Casbin | 典型接口 |
|------|------|--------|----------|
| public | 不需要 | 不需要 | Login、GetCaptcha、GetLoginCrypto |
| login-only | 需要 | 不需要 | GetMenus、Logout、HeartBeatUser、GetProfile |
| protected | 需要 | 需要 | 用户管理、角色管理、部门管理等 |

```go
// public 接口：不经过 AuthSession 中间件
// login-only 接口：经过 AuthSession 中间件，跳过 Casbin
// protected 接口：经过 AuthSession 中间件 + Casbin 校验
```

## 工作原理

### 令牌结构

Access token 是标准 JWT（HS256），Claims 包含：

```go
type Claims struct {
    UID         uint64 `json:"uid"`          // 用户 ID
    Username    string `json:"username"`      // 用户名
    UserType    int32  `json:"typ"`           // 用户类型
    UA          string `json:"ua"`            // User-Agent
    SessionID   string `json:"sid"`           // 会话 ID
    TokenID     string `json:"jti"`           // 令牌 ID
    WorkspaceID uint64 `json:"workspace_id"`  // 工作区 ID
    jwtv5.RegisteredClaims                     // 标准字段：iss, sub, exp, nbf, iat
}
```

::: warning 令牌不含权限
JWT Claims 只存储标识符。角色、菜单和权限数据保持在服务端，通过 Casbin 和 DataScope 在请求时查询和校验。这避免了令牌过长和权限变更不及时的问题。
:::

Refresh token 是不透明令牌（opaque token），由 IDGen 生成，服务端只存储其哈希值。

### 会话存储模型

```text
SessionRecord (JetCache)
  ├── ID, UserID, Username, UserType, UA, DeviceHash, IP, WorkspaceID
  ├── CurrentAccessID     -> 指向当前活跃的 AccessRecord
  ├── CurrentRefreshHash  -> 指向当前活跃的 RefreshRecord
  ├── Status, LoginAt, LastActiveAt, ExpiresAt
  └── RevokedAt, RevokeReason

AccessRecord (JetCache)
  ├── ID (= JWT jti), SessionID, UserID, UA
  ├── TokenHash           -> access token 的哈希
  └── Status, IssuedAt, ExpiresAt, RevokedAt, RevokeReason

RefreshRecord (JetCache)
  ├── Hash                -> refresh token 的哈希
  ├── SessionID, AccessID, UserID
  ├── Status, IssuedAt, ExpiresAt
  └── RotatedAt, NextTokenHash  -> 轮换后指向新 refresh hash
```

Redis 索引：

```text
user_sessions:{userID}   -> Sorted Set，member=sessionId, score=loginTimeMillis
device_session:{userID}:{deviceHash} -> String，value=sessionId
```

### 请求校验全流程

```text
gRPC 请求到达
  │
  ├─ metadata.ExtractIncoming(ctx).Get("authorization")
  │
  ├─ extractBearerToken() -> "Bearer xxx" 中提取 rawToken
  │
  ├─ parseAccessToken() -> JWT 解析 + HS256 签名校验
  │
  ├─ getAccess(tokenID) -> JetCache 查询 AccessRecord
  │   ├─ status != active -> ErrTokenRevoked
  │   ├─ expired -> ErrTokenExpired
  │   ├─ hash mismatch -> errTokenHashMismatch
  │   └─ ok
  │
  ├─ getSession(sessionID) -> JetCache 查询 SessionRecord
  │   ├─ status != active -> ErrSessionRevoked
  │   ├─ expired -> ErrSessionExpired
  │   ├─ currentAccessID != tokenID -> ErrInvalidToken
  │   └─ ok
  │
  ├─ ContextValidator.ValidateAuthContext()（可选）
  │
  ├─ touch LastActiveAt（如果超过 TouchInterval）
  │
  ├─ NewContext(ctx, auth) -> AuthContext 注入 context
  │
  └─ handler(ctx, req) -> 业务逻辑通过 FromContext(ctx) 获取认证信息
```

### 错误映射

组件内部错误自动映射为 API 错误码：

| 内部错误 | API 错误 | 前端提示 |
|----------|----------|----------|
| `ErrMissingToken` | `Unauthenticated` | 未提供认证令牌 |
| `ErrTokenExpired` / `ErrSessionExpired` / `ErrRefreshExpired` | `LoginExpired` | 登录已过期 |
| `ErrInvalidToken` / `ErrTokenRevoked` / `ErrSessionRevoked` | `NotLogin` | 未登录或令牌无效 |
| `ErrRefreshReused` | `NotLogin` + `LoginAbnormal` | 登录异常 |
| `StatusLogout` | `NotLogin` + `LoggedOut` | 已退出登录 |
| `StatusKicked` | `NotLogin` + `ForcedOffline` | 被强制下线 |
| `StatusReplaced` | `NotLogin` + `LoginReplaced` | 被新设备登录替换 |

前端可根据 `auth_status` metadata 判断具体原因并做针对性处理。

## 常见问题

### 令牌过期但未自动刷新

**现象**：用户操作过程中频繁提示"登录已过期"。

**排查**：
- 检查 `jwtExpire` 配置值是否过短（默认 604800 秒 = 7 天）。
- 确认前端是否实现了 `LoginExpired` 错误码拦截和自动刷新逻辑。
- 检查前端是否在 `ExpiresAt` 时间点主动刷新（而非等到实际过期）。
- 查看 `accessTokenDisplaySkew` 配置，确保前端有足够的刷新窗口。

### 角色变更后权限未生效

**现象**：管理员修改了用户角色，但用户仍看到旧权限。

**排查**：
- Casbin 策略有缓存，需要清除缓存或等待缓存过期。
- 如果 DataScope 权限快照使用了 JetCache，也需要清除对应缓存。
- 最快的解决方式是让用户重新登录（触发新的权限加载）。

### 用户退出后仍显示在线

**现象**：用户已退出，但后台仍显示在线状态。

**排查**：
- 确认心跳离线检测的 Cron 定时任务是否正在运行。
- 检查 `heartbeatOfflineEnabled` 是否为 `true`。
- 确认 Redis 连接正常（心跳状态存储在 Redis 中）。
- 检查 `heartbeatOfflineSeconds` 设置是否合理（默认 660 秒 = 11 分钟）。

### 强制下线未生效

**现象**：调用 `RevokeUser` 后用户仍然可以正常请求。

**排查**：
- 确认 `RevokeUser` 的 `reason` 参数传入了正确状态（如 `StatusRevoked`）。
- 检查 JetCache 是否正常工作，撤销操作需要更新缓存中的记录状态。
- 用户的下一次请求才会校验到撤销状态（已发出的请求不受影响）。
- 检查是否有多个 AuthSession 实例共享同一 Redis 但 keyPrefix 不同。

### 多设备登录超限未踢出旧会话

**现象**：用户登录设备数超过 `maxLoginClient`，但旧设备未被强制下线。

**排查**：
- 确认 `multiLoginEnabled` 为 `true`。
- 检查 `overflowStrategy` 是否为 `revoke_oldest`（而非 `reject`）。
- 旧设备只有在下次请求时才会感知到会话被撤销。
- 检查 `maxSessions` 的值是否正确（注意：配置字段名是 `maxSessions`，不是 `maxLoginClient`）。

### Refresh token 被判定为重放

**现象**：刷新时报 `LoginAbnormal` 错误。

**原因**：Refresh token 采用单次使用设计。以下情况会触发此错误：
- 前端并发发送了多次刷新请求。
- 前端使用了已刷新过的旧 refresh token。
- 网络重试导致同一 refresh token 被使用两次。

**解决**：
- 前端实现刷新请求的互斥锁（上一个刷新完成前不发送新的）。
- 每次刷新成功后立即更新存储的 refresh token。

## 参考链接

- 源码：`internal/component/authsession/`
- 组件系统概览：[组件系统](/guide/components)
- 权限系统：[权限系统](/guide/permission-system)
- 运行时配置：[运行时配置](/guide/configuration)
- 优雅停机：[优雅停机与生命周期](/guide/graceful-shutdown)
- IDGen 组件：[IDGen ID 生成](/guide/components/idgen)
- LoginCrypto 组件：[LoginCrypto 登录加密](/guide/components/login-crypto)
- ERedis 组件：[ERedis 缓存](/guide/components/eredis)
