# ERedis 缓存组件

ERedis 将 go-redis 封装为 EGO 组件，支持 Cluster、Standalone (Stub)、Sentinel 和 Ring 四种模式，提供统一的 `Cmdable` 接口、分布式锁客户端以及 debug/trace 拦截器。

## 概述

EgoAdmin 中所有 Redis 访问都通过 `internal/component/eredis` 包完成。组件在启动时根据配置自动选择合适的 go-redis 客户端类型，并注入可观测性拦截器（日志、指标、链路追踪），业务代码无需关心底层连接细节。

核心能力：

- **四种部署模式**：`stub`（单机）、`cluster`（集群）、`sentinel`（哨兵）、`ring`（分片环）。
- **统一接口**：所有模式都暴露 `redis.Cmdable`，业务层使用相同的 API。
- **分布式锁**：内置基于 `SET NX PX` + Lua 脚本的分布式锁客户端，支持重试策略。
- **可观测性拦截器**：debug 日志、结构化 access 日志、Prometheus 指标、OpenTelemetry 链路追踪。
- **连接池监控**：通过 governor 端点 `/debug/redis/stats` 暴露连接池指标，Prometheus 指标每 10 秒自动采集。
- **便捷方法**：Component 自身封装了 `Get`、`Set`、`HGet`、`HSet`、`Del`、`Incr`、`ZAdd`、`LPush`、`SAdd` 等常用操作。

## 核心用法

### 加载与构建组件

通过 `eredis.Load` 从配置中读取参数，然后调用 `Build` 构建组件实例：

```go
redisComp := eredis.Load("client.redis").Build()
```

在 Wire 注入体系中，组件作为 Options 的一部分自动注入：

```go
type Options struct {
    Redis *eredis.Component
}

// 获取通用 Cmdable 接口
client := s.Redis.Client()
```

### 模式选择

组件启动后可通过 `Mode()` 查询当前模式，也可使用模式专用访问器：

```go
// 查询模式
mode := s.Redis.Mode() // "stub" | "cluster" | "sentinel" | "ring"

// 模式专用访问器（不匹配时返回 nil）
cluster := s.Redis.Cluster()   // *redis.ClusterClient
stub    := s.Redis.Stub()      // *redis.Client
sentinel := s.Redis.Sentinel() // *redis.Client
ring    := s.Redis.Ring()      // *redis.Ring
```

::: warning 模式访问器安全
`Cluster()` 在 stub 模式下返回 `nil` 而非 panic。使用前务必检查返回值或先确认 `Mode()`。
:::

### 常见数据操作

组件提供了覆盖 String、Hash、List、Set、ZSet、Geo 的便捷方法，内部委托给 `redis.Cmdable`：

```go
// String
err := s.Redis.Set(ctx, "key", "value", time.Hour)
val, err := s.Redis.Get(ctx, "key")
err := s.Redis.SetNX(ctx, "lock:resource", "token", 10*time.Second)
cnt, err := s.Redis.Del(ctx, "key1", "key2")

// Hash
err := s.Redis.HSet(ctx, "user:1", "name", "John")
val, err := s.Redis.HGet(ctx, "user:1", "name")
m, err := s.Redis.HGetAll(ctx, "user:1")

// List
cnt, err := s.Redis.LPush(ctx, "queue", "task1", "task2")
val, err := s.Redis.RPop(ctx, "queue")
items, err := s.Redis.LRange(ctx, "queue", 0, -1)

// Set
cnt, err := s.Redis.SAdd(ctx, "tags", "go", "redis")
members, err := s.Redis.SMembers(ctx, "tags")

// ZSet
cnt, err := s.Redis.ZAdd(ctx, "leaderboard", redis.Z{Score: 100, Member: "alice"})
items, err := s.Redis.ZRange(ctx, "leaderboard", 0, 9)

// 计数器
n, err := s.Redis.Incr(ctx, "page:views")
n, err := s.Redis.IncrBy(ctx, "counter", 5)
```

也可以直接使用 `redis.Cmdable` 接口执行任意命令：

```go
client := s.Redis.Client()
client.Set(ctx, "key", "value", time.Hour)
client.Get(ctx, "key").Result()
client.Del(ctx, "key1", "key2").Err()
```

### 分布式锁

ERedis 内置分布式锁客户端，基于 Redis `SET NX PX` 和 Lua 原子脚本实现：

```go
lockClient := s.Redis.LockClient()

// 获取锁（默认不重试）
lock, err := lockClient.Obtain(ctx, "user:add", 5*time.Second)
if errors.Is(err, eredis.ErrNotObtained) {
    // 锁被其他进程持有
    return fmt.Errorf("resource locked")
}
if err != nil {
    return err
}
defer lock.Release(ctx)

// 临界区操作
// ...
```

#### 重试策略

锁获取支持可插拔的重试策略：

```go
import "internal/component/eredis"

// 不重试（默认）
lock, err := lockClient.Obtain(ctx, "key", ttl)

// 固定间隔重试
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(eredis.LinearBackoffRetry(100*time.Millisecond)),
)

// 指数退避重试（最小 16ms，最大 1s）
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(eredis.ExponentialBackoffRetry(16*time.Millisecond, time.Second)),
)

// 限制重试次数
lock, err := lockClient.Obtain(ctx, "key", ttl,
    eredis.WithLockOptionRetryStrategy(
        eredis.LimitRetry(eredis.LinearBackoffRetry(100*time.Millisecond), 5),
    ),
)
```

#### 锁的生命周期管理

```go
lock, err := lockClient.Obtain(ctx, "key", 10*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx)

// 查询剩余 TTL
remaining, err := lock.TTL(ctx)

// 续期（仅在锁仍持有时成功）
err = lock.Refresh(ctx, 20*time.Second)

// 获取锁的 key 和 token
key := lock.Key()
token := lock.Token()
```

### Build 选项

`Build` 方法支持函数选项，在构建时覆盖配置：

```go
redisComp := eredis.Load("client.redis").Build(
    eredis.WithStub(),              // 强制 stub 模式
    eredis.WithAddr("127.0.0.1:6380"),
    eredis.WithPassword("secret"),
    eredis.WithDB(1),
    eredis.WithPoolSize(50),
)
```

可用选项：

| 选项 | 说明 |
|------|------|
| `WithStub()` | 设置模式为 stub |
| `WithCluster()` | 设置模式为 cluster |
| `WithSentinel()` | 设置模式为 sentinel |
| `WithRing()` | 设置模式为 ring |
| `WithAddr(addr)` | 设置单机地址 |
| `WithAddrs(addrs)` | 设置集群/哨兵/环地址列表 |
| `WithPassword(p)` | 设置密码 |
| `WithDB(n)` | 设置数据库编号 |
| `WithPoolSize(n)` | 设置连接池大小 |
| `WithMasterName(name)` | 设置哨兵主节点名称 |

## 配置示例

### Standalone（Stub）模式

```toml
[client.redis]
addr = "127.0.0.1:6380"
mode = "stub"
password = "egoadmin"
db = 0
debug = true
```

### Cluster 模式

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "cluster"
password = "egoadmin"
readOnly = true
```

### Sentinel 模式

```toml
[client.redis]
addrs = ["sentinel-1:26379", "sentinel-2:26379", "sentinel-3:26379"]
mode = "sentinel"
masterName = "mymaster"
password = "egoadmin"
sentinelPassword = "sentinel-pass"
db = 0
```

### Ring 模式

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "ring"
password = "egoadmin"
db = 0
```

### 全量配置参考

```toml
[client.redis]
addr = "127.0.0.1:6380"           # stub 模式地址
addrs = []                         # cluster/sentinel/ring 模式地址列表
mode = "stub"                      # stub | cluster | sentinel | ring
masterName = ""                    # sentinel 模式主节点名称
password = ""                      # 连接密码
sentinelUsername = ""              # sentinel 模式用户名
sentinelPassword = ""              # sentinel 模式密码
db = 0                             # 数据库编号（cluster 模式不适用）
poolSize = 20                      # 每个节点最大连接数
maxRetries = 0                     # 网络错误最大重试次数
minIdleConns = 4                   # 最小空闲连接数
dialTimeout = "1s"                 # 连接超时
readTimeout = "1s"                 # 读超时
writeTimeout = "1s"                # 写超时
idleTimeout = "60s"                # 连接最大空闲时间
readOnly = false                   # cluster 模式从节点读
debug = false                      # 调试日志开关
slowLogThreshold = "250ms"         # 慢日志门限
onFail = "panic"                   # 启动失败行为：panic | error

# 拦截器
enableMetricInterceptor = true     # Prometheus 指标
enableTraceInterceptor = true      # OpenTelemetry 链路
enableAccessInterceptor = false    # 结构化访问日志
enableAccessInterceptorReq = false # 记录请求参数
enableAccessInterceptorRes = false # 记录响应参数

# TLS
[client.redis.authentication]
# TLS 配置（见 authentication.go）
```

### JetCache 集成

ERedis 配合 JetCache 实现多级缓存（本地 LRU + Redis 远程）：

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

适用场景：权限快照、用户基本信息、高频读且允许短时间缓存的数据。

## 实战示例

### 在服务中注入并使用 ERedis

```go
// internal/app/user/server/options.go
type Options struct {
    Redis    *eredis.Component
    JetCache *ejetcache.Component
}

// internal/app/user/server/service.go
type UserService struct {
    opts Options
}

func (s *UserService) GetUserProfile(ctx context.Context, userID uint64) (*Profile, error) {
    key := fmt.Sprintf("user:profile:%d", userID)

    // 尝试从缓存获取
    data, err := s.opts.Redis.GetBytes(ctx, key)
    if err == nil {
        var p Profile
        if err := json.Unmarshal(data, &p); err == nil {
            return &p, nil
        }
    }

    // 缓存未命中，从数据库加载
    profile, err := s.loadFromDB(ctx, userID)
    if err != nil {
        return nil, err
    }

    // 写入缓存
    bytes, _ := json.Marshal(profile)
    _ = s.opts.Redis.Set(ctx, key, bytes, 30*time.Minute)

    return profile, nil
}
```

### 防重复提交

```go
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderReq) error {
    lockKey := fmt.Sprintf("lock:order:create:%d", req.UserID)
    lock, err := s.Redis.LockClient().Obtain(ctx, lockKey, 10*time.Second,
        eredis.WithLockOptionRetryStrategy(eredis.LinearBackoffRetry(50*time.Millisecond)),
    )
    if errors.Is(err, eredis.ErrNotObtained) {
        return fmt.Errorf("请勿重复提交")
    }
    if err != nil {
        return err
    }
    defer lock.Release(ctx)

    // 创建订单逻辑
    return s.doCreateOrder(ctx, req)
}
```

### 限流计数器

```go
func (s *GatewayService) RateLimit(ctx context.Context, userID uint64, limit int64) (bool, error) {
    key := fmt.Sprintf("ratelimit:%d:%d", userID, time.Now().Unix()/60)
    n, err := s.Redis.Incr(ctx, key)
    if err != nil {
        return false, err
    }
    if n == 1 {
        // 首次访问，设置过期
        _, _ = s.Redis.Expire(ctx, key, time.Minute)
    }
    return n <= limit, nil
}
```

## 工作原理

### 初始化流程

```
eredis.Load("client.redis")
    |
    v
Container.Build()
    |-- 解析配置，确定 mode
    |-- 按顺序注入拦截器：
    |     1. fixedInterceptor（基础错误包装）
    |     2. debugInterceptor（仅 debug=true 时）
    |     3. metricInterceptor（仅 enableMetricInterceptor=true 时）
    |     4. accessInterceptor（仅 enableAccessInterceptor=true 时）
    |     5. traceInterceptor（仅 enableTraceInterceptor=true 时）
    |
    v
根据 mode 创建对应客户端：
    |-- cluster  -> redis.NewClusterClient (使用 Addrs)
    |-- stub     -> redis.NewClient (使用 Addr)
    |-- sentinel -> redis.NewFailoverClient (使用 Addrs + MasterName)
    |-- ring     -> redis.NewRing (使用 Addrs，自动分片命名 shard1, shard2...)
    |
    v
为客户端注册 Hook（AddHook）
    |
    v
执行 Ping 连通性测试
    |-- 成功 -> 返回 Component
    |-- 失败 -> 根据 onFail 配置 panic 或 error
    |
    v
创建 lockClient（共享底层 Cmdable）
    |
    v
注册到 instances（用于 stats 和监控）
```

### 拦截器链

ERedis 利用 go-redis 的 `Hook` 接口实现拦截。每个拦截器实现 `ProcessHook` 和 `ProcessPipelineHook`，在命令执行前后插入逻辑：

| 拦截器 | 触发条件 | 作用 |
|--------|----------|------|
| `fixed` | 始终启用 | 记录起始时间，包装非 NOSCRIPT 错误 |
| `debug` | `debug=true` 且 `EGO_DEBUG=true` | 打印命令参数、响应和耗时到 stdout |
| `metric` | `enableMetricInterceptor=true` | 记录 Prometheus 直方图（耗时）和计数器（状态） |
| `access` | `enableAccessInterceptor=true` 或超过慢日志门限 | 输出结构化 JSON 日志，含请求/响应、耗时、trace ID |
| `trace` | `enableTraceInterceptor=true` | 创建 OpenTelemetry span，注入 net.peer、db.system 等属性 |

### 分布式锁机制

锁使用三个 Lua 脚本保证原子性：

| 脚本 | 用途 | Redis 命令 |
|------|------|-----------|
| `SET NX PX` | 获取锁 | 原子设置值和 TTL |
| `luaRefresh` | 续期 | `GET` + `PEXPIRE`（仅值匹配时） |
| `luaRelease` | 释放 | `GET` + `DEL`（仅值匹配时） |
| `luaPTTL` | 查询剩余 TTL | `GET` + `PTTL`（仅值匹配时） |

锁的值由 16 字节随机 token（Base64 编码）加上可选 metadata 组成。所有 Lua 脚本通过值比对防止误释放其他进程的锁。

### 连接池监控

组件在 `init()` 中注册 governor 端点和后台监控 goroutine：

- `GET /debug/redis/stats`：返回所有 Redis 实例的连接池统计（JSON）。
- 后台 goroutine 每 10 秒采集一次 `PoolStats`，写入 Prometheus gauge，标签包括 `hits`、`misses`、`timeouts`、`total_conns`、`idle_conns`、`stale_conns`。

## 常见问题

### Cluster() 返回 nil

在 stub 模式下调用 `Cluster()` 会返回 `nil`，解引用会 panic。正确做法是先检查 `Mode()`：

```go
if s.Redis.Mode() == eredis.ClusterMode {
    cluster := s.Redis.Cluster()
    // 使用 cluster...
}
```

### 连接池耗尽

高并发场景下默认 `poolSize=20` 可能不够。症状是请求超时或 `connection pool exhausted` 错误。调大连接池并适当增加 `minIdleConns`：

```toml
[client.redis]
poolSize = 100
minIdleConns = 20
```

同时注意检查是否有未关闭的连接或长时间阻塞的命令（如 `KEYS *`）。

### 锁未释放

如果进程在持有锁期间崩溃，锁会在 TTL 到期后自动释放。但正常路径中必须显式释放：

```go
lock, err := lockClient.Obtain(ctx, "key", 10*time.Second)
if err != nil {
    return err
}
defer lock.Release(ctx) // 始终 defer
```

不要依赖 TTL 作为唯一的释放机制，TTL 只是安全网。

### 生产环境开启 debug

`debug = true` 会为每条 Redis 命令打印日志。在高 QPS 环境下会产生大量日志输出，严重影响性能。生产环境应确保 `debug = false`。

如果需要排查问题，临时开启 `enableAccessInterceptor` 并配合 `slowLogThreshold` 只记录慢请求：

```toml
[client.redis]
debug = false
enableAccessInterceptor = true
enableAccessInterceptorReq = true
slowLogThreshold = "100ms"
```

### redis.Nil 错误

`redis.Nil` 是 go-redis 在 key 不存在时返回的特殊错误，不是真正的异常。ERedis 的 access 拦截器会将其降级为 WARN（而非 ERROR）。业务代码中判断 key 不存在应使用 `errors.Is(err, redis.Nil)`：

```go
val, err := s.Redis.Get(ctx, key)
if errors.Is(err, redis.Nil) {
    // key 不存在，不是错误
} else if err != nil {
    return err
}
```

### Sentinel 模式配置遗漏

Sentinel 模式需要同时配置 `addrs`（哨兵地址）和 `masterName`（主节点名称）。缺少任一项会导致组件启动 panic。

### 容器环境地址配置

容器内必须使用 Docker Compose service DNS 名称，不能使用 `127.0.0.1`：

```toml
[client.redis]
addr = "redis:6380"
mode = "stub"
```

集群模式同理：

```toml
[client.redis]
addrs = ["redis-1:6379", "redis-2:6379", "redis-3:6379"]
mode = "cluster"
```

## 参考链接

- `internal/component/eredis/component.go` -- Component 定义与模式访问器
- `internal/component/eredis/config.go` -- Config 结构体与默认值
- `internal/component/eredis/container.go` -- Load / Build 流程与四种客户端构建
- `internal/component/eredis/option.go` -- Build 函数选项
- `internal/component/eredis/interceptor.go` -- 拦截器实现（debug、metric、access、trace）
- `internal/component/eredis/comopnent_cmds.go` -- Component 便捷方法（Get/Set/HGet/HSet 等）
- `internal/component/eredis/lock.go` -- 分布式锁实现（Obtain/Release/Refresh/TTL）
- `internal/component/eredis/lock_retry.go` -- 重试策略（Linear/Exponential/Limit/NoRetry）
- `internal/component/eredis/error.go` -- 错误常量（ErrNotObtained、ErrLockNotHeld）
- `internal/component/eredis/stat.go` -- 连接池监控与 governor 端点
