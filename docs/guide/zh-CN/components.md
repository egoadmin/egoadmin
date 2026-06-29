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
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true
maxLoginClient = 2
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

## LoginCrypto

用于密码安全传输：

```toml
[component.logincrypto]
challengeTTL = "3m0s"
timestampSkew = "2m0s"
rsaKeyBits = 4096
enableMetrics = true
```

前端提交密码前必须调用 `GetLoginCrypto`，然后使用 Web Crypto RSA-OAEP/SHA-256 加密。

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

::: warning
`idcodec` 只是可逆 public ID 编码，不是授权机制。不要用编码后的 ID 代替权限检查。
:::

## Redis / JetCache

Redis 配置：

```toml
[client.redis]
addr = "127.0.0.1:6380"
debug = true
mode = "stub"
password = "egoadmin"
```

JetCache 配置：

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

适用场景：

- 权限快照。
- 用户基本信息。
- 高频读且允许短时间缓存的数据。

## AsyncQ

异步任务配置：

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

适合：

- 审计扩展。
- 发送通知。
- 低优先级后台处理。
- 可重试任务。

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

## 组件开发原则

- 组件只提供通用能力，不写服务业务规则。
- 配置结构保持最小。
- 组件需要可测试，避免启动真实外部服务才能测试核心逻辑。
- 资源关闭注册到 `shutdown.Manager`。
- 业务层通过接口依赖组件能力，避免强耦合。

