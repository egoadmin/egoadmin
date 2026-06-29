# 组件系统

EgoAdmin 把可复用能力放在 `internal/component` 和 `internal/platform`。业务服务通过 Wire 注入使用组件，不直接在业务逻辑中重复创建底层客户端。

## 组件目录

```text
internal/component/
├── authsession/     # 认证会话
├── logincrypto/     # 登录加密挑战
├── idgen/           # ID 生成和编解码
├── eredis/          # Redis 组件封装
├── etusupload/      # TUS 文件上传
└── ...
```

```text
internal/platform/
├── database/mysql/  # GORM、事务、迁移
├── i18n/            # 国际化
├── shutdown/        # 优雅停机
└── ...
```

## 组件生命周期

组件实现统一的生命周期接口，通过 `shutdown.Manager` 注册清理逻辑：

```go
// internal/platform/shutdown/manager.go
type Closer interface {
  Close() error
}
```

典型组件生命周期：

```text
Server 启动
  -> Wire 依赖注入，构造组件
  -> 组件初始化（连接池、连接、预热缓存等）
  -> 注册到 shutdown.Manager
  -> 服务开始接收请求

Server 收到 SIGTERM/SIGINT
  -> shutdown.Manager 按 LIFO 顺序执行 Close
  -> 组件释放资源（关闭连接、刷新缓冲区等）
  -> 进程退出
```

组件 Close 实现示例：

```go
// internal/component/eredis/eredis.go
type ERedis struct {
  client redis.UniversalClient
}

func (r *ERedis) Close() error {
  return r.client.Close()
}
```

::: tip
组件的 `Close()` 方法由 `shutdown.Manager` 在进程退出时自动调用。业务代码不需要手动关闭组件。
:::

## Wire Provider Set 模式

每个组件通过 Wire provider set 暴露构造函数，业务服务在 Wire 注入时引用：

```go
// internal/component/authsession/wire.go
package authsession

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
  NewAuthSession,
  NewAuthMiddleware,
)

// internal/component/eredis/wire.go
package eredis

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
  NewERedis,
)

// internal/component/idgen/wire.go
package idgen

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
  NewIDGenerator,
  NewIDCodec,
)
```

服务的 Wire 注入文件引用这些 provider set：

```go
// internal/app/user/wire.go
//go:build wireinject

package user

import (
  "github.com/google/wire"
  "github.com/egoadmin/egoadmin/internal/component/authsession"
  "github.com/egoadmin/egoadmin/internal/component/eredis"
  "github.com/egoadmin/egoadmin/internal/component/idgen"
  "github.com/egoadmin/egoadmin/internal/component/logincrypto"
)

func InitializeServer() (*Server, error) {
  wire.Build(
    authsession.ProviderSet,
    eredis.ProviderSet,
    idgen.ProviderSet,
    logincrypto.ProviderSet,
    // ... 业务绑定
    NewServer,
  )
  return nil, nil
}
```

::: warning
不要编辑 `wire_gen.go`，该文件由 `make gen` 自动生成。
:::

## AuthSession

职责：

- access token / refresh token。
- JWT 校验。
- 多端登录控制。
- 心跳离线。
- forced offline。
- AuthContext 注入。

配置：

```toml
[app.user]
adminPassword = "123456"
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
useCaptcha = false
multiLoginEnabled = true
maxLoginClient = 2
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
revokeSessionOnHeartbeatOffline = false
```

### AuthContext 注入

AuthSession 中间件在每个请求中注入 AuthContext：

```go
type AuthContext struct {
  UserID    uint64
  Username  string
  IsAdmin   bool
  IsRoot    bool
  DataScope DataScopeType
  DeptID    uint64
  DeptIDs   []uint64
  RoleIDs   []uint64
}
```

业务层通过 `permission.FromContext(ctx)` 获取当前用户信息。

### 多端登录控制

| 配置项 | 说明 |
|--------|------|
| `multiLoginEnabled` | 是否允许多端同时登录 |
| `maxLoginClient` | 最大同时登录设备数 |
| `revokeSessionOnHeartbeatOffline` | 心跳离线时是否撤销会话 |

多端登录模式下，新设备登录时如果超过 `maxLoginClient`，最早的会话会被踢出。

## LoginCrypto

用于密码安全传输。前端提交密码前必须调用 `GetLoginCrypto`，然后使用 Web Crypto RSA-OAEP/SHA-256 加密。

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
rsaKeyBits = 4096
enableMetrics = true
```

### 流程详解

```text
前端                                    后端
  |                                      |
  |-- GetLoginCrypto(username, ua) ----> |
  |                                      |-- 生成 RSA 密钥对
  |                                      |-- 生成 challengeId + nonce
  |                                      |-- 存入 Redis，TTL = challengeTTL
  | <------ 返回 publicKey, keyId,       |
  |         challengeId, nonce           |
  |                                      |
  |-- Web Crypto 加密密码 ------------> |
  |   RSA-OAEP + SHA-256                 |
  |                                      |
  |-- Login(passwordCipher, keyId,      |
  |    challengeId, nonce) ----------->  |
  |                                      |-- 校验 challenge 存在且未过期
  |                                      |-- 校验时间戳在 timestampSkew 内
  |                                      |-- 用私钥解密密码
  |                                      |-- 校验密码哈希
  | <------ 返回 accessToken,            |
  |         refreshToken                 |
```

::: warning
`challengeTTL` 不要设置过长，避免 challenge 被重放攻击。生产建议 1-3 分钟。
:::

## IDGen

组件职责：

- 雪花 ID。
- 号段生成。
- 机器租约。
- 命名空间隔离。
- 稳定 ID 编解码。

配置：

```toml
[component.idgen.default]
namespace = "egoadmin-local"
name = "default"

[component.idgen.machine]
group = "egoadmin-local"

[component.idgen.codec]
secret = "local-stable-idcodec-secret"
```

### IDGen 使用

```go
// 注入 IDGen
type UserService struct {
  idGen *idgen.IDGenerator
}

// 生成唯一 ID
func (s *UserService) CreateUser(ctx context.Context, cmd CreateUserCommand) error {
  userID := s.idGen.NextID()
  // userID 是全局唯一雪花 ID
  user := &User{ID: userID, Username: cmd.Username}
  return s.repo.Create(ctx, user)
}
```

### IDCodec 稳定 ID 编解码

IDCodec 将数字 ID 编码为稳定的短字符串，用于对外暴露的 API：

```go
// 编码：数字 ID -> 短字符串
encoded := idCodec.Encode(123456789) // 例如 "Kx7fM2"

// 解码：短字符串 -> 数字 ID
decoded, err := idCodec.Decode("Kx7fM2") // 返回 123456789
```

::: warning
`idcodec` 只是可逆 public ID 编码，不是授权机制。不要用编码后的 ID 代替权限检查。
:::

## Redis / JetCache

### ERedis 模式

ERedis 支持多种 Redis 部署模式：

| 模式 | mode 值 | 场景 |
|------|---------|------|
| 单机 | `stub` | 本地开发、单节点部署 |
| 主从 | `master-slave` | 读写分离场景 |
| 哨兵 | `sentinel` | 高可用自动故障转移 |
| 集群 | `cluster` | 大规模水平扩展 |

Redis 配置：

```toml
[client.redis]
addr = "127.0.0.1:6380"
debug = true
mode = "stub"
password = "egoadmin"
db = 0
poolSize = 10
minIdleConns = 5
```

ERedis 使用示例：

```go
type UserRepository struct {
  redis *eredis.ERedis
}

func (r *UserRepository) CacheUser(ctx context.Context, user *User) error {
  key := fmt.Sprintf("user:%d", user.ID)
  return r.redis.Set(ctx, key, user, 10*time.Minute)
}

func (r *UserRepository) GetCachedUser(ctx context.Context, id uint64) (*User, error) {
  key := fmt.Sprintf("user:%d", id)
  var user User
  if err := r.redis.Get(ctx, key, &user); err != nil {
    return nil, err
  }
  return &user, nil
}
```

### JetCache

JetCache 是多级缓存框架，支持本地缓存 + Redis 远程缓存：

```toml
[client.jetcache]
name = "default"
remoteExpiry = "1h0m0s"
localSize = 256
localExpiry = "1m0s"
refreshDuration = "2m0s"
stopRefreshAfter = "1h0m0s"
notFoundExpiry = "1m0s"
enableMetrics = true
enableSyncLocal = false
codec = "msgpack"
```

JetCache 配置说明：

| 配置项 | 说明 |
|--------|------|
| `remoteExpiry` | Redis 远程缓存过期时间 |
| `localSize` | 本地缓存最大条目数 |
| `localExpiry` | 本地缓存过期时间 |
| `refreshDuration` | 后台刷新间隔 |
| `stopRefreshAfter` | 无人访问后停止刷新的时间 |
| `notFoundExpiry` | 缓存穿透保护，空值缓存时间 |
| `codec` | 序列化方式，`msgpack` 或 `json` |

适用场景：

- 权限快照。
- 用户基本信息。
- 高频读且允许短时间缓存的数据。
- 部门树结构。

```go
type PermissionCache struct {
  cache *jetcache.Cache
}

func (c *PermissionCache) GetUserMenus(ctx context.Context, userID uint64) ([]*Menu, error) {
  key := fmt.Sprintf("user_menus:%d", userID)
  var menus []*Menu
  if err := c.cache.Get(ctx, key, &menus); err != nil {
    return nil, err
  }
  return menus, nil
}

func (c *PermissionCache) InvalidateUserMenus(ctx context.Context, userID uint64) error {
  key := fmt.Sprintf("user_menus:%d", userID)
  return c.cache.Del(ctx, key)
}
```

## AsyncQ

异步任务队列，基于 Redis List 实现，支持优先级队列和重试：

```toml
[client.asyncq]
redisAddr = "127.0.0.1:6380"
redisPassword = "egoadmin"
redisDB = 0
enableClient = true
enableServer = false
concurrency = 10
queues = { critical = 6, default = 3, low = 1 }
maxRetry = 3
retryDelayFunc = "exponential"
taskTimeout = "30s"
```

AsyncQ 配置说明：

| 配置项 | 说明 |
|--------|------|
| `enableClient` | 启用任务投递端 |
| `enableServer` | 启用任务消费端 |
| `concurrency` | 每个队列的并发 worker 数 |
| `queues` | 队列名称及优先级权重 |
| `maxRetry` | 最大重试次数 |
| `retryDelayFunc` | 重试间隔策略，`exponential` 指数退避 |
| `taskTimeout` | 单个任务执行超时 |

### 任务投递与消费

```go
// 投递任务
type AuditNotifier struct {
  asyncQ *asyncq.Client
}

func (n *AuditNotifier) NotifyAudit(ctx context.Context, event *AuditEvent) error {
  return n.asyncQ.Enqueue(ctx, &asyncq.Task{
    Queue: "default",
    Type:  "audit.notify",
    Payload: event,
  })
}

// 消费任务
type AuditHandler struct{}

func (h *AuditHandler) Handle(ctx context.Context, task *asyncq.Task) error {
  event := task.Payload.(*AuditEvent)
  // 发送通知、写入审计日志等
  return nil
}
```

适合：

- 审计日志扩展。
- 发送通知。
- 低优先级后台处理。
- 可重试任务。

::: tip
本地开发时 `enableServer = false`，只投递不消费。生产环境需要在至少一个服务实例上启用 `enableServer = true`。
:::

## ETUSUpload

TUS 断点续传组件，gateway 暴露上传入口，MinIO 作为对象存储。

MinIO 配置：

```toml
[client.minio]
endpoint = "127.0.0.1:9000"
accessKeyID = "egoadmin"
secretAccessKey = "egoadmin123"
ssl = false
```

## CDN / Image Processor

```toml
[component.cdn]
signSecret = "local-cdn-sign-secret"

[client.imageProcessor]
url = "http://127.0.0.1:2853"
secret = "local-image-processor-secret"
timeout = "5s"
```

用于文件访问签名、图片裁剪和格式转换。

## 详细组件文档

| 组件 | 说明 | 文档 |
|------|------|------|
| AuthSession | 认证会话、JWT、多端登录、心跳 | [权限系统](/guide/zh-CN/permission-system) |
| LoginCrypto | RSA 挑战密码加密 | [权限系统 - 登录加密](/guide/zh-CN/permission-system#登录加密) |
| IDGen | 雪花 ID、号段、IDCodec | [IDGen 详细文档](/guide/zh-CN/idgen) |
| ERedis | Redis 封装 | [Redis 使用指南](/guide/zh-CN/redis) |
| JetCache | 多级缓存 | [缓存策略](/guide/zh-CN/cache) |
| AsyncQ | 异步任务队列 | [异步任务](/guide/zh-CN/async-tasks) |
| ETUSUpload | TUS 断点续传 | [文件上传](/guide/zh-CN/file-upload) |
| Database | GORM、事务、迁移 | [数据库与迁移](/guide/zh-CN/database-migration) |

## 组件开发原则

- 组件只提供通用能力，不写服务业务规则。
- 配置结构保持最小。
- 组件需要可测试，避免启动真实外部服务才能测试核心逻辑。
- 资源关闭注册到 `shutdown.Manager`。
- 业务层通过接口依赖组件能力，避免强耦合。

