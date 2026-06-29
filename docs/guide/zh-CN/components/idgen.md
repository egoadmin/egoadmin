# IDGen ID 生成

IDGen 为 EgoAdmin 提供全局唯一 ID 生成能力，支持雪花算法和号段两种策略，并附带 idcodec 稳定 ID 编解码与机器租约管理。

## 概述

EgoAdmin 的 IDGen 组件位于 `internal/component/idgen`，对外通过 `Interface` 接口暴露核心能力。它由三个子模块组成：

| 子模块 | 包路径 | 职责 |
|--------|--------|------|
| Segment 生成器 | `internal/component/idgen` | 号段模式：从数据库预分配 ID 区间，减少数据库往返 |
| idcodec 编解码器 | `internal/component/idgen/idcodec` | 将内部数字 ID 可逆编码为稳定的公开 ID |
| Machine 租约 | `internal/component/idgen` (`MachineLeaseManager`) | 分布式 worker ID 协调，管理进程级机器租约 |

::: tip 何时使用 IDGen
- 数据库主键：使用默认的号段生成器 `Next()` / `NextDefault()`。
- 高吞吐批量分配：使用 `Reserve()` 一次获取 ID 区间。
- 公开 ID 展示：使用 idcodec 编码内部 ID，避免暴露自增序列。
- 外部系统引用：idcodec 编码后的 ID 可用于 URL、邀请码等对外场景。
:::

## 核心用法

### 号段 ID 生成

号段模式是 EgoAdmin 的默认 ID 生成策略。组件启动时自动从数据库预分配一个 ID 区间（`[Start, End)`），进程内通过原子操作分配 ID，当剩余量不足时异步预取下一段。

```go
// 通过 Wire 注入 idgen.Component
func (r *UserRepository) Create(ctx context.Context, user *User) error {
    id, err := r.idgen.Next(ctx, "user_id")
    if err != nil {
        return fmt.Errorf("generate user id: %w", err)
    }
    user.ID = id
    return r.db.WithContext(ctx).Create(user).Error
}
```

常用方法：

| 方法 | 说明 |
|------|------|
| `Next(ctx, name)` | 获取下一个 ID（`int64`） |
| `NextDefault(ctx)` | 使用配置中的默认 name 获取下一个 ID |
| `Reserve(ctx, name, n)` | 一次保留 n 个 ID，返回 `Range{Start, End}` |
| `ReserveDefault(ctx, n)` | 使用默认 name 批量保留 ID |
| `Generator(name)` | 获取命名生成器句柄，适合高频场景复用 |
| `Stats(name)` | 查询当前生成器快照（剩余量、预取状态等） |
| `Health(ctx)` | 健康检查（存储可用性 + 机器租约状态） |

### 批量 ID 保留

当需要在一次操作中分配大量 ID（如批量导入），使用 `Reserve` 避免逐个调用的开销：

```go
func (r *OrderRepository) BatchCreate(ctx context.Context, orders []*Order) error {
    rng, err := r.idgen.Reserve(ctx, "order_id", int64(len(orders)))
    if err != nil {
        return fmt.Errorf("reserve order ids: %w", err)
    }
    for i, order := range orders {
        order.ID = rng.Start + int64(i)
    }
    return r.db.WithContext(ctx).CreateInBatches(orders, 100).Error
}
```

`Range` 是一个左闭右开区间 `[Start, End)`，调用方自行按需分配。

### 稳定 ID 编解码（idcodec）

idcodec 将内部数字 ID 编码为带前缀的稳定公开 ID。编码使用 Feistel 网络 + HMAC-SHA256 + Base62，可逆但不可预测。

```go
codec := idcodec.Load("component.idgen.codec").Build()

// 编码：内部 ID -> 公开 ID
publicID, err := codec.Encode("order", 12345)
// 结果示例: "order-07uQlcBmL6d0"

// 解码：公开 ID -> 内部 ID
prefix, id, err := codec.Decode(publicID)
// prefix = "order", id = 12345

// 带前缀校验的解码
id, err := codec.DecodeWithPrefix("order", publicID)
// id = 12345
```

idcodec 的核心接口：

| 方法 | 说明 |
|------|------|
| `Encode(prefix, id)` | 将正整数 ID 编码为 `prefix-separator-Base62Body` 格式 |
| `Decode(value)` | 解码公开 ID，返回 `(prefix, id, err)` |
| `DecodeWithPrefix(prefix, value)` | 解码并校验前缀是否匹配 |

::: warning idcodec 不是鉴权机制
idcodec 使用 HMAC-SHA256 + Feistel 网络实现可逆编码。它是混淆而非加密，不提供安全性保证。不要将 idcodec 用于安全敏感场景（如邀请码、重置密码令牌），这些应使用密码学安全的随机令牌。
:::

### 公开 ID 在 API 层的使用

典型分层中，idcodec 在 adapter/schema 层编解码，domain 层始终使用 `int64`：

```go
// adapter 层：编码
func toPublicUser(u *domain.User) *schema.UserPublic {
    publicID, _ := idcodec.Encode("usr", u.ID)
    return &schema.UserPublic{
        ID:        publicID,
        Name:      u.Name,
        CreatedAt: u.CreatedAt,
    }
}

// adapter 层：解码
func parseUserID(publicID string) (int64, error) {
    return idcodec.DecodeWithPrefix("usr", publicID)
}
```

### Generator 缓存句柄

对于高频 ID 生成场景，可缓存 Generator 句柄避免重复查找：

```go
gen, err := comp.Generator("order")

// 后续使用 gen 直接生成
id, err := gen.Next(ctx)

// 批量预留
rng, err := gen.Reserve(ctx, 100)

// 查看统计信息
stats := gen.Stats()
fmt.Printf("已生成: %d, 剩余: %d\n", stats.Generated, stats.CurrentRemaining)
```

### IDGetter 适配器

将 idgen 适配为 `xflake.Geter` 接口形态，返回 `uint64`：

```go
gen, err := comp.Generator("snowflake")
getter := idgen.NewIDGetter(gen)

id, err := getter.Get() // 返回 uint64
```

## 配置示例

### 组件配置（config.toml）

```toml
# 号段生成器实例
[component.idgen.default]
namespace = "egoadmin-local"       # 命名空间隔离，不同环境使用不同值
name = "default"                   # 实例名称，对应 Segment 名
step = 100000                      # 每次预分配的 ID 数量
minStep = 10000                    # 动态步长下限
maxStep = 100000000                # 动态步长上限
autoEnsure = true                  # 启动时自动创建缺失的 segment 定义
warmup = true                      # 启动时预热第一个号段
fetchTimeout = "2s"                # 从数据库获取号段的超时时间
waitTimeout = "200ms"              # 号段耗尽时等待预取的超时时间
prefetchRemainingRatio = 0.2       # 剩余比例低于此值时触发异步预取
dynamicStep = true                 # 启用动态步长调整
targetDuration = "15m"             # 动态步长的目标消耗时间
maxPrefetchWorkers = 8             # 最大并发预取 worker 数
enableMetrics = true               # 启用 Prometheus 指标

# 机器租约配置
[component.idgen.machine]
group = "egoadmin-local"           # 租约组名，同组共享机器 ID 空间
maxMachineID = 1023                # 最大机器 ID
ttl = "60s"                        # 租约 TTL
renewInterval = "10s"              # 续约间隔（必须小于 TTL）
renewTimeout = "5s"                # 续约超时
minRenewWindows = 5                # 最小续约窗口数，TTL 至少覆盖 (minRenewWindows+1) * renewInterval
reallocateBackoff = "2s"           # 分配失败后重试退避
stableInstanceID = ""              # 稳定实例 ID（如 Kubernetes Pod 名），留空则使用 hostname:pid
lostPolicy = "fail_closed"         # 租约丢失策略：degraded（降级）或 fail_closed（拒绝服务）

# idcodec 编解码器配置
[component.idgen.codec]
secret = "local-stable-idcodec-secret"  # HMAC 密钥（至少 16 字节）
algorithm = "feistel-base62"             # 编码算法
alphabet = "base62"                      # 字母表，base62 或自定义 62 字符
minLength = 12                           # 编码后 body 最小长度
separator = "-"                          # 前缀与 body 的分隔符
enableMetrics = true                     # 启用 Prometheus 指标
```

### gRPC 客户端配置

非 idgen 服务通过 gRPC 客户端访问 IDGen 服务：

```toml
[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"    # 服务发现地址
debug = true                       # 启用调试日志
readTimeout = "3s"                 # 读超时
dialTimeout = "5s"                 # 连接超时
```

客户端通过 Wire 注入：

```go
// idgenclient.Client 自动通过 egrpc.Load("client.grpc.idgen") 构建
type Client struct {
    Segment      SegmentService       // 号段操作
    MachineLease MachineLeaseService  // 租约操作
}
```

### 默认值速查

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `step` | `100000` | 每次预分配量 |
| `dynamicStep` | `true` | 根据消耗速率自动调整步长 |
| `targetDuration` | `15m` | 动态步长目标间隔 |
| `prefetchRemainingRatio` | `0.2` | 剩余 20% 时触发预取 |
| `maxPrefetchWorkers` | `8` | 并发预取上限 |
| `maxMachineID` | `1023` | 机器 ID 上限 |
| `lostPolicy` | `fail_closed` | 租约丢失时拒绝生成 ID |
| `secret`（codec） | 无 | 必填，至少 16 字节 |
| `minLength`（codec） | `12` | 编码后 body 最短长度 |

## 实战示例

### 用户创建完整流程

```go
// domain 层：User 实体
type User struct {
    ID        int64
    Name      string
    Email     string
    CreatedAt time.Time
}

// application 层：创建用例
type CreateUserUseCase struct {
    idgen idgen.Interface
    userRepo UserRepository
}

func (uc *CreateUserUseCase) Execute(ctx context.Context, cmd CreateUserCommand) (*User, error) {
    id, err := uc.idgen.Next(ctx, "user_id")
    if err != nil {
        return nil, fmt.Errorf("generate user id: %w", err)
    }
    user := &User{
        ID:    id,
        Name:  cmd.Name,
        Email: cmd.Email,
    }
    if err := uc.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }
    return user, nil
}

// adapter 层：返回公开 ID
func toPublicUser(u *User) *UserPublicResponse {
    publicID, _ := idcodec.Encode("usr", u.ID)
    return &UserPublicResponse{
        ID:    publicID,  // "usr-Ab3kL9xQm2Nf"
        Name:  u.Name,
        Email: u.Email,
    }
}
```

### 批量订单导入

```go
func (uc *ImportOrdersUseCase) Execute(ctx context.Context, rawOrders []RawOrder) error {
    rng, err := uc.idgen.Reserve(ctx, "order_id", int64(len(rawOrders)))
    if err != nil {
        return fmt.Errorf("reserve order ids: %w", err)
    }

    orders := make([]*Order, len(rawOrders))
    for i, raw := range rawOrders {
        orders[i] = &Order{
            ID:     rng.Start + int64(i),
            UserID: raw.UserID,
            Amount: raw.Amount,
        }
    }
    return uc.orderRepo.BatchCreate(ctx, orders)
}
```

### 带前缀的公开 ID 路由解析

```go
// 解析 URL 中的公开 ID
func ParseResourceID(prefix string, publicID string) (int64, error) {
    id, err := idcodec.DecodeWithPrefix(prefix, publicID)
    if err != nil {
        return 0, fmt.Errorf("invalid %s id: %w", prefix, err)
    }
    return id, nil
}

// 示例路由: GET /orders/:id
// URL: /orders/order-07uQlcBmL6d0
// 解析: order-07uQlcBmL6d0 -> 12345
```

## 机器租约详解

每个 IDGen 进程在启动时向 IDGen 服务申请一个机器租约，用于 Snowflake 算法的 worker ID 协调。

### 生命周期

```text
Start() -> AllocateLease -> [renew loop via ecron] -> Stop() / StopWithoutRelease()
```

1. **分配**：进程启动时调用 `AllocateLease`，获取唯一 `machineID`。
2. **续约**：后台 cron（`cron.idgen.machine.renew`）按 `renewInterval` 定期续约。
3. **丢失处理**：
   - `fail_closed`（默认）：租约丢失后拒绝生成 ID。
   - `degraded`：允许继续使用已分配的号段。
4. **释放**：进程退出时调用 `Stop()` 释放租约，或 `StopWithoutRelease()` 静默停止续约。

### 优雅停机

```go
// 推荐：在 shutdown.Manager 中注册
shutdown.RegisterWithTimeout(5*time.Second, func(ctx context.Context) {
    machineManager.StopWithoutRelease(ctx)
})
```

`StopWithoutRelease` 在停机时停止续约但不调用远程释放接口，避免在 IDGen 服务已经停止时产生噪音错误。TTL 过期后租约自动回收。

### 实例 ID 策略

| 场景 | 推荐配置 |
|------|----------|
| 本地开发 | 留空 `stableInstanceID`，使用 `hostname:pid` |
| Docker Compose | 使用容器名 |
| Kubernetes | 使用 Pod 名称（`metadata.name`） |

## 工作原理

### 号段模式（Segment）

```text
                  应用调用 Next("user_id")
                         |
                         v
            segmentGenerator（进程内缓存）
            当前段: [1000, 2000), cursor = 1500
                         |
                    原子递增 cursor
                         |
              剩余 < 20% -> 异步预取下一段
                         |
                    返回 int64 ID
```

核心设计：

- **双缓冲**：持有当前段和预取段（`current` + `next`），当前段耗尽时无缝切换。
- **动态步长**：根据历史消耗速率自动调整下次预取量（`stepPolicy`），目标是每次预取覆盖 `targetDuration`（默认 15 分钟）。
- **并发安全**：segment 内部使用 `atomic.Int64` 实现无锁分配。
- **自动 Ensure**：`autoEnsure = true` 时，启动阶段自动在数据库创建缺失的 segment 定义。

### 动态步长算法

当 `dynamicStep = true` 时，步长根据号段消耗速度自动调整：

- 消耗时间 < `targetDuration`（15 分钟）：步长翻倍（快速消耗，需要更大的步长）。
- 消耗时间 >= 2 * `targetDuration`：步长减半（消耗缓慢，减少浪费）。
- 其他情况：步长不变。

步长始终限制在 `[minStep, maxStep]` 范围内。

### 预取机制

当当前号段剩余量 <= `prefetchRemainingRatio`（默认 20%）时触发预取：

1. 启动后台 goroutine 从数据库加载下一个号段到 `next`。
2. 当 `current` 耗尽时，如果 `next` 已就绪，直接切换（零延迟）。
3. 如果 `next` 未就绪，等待最多 `waitTimeout`（200ms），超时后同步拉取。
4. 预取受 `maxPrefetchWorkers` 限制，避免过多并发数据库请求。

### Snowflake 模式

当配置了机器租约时，IDGen 也支持 Snowflake 算法。64 位 ID 结构：

```text
|  0  |    41 位时间戳    |  10 位机器 ID  |  12 位序列号  |
| --- | ----------------- | -------------- | ------------- |
|  1b |       41b         |      10b       |      12b      |
```

- 时间戳：毫秒级，相对于自定义纪元。
- 机器 ID：由机器租约分配，最大 1023。
- 序列号：同一毫秒内的自增序列，每毫秒最多 4096 个。

### idcodec 编码算法

```text
内部 ID (int64)
    |
    v
Feistel 网络置换（6 轮，HMAC-SHA256 密钥派生）
    |
    v
Base62 编码（可配置字母表）
    |
    v
补齐到 minLength + 拼接 prefix-separator-body
    |
    v
公开 ID: "order-07uQlcBmL6d0"
```

- 使用 Feistel 网络实现可逆的比特置换，每轮的 round function 使用 HMAC-SHA256 以 secret 和 prefix 作为输入。
- 不同 prefix 使用不同的置换映射，同一数字 ID 在不同前缀下编码结果不同。
- 解码过程是编码的精确逆操作。

### Prometheus 指标

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|------|------|
| `idgen_generated_total` | Counter | component, name, operation, status | ID 生成计数 |
| `idgen_segment_fetch_total` | Counter | component, name, status | 号段拉取次数 |
| `idgen_segment_fetch_seconds` | Histogram | component, name | 号段拉取延迟 |
| `idgen_segment_remaining` | Gauge | component, name | 当前号段剩余量 |
| `idgen_segment_step` | Gauge | component, name | 当前有效步长 |
| `idgen_prefetch_total` | Counter | component, name, status | 预取尝试次数 |
| `idgen_machine_lease_renew_total` | Counter | component, status | 租约续期次数 |
| `idgen_machine_lease_status` | Gauge | component | 租约状态（1=有效, 0=丢失） |
| `idgen_health_status` | Gauge | component | 组件健康状态 |

## 常见问题

### 时钟漂移导致 ID 冲突

::: danger
Snowflake 算法对系统时钟敏感。如果发生时钟回拨，可能产生重复 ID。
:::

**症状**：日志出现 `idgen machine lease lost` 或 ID 重复。

**排查**：

```bash
# 检查 NTP 同步状态
timedatectl status
ntpq -p
```

**缓解**：号段模式（EgoAdmin 默认）不受时钟漂移影响，因为它不依赖系统时钟生成 ID。

### 机器 ID 冲突

::: warning
确保每个 worker 进程拥有唯一的机器租约。
:::

**原因**：多个进程使用相同的 `stableInstanceID`。

**排查**：检查 `[component.idgen.machine].stableInstanceID` 配置，在容器环境中使用唯一的 Pod/容器名。

### JavaScript 精度丢失

::: warning
Go 的 `int64` / `uint64` 超过 JavaScript `Number.MAX_SAFE_INTEGER`（2^53 - 1）时会丢失精度。
:::

**解决方案**：

1. 使用 idcodec 将 ID 编码为字符串后传给前端。
2. 在 proto / JSON 序列化中将 ID 字段定义为 `string` 类型。
3. 前端始终以字符串处理 ID，不做数值运算。

```protobuf
message UserResponse {
    string id = 1;    // 使用 string 而非 uint64
    string name = 2;
}
```

### idcodec 解码失败

**症状**：`ErrInvalidFormat` 或 `ErrInvalidPrefix`。

**排查清单**：

- 公开 ID 是否完整（未被截断）。
- 前缀是否正确（区分大小写）。
- separator 是否匹配配置。
- idcodec 实例的 secret 是否与编码时一致。

### 号段耗尽或预取超时

**症状**：`ErrSegmentExhausted` 或 `ErrStoreUnavailable`。

**排查**：

- 数据库连接是否正常。
- `fetchTimeout` 是否过短（默认 2 秒）。
- segment 定义的 step 是否过小（高并发场景建议增大）。
- 检查 `Stats()` 中的 `SegmentFetchFails` 计数。

### 动态步长调整不生效

**原因**：`dynamicStep = false` 或 `targetDuration` 配置不合理。

动态步长的行为：

- 消耗速度快于目标 -> 下次步长翻倍（不超过 `maxStep`）。
- 消耗速度慢于目标（超过 2 倍） -> 下次步长减半（不低于 `minStep`）。
- 消耗速度在目标范围内 -> 步长不变。

### 租约丢失后的 ID 唯一性

`LostPolicy = "degraded"` 模式下，租约丢失后继续生成 ID，但不再保证 Snowflake machine ID 部分唯一。如果需要强唯一性保证，使用 `fail_closed` 策略。

### 组件关闭

```go
// 优雅关闭：停止续约，释放远程租约
err := comp.Stop()

// 停止续约但不释放远程租约（适合进程退出时远程服务已停止的场景）
err := machineManager.StopWithoutRelease(ctx)
```

## 错误码参考

| 错误 | 含义 | 常见原因 |
|------|------|----------|
| `ErrInvalidConfig` | 配置无效 | 缺少必填字段或值超出范围 |
| `ErrStoreUnavailable` | 存储不可用 | 数据库连接失败 |
| `ErrNameNotFound` | segment 名称不存在 | 未创建 segment 定义 |
| `ErrNameDisabled` | segment 已禁用 | segment status != 1 |
| `ErrSegmentExhausted` | 号段耗尽 | 预取失败且当前段用尽 |
| `ErrOverflow` | ID 溢出 | 超出 int64 范围 |
| `ErrComponentClosed` | 组件已关闭 | 进程正在退出 |
| `ErrMachineLeaseLost` | 机器租约丢失 | 续约失败或 TTL 过期 |
| `ErrMachineIDOverflow` | 机器 ID 溢出 | 超过 maxMachineID |

## 参考链接

- `internal/component/idgen/component.go` -- Component 定义与生命周期（Next, Reserve, Generator, Health）
- `internal/component/idgen/config.go` -- Config 和 MachineConfig 结构体与默认值
- `internal/component/idgen/container.go` -- Load / Build 流程
- `internal/component/idgen/interface.go` -- Interface、Generator、SegmentStore、MachineAllocator 接口定义
- `internal/component/idgen/generator.go` -- segmentGenerator 实现（Next、Reserve、prefetch、switchOrFetch）
- `internal/component/idgen/segment.go` -- 无锁号段数据结构（atomic.Int64 cursor）
- `internal/component/idgen/step.go` -- 动态步长策略（stepPolicy）
- `internal/component/idgen/machine_manager.go` -- ProcessMachineLeaseManager（Allocate、Renew、Release、StopWithoutRelease）
- `internal/component/idgen/machine_cron.go` -- ecron 续约定时任务
- `internal/component/idgen/options.go` -- Build 函数选项（WithConfig、WithSegmentStore、WithMachineLeaseManager）
- `internal/component/idgen/errors.go` -- 错误常量
- `internal/component/idgen/metrics.go` -- Prometheus 指标定义
- `internal/component/idgen/idgetter.go` -- IDGetter 适配器（xflake.Geter 形态）
- `internal/component/idgen/idcodec/component.go` -- idcodec Feistel + Base62 编解码实现
- `internal/component/idgen/idcodec/config.go` -- idcodec 配置与默认值
- `internal/component/idgen/idcodec/interface.go` -- idcodec Interface 定义（Encode、Decode、DecodeWithPrefix）
- `internal/component/idgen/idcodec/container.go` -- idcodec Load / Build 流程
- `internal/client/idgenclient/client.go` -- gRPC 客户端（SegmentService + MachineLeaseService）
