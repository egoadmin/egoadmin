# 认证与会话安全

EgoAdmin 使用 JWT 双 token 机制实现分布式会话管理，结合 Redis 存储支持多端登录控制和强制下线。

## 概述

认证体系的核心是 `authsession` 组件，位于 `internal/component/authsession`。它负责签发、刷新、撤销 JWT token，并在 gateway 层通过 gRPC metadata 提取 Bearer token 完成身份校验。

```text
客户端请求
  -> gateway gRPC 拦截器提取 Authorization: Bearer <token>
  -> authsession 校验 token 签名和有效期
  -> Redis 查询会话状态（在线/离线/已撤销）
  -> API 分类路由：public / login-only / protected
  -> protected API 进入 Casbin 权限校验
```

## 核心用法

### Token 生命周期

系统签发两种 token：

| Token | 用途 | 默认有效期 |
|-------|------|-----------|
| access token | API 请求认证 | 7 天（604800 秒） |
| refresh token | 刷新 access token | 30 天（2592000 秒） |

签发流程：

```text
1. 用户登录成功
2. 生成 access token（JWT HS256 签名）
3. 生成 refresh token（随机 UUID，存入 Redis）
4. 返回 token pair 给客户端
```

### 刷新轮换

refresh token 使用后立即失效，防止重放攻击：

```text
客户端携带 refresh token 请求刷新
  -> 校验 refresh token 是否存在于 Redis
  -> 存在则删除旧 refresh token
  -> 签发新的 access + refresh token pair
  -> 返回新 token pair
```

::: warning
refresh token 是一次性令牌。刷新后旧 refresh token 不可再用。如果 refresh token 被盗用，原持有者刷新时会失败，从而暴露异常。
:::

### JWT 签名

所有 JWT 使用 HS256 算法签名，签名密钥通过 `jwtSignKey` 配置：

```toml
[app.user]
jwtSignKey = "CHANGE-ME-IN-PRODUCTION"
```

::: danger
生产环境必须更换 `jwtSignKey`。使用弱密钥或默认密钥会导致 token 被伪造。密钥长度建议至少 32 字节。
:::

### 多端登录控制

通过配置控制同一账号的并发登录设备数：

```toml
[app.user]
multiLoginEnabled = true
maxLoginClient = 2
```

行为说明：

```text
multiLoginEnabled = true, maxLoginClient = 2
  -> 允许同一账号在最多 2 个设备同时在线
  -> 第 3 个设备登录时，最早登录的设备会被强制下线

multiLoginEnabled = false
  -> 不限制登录设备数
  -> 每次登录都生成新的 session
```

实现原理是 Redis 中为每个用户维护一个 session 集合，登录时检查集合大小：

```go
// internal/component/authsession/session.go
func (s *SessionManager) Login(ctx context.Context, userID int64, clientInfo string) (*TokenPair, error) {
    if s.config.MultiLoginEnabled {
        count, err := s.rdb.SCard(ctx, userSessionKey(userID)).Result()
        if err != nil {
            return nil, err
        }
        if count >= int64(s.config.MaxLoginClient) {
            // 移除最早的 session
            s.evictOldestSession(ctx, userID)
        }
    }
    // 签发新 token pair 并存入 session 集合
}
```

### 心跳离线检测

客户端定期发送心跳，服务端记录最后活跃时间：

```toml
[app.user]
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

心跳机制：

```text
客户端每 N 秒发送心跳请求
  -> 服务端更新 Redis 中 last_active_at 时间戳
  -> 超过 heartbeatOfflineSeconds 未收到心跳
  -> 标记用户为离线状态
```

660 秒（11 分钟）的默认值覆盖了网络波动和客户端休眠的场景。

### 强制下线

管理员可以通过 `RevokeUser` 接口强制踢出指定用户：

```go
// 内部调用示例
err := sessionManager.RevokeUser(ctx, targetUserID)
if err != nil {
    // 处理错误
}
```

强制下线会撤销该用户的所有活跃 session，客户端下次请求时 token 校验失败，自动跳转到登录页。

### Token 撤销

用户主动退出登录时，同时撤销 access token 和 refresh token：

```text
客户端请求退出登录
  -> 从 Bearer token 提取 session ID
  -> 删除 Redis 中的 access token 映射
  -> 删除 Redis 中的 refresh token
  -> 从用户 session 集合中移除
```

### 认证中间件

gateway 层的 gRPC 拦截器完成 token 提取和校验：

```go
// internal/app/gateway/server/interceptor.go
func AuthInterceptor(session *authsession.SessionManager) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        tokens := md.Get("authorization")
        if len(tokens) == 0 {
            return nil, status.Error(codes.Unauthenticated, "missing token")
        }

        userID, err := session.Validate(ctx, tokens[0])
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token")
        }

        ctx = context.WithValue(ctx, ctxKeyUserID, userID)
        return handler(ctx, req)
    }
}
```

### 三层 API 分类

所有 API 按照认证和权限要求分为三层：

```text
openPack（公开）
  -> 无需登录，无需权限校验
  -> Login、GetLoginCrypto、Captcha

justLoginPack（登录即可）
  -> 需要登录，无需 Casbin 校验
  -> GetMenus、Logout、Heartbeat、GetProfile

protected（受保护）
  -> 需要登录 + Casbin 权限校验
  -> 用户管理、角色管理、部门管理等
```

分类在 gateway 启动时注册：

```go
// internal/app/gateway/server/server.go
openPack := map[string]bool{
    "/user.v1.UserService/Login":          true,
    "/user.v1.UserService/GetLoginCrypto": true,
}

justLoginPack := map[string]bool{
    "/user.v1.UserService/GetMenus":  true,
    "/user.v1.UserService/Logout":   true,
    "/user.v1.UserService/Heartbeat": true,
}
```

未在上述两个集合中的 API 默认为 protected，需要 Casbin 校验。

### 会话存储

所有会话数据存储在 Redis 中，支持分布式部署：

```text
Redis Key 结构：
  session:{session_id}     -> {user_id, client_info, created_at, last_active}
  user_sessions:{user_id}  -> Set<session_id>
  refresh:{refresh_token}  -> session_id
```

## 配置示例

### 完整认证配置

```toml
[app.user]
# JWT access token 有效期（秒）
jwtExpire = 604800
# JWT refresh token 有效期（秒）
refreshTokenExpire = 2592000
# JWT 签名密钥（生产环境必须更换）
jwtSignKey = "CHANGE-ME-IN-PRODUCTION"
# 是否允许多端登录
multiLoginEnabled = true
# 最大并发登录设备数
maxLoginClient = 2
# 是否启用心跳离线检测
heartbeatOfflineEnabled = true
# 心跳超时时间（秒）
heartbeatOfflineSeconds = 660
# 心跳离线时是否撤销会话
revokeSessionOnHeartbeatOffline = false
```

### 生产环境覆盖

```bash
# 通过环境变量覆盖敏感配置
export EGOADMIN_USER_JWTSIGNKEY="your-strong-random-key-at-least-32-bytes"
export EGOADMIN_USER_MAXLOGINCLIENT=5
```

## 实战场景

### 场景一：单设备登录

限制用户只能在一台设备登录：

```toml
[app.user]
multiLoginEnabled = true
maxLoginClient = 1
```

用户在新设备登录时，旧设备的 session 被自动清除。

### 场景二：生产环境强制安全

```toml
[app.user]
jwtExpire = 3600           # 1 小时，缩短 token 有效期
refreshTokenExpire = 86400 # 1 天
multiLoginEnabled = true
maxLoginClient = 3
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 300  # 5 分钟无心跳即离线
```

### 场景三：管理员踢出问题用户

```go
// 在 controller 中调用
func (c *UserController) RevokeUser(ctx context.Context, req *pb.RevokeUserRequest) (*pb.RevokeUserResponse, error) {
    // 权限校验已完成（Casbin）
    err := c.sessionMgr.RevokeUser(ctx, req.UserId)
    if err != nil {
        return nil, err
    }
    return &pb.RevokeUserResponse{}, nil
}
```

## 工作原理

### Token 校验流程

```text
1. 从 gRPC metadata 提取 "authorization" header
2. 去掉 "Bearer " 前缀，获取 token 字符串
3. 使用 jwtSignKey 验证 HS256 签名
4. 解析 claims，提取 user_id 和 session_id
5. 查询 Redis 确认 session 未被撤销
6. 查询 Redis 确认 token 未在黑名单中
7. 校验通过，将 user_id 写入 context
```

### Refresh 流程

```text
1. 客户端 access token 过期（401 响应）
2. 客户端携带 refresh token 请求 /RefreshToken
3. 服务端在 Redis 中查找 refresh token 对应的 session
4. 删除旧 refresh token 和旧 access token
5. 签发新的 token pair
6. 更新 Redis session 映射
7. 返回新 token pair
```

## 常见问题

### Token 过期后如何处理？

客户端收到 401 响应后，应使用 refresh token 请求新的 token pair。如果 refresh token 也已过期，则需要重新登录。

### 多端登录限制是否立即生效？

是的。新的登录请求超过 `maxLoginClient` 限制时，最早的 session 会被立即清除，旧设备下次请求时会收到认证失败。

### 心跳频率建议是多少？

建议客户端心跳间隔为 `heartbeatOfflineSeconds / 3`。默认配置下，建议每 220 秒发送一次心跳，确保在超时前至少有 2 次心跳机会。

### Redis 不可用时怎么办？

Redis 不可用会导致所有认证功能失效。生产环境应使用 Redis Sentinel 或 Redis Cluster 保证高可用。建议在 gateway 层配置健康检查，Redis 不可用时返回 503。

### 如何调试 JWT 问题？

```bash
# 使用 jwt.io 解码 token 查看 claims
# 或使用 Go 命令行工具
go run ./cmd/user jwt decode <token>
```

## 参考链接

- [权限系统](/guide/zh-CN/permission-system) -- 权限体系全链路说明
- [运行时配置](/guide/zh-CN/configuration) -- 配置文件和环境变量说明
- [密码安全](/guide/zh-CN/security/password-security) -- 密码传输和存储安全
- `internal/component/authsession` -- 认证会话组件源码
- `configs/user/config.toml` -- user 服务默认配置
