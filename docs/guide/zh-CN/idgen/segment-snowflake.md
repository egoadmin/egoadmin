# 号段与雪花算法

EgoAdmin 的 IDGen 服务提供号段（Segment）和雪花（Snowflake）两种全局唯一 ID 生成策略，均通过 `internal/component/idgen` 组件统一对外暴露。

## 概述

EgoAdmin 默认使用号段模式作为主 ID 生成策略，通过数据库预分配 ID 区间实现高吞吐。雪花算法作为补充，用于需要时间有序性的场景。两种策略共享同一组件框架和机器租约管理。

| 策略 | 位宽 | 生成方式 | 适用场景 |
|------|------|----------|----------|
| 号段 | `int64` | 数据库预分配区间 + 进程内原子递增 | 数据库主键、批量导入 |
| 雪花 | `uint64` | 时间戳 + 机器 ID + 序列号 | 需要时间排序的分布式 ID |

::: tip 选择建议
大多数业务场景使用号段模式即可。雪花模式对系统时钟有要求，需要确保 NTP 同步。EgoAdmin 的 `IDGetter` 适配器可将号段生成器包装为雪花 `xflake.Geter` 接口形态。
:::

## 核心用法

### 号段模式

号段模式是 EgoAdmin 默认的 ID 生成策略。组件启动时自动从数据库预分配一个 ID 区间 `[Start, End)`，进程内通过原子操作分配 ID，当剩余量低于阈值时异步预取下一段。

```go
// 通过 Wire 注入 idgen.Component
type UserRepository struct {
    idgen *idgen.Component
}

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

### 雪花模式

当配置了机器租约时，IDGen 通过 `IDGetter` 适配器支持雪花算法。64 位 ID 结构：

```text
| 1 bit 符号位 | 41 位时间戳 | 10 位机器 ID | 12 位序列号 |
| ------------- | ----------- | ------------ | ----------- |
|       0       |   41 bits   |   10 bits    |   12 bits   |
```

- **符号位**：固定为 0，保证 ID 为正数。
- **时间戳**：毫秒级，相对于自定义 epoch（2020 年 1 月 1 日）。
- **机器 ID**：由机器租约分配，范围 `[0, 1023]`。
- **序列号**：同一毫秒内的自增序列，每毫秒最多 4096 个。

雪花模式通过 `IDGetter` 适配器使用：

```go
// 在 platform/idgen/provider.go 中构建
gen, err := component.GeneratorDefault()
getter := idgen.NewIDGetter(gen)

// 获取 ID（uint64）
id, err := getter.Get()
```

::: warning 时钟敏感
雪花算法依赖系统时钟。发生时钟回拨时可能产生重复 ID。号段模式不受时钟漂移影响。
:::

## 配置示例

### 组件配置

```toml
# 号段生成器实例
[component.idgen.default]
namespace = "egoadmin-local"       # 命名空间隔离，不同环境使用不同值
name = "default"                   # 实例名称，对应 segment 名称
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
```

### 最小配置

```toml
[component.idgen.default]
namespace = "egoadmin-local"
name = "default"
```

::: tip 默认值
未配置的字段使用 `DefaultConfig()` 中的保守默认值：`step = 100000`，`dynamicStep = true`，`prefetchRemainingRatio = 0.2`，`targetDuration = 15m`。
:::

### 默认值速查

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `namespace` | `"default"` | 命名空间 |
| `name` | `"default"` | 实例名称 |
| `step` | `100000` | 每次预分配量 |
| `minStep` | `10000` | 动态步长下限 |
| `maxStep` | `100000000` | 动态步长上限 |
| `autoEnsure` | `true` | 自动创建 segment |
| `warmup` | `true` | 启动预热 |
| `fetchTimeout` | `2s` | 拉取超时 |
| `waitTimeout` | `200ms` | 等待预取超时 |
| `prefetchRemainingRatio` | `0.2` | 预取阈值比例 |
| `dynamicStep` | `true` | 动态步长 |
| `targetDuration` | `15m` | 步长目标间隔 |
| `maxPrefetchWorkers` | `8` | 并发预取上限 |

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
    idgen    idgen.Interface
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

## 工作原理

### 号段模式架构

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
- **并发安全**：segment 内部使用 `atomic.Pointer[segmentState]` 和 `atomic.Int64` 实现无锁分配。
- **自动 Ensure**：`autoEnsure = true` 时，启动阶段自动在数据库创建缺失的 segment 定义。

### 号段数据结构

```go
type segment struct {
    ptr atomic.Pointer[segmentState]  // 原子指针，支持无锁状态切换
}

type segmentState struct {
    start  int64       // 区间起始
    end    int64       // 区间结束（不含）
    cursor atomic.Int64 // 当前游标
}
```

每次调用 `next()` 时，通过 `cursor.Add(1) - 1` 原子递增获取 ID。当 `cursor >= end` 时号段耗尽，触发切换或预取。

### 双缓冲切换流程

```text
current 耗尽
    |
    v
next 是否就绪？ --是--> current = next, next 清空, currentVersion++
    |
    否
    |
    v
fetchSegmentLocked() 从数据库拉取新段
    |
    v
current = 新段, next 清空
```

`currentVersion` 原子计数器确保预取线程不会覆盖已切换的状态。

### 动态步长算法

当 `dynamicStep = true` 时，步长根据号段消耗速度自动调整：

```go
// internal/component/idgen/step.go
func (p stepPolicy) next(current, minStep, maxStep int64, elapsed time.Duration) int64 {
    if elapsed < p.targetDuration {
        return min(current*2, maxStep)   // 消耗快 -> 步长翻倍
    }
    if elapsed >= 2*p.targetDuration {
        return max(current/2, minStep)   // 消耗慢 -> 步长减半
    }
    return current                        // 适中 -> 不变
}
```

步长始终限制在 `[minStep, maxStep]` 范围内。服务端可通过 `SegmentConfig` 动态下发步长策略。

### 预取机制

当当前号段剩余量 <= `prefetchRemainingRatio`（默认 20%）时触发预取：

1. 启动后台 goroutine 从数据库加载下一个号段到 `next`。
2. 当 `current` 耗尽时，如果 `next` 已就绪，直接切换（零延迟）。
3. 如果 `next` 未就绪，等待最多 `waitTimeout`（200ms），超时后同步拉取。
4. 预取受 `maxPrefetchWorkers` 限制，避免过多并发数据库请求。

```go
func (g *segmentGenerator) maybePrefetch() {
    // 双重检查：已初始化且无预取线程运行
    // 检查 remaining <= currentLen * prefetchRemainingRatio
    // 通过 CompareAndSwap 抢占预取线程槽位
    // 异步执行 prefetch()
}
```

### 雪花模式位分配

```text
|  0  |    41 位时间戳    |  10 位机器 ID  |  12 位序列号  |
| --- | ----------------- | -------------- | ------------- |
|  1b |       41b         |      10b       |      12b      |
```

- 时间戳：毫秒级，epoch 从 2020 年 1 月 1 日起算，41 位可使用约 69 年。
- 机器 ID：由机器租约分配，最大 1023，支持最多 1024 个 worker。
- 序列号：同一毫秒内的自增序列，每毫秒最多 4096 个 ID。

### 性能特征

| 环境 | 典型 QPS | 说明 |
|------|----------|------|
| 开发环境 | ~50K | 单进程，本地数据库 |
| 生产环境 | ~200K | 多进程，优化数据库连接 |
| 高负载场景 | ~800K | 动态步长 + 大 step 配置 |

号段模式的性能瓶颈在数据库段分配而非进程内生成。通过调整 `step` 和 `dynamicStep`，可将数据库交互频率降至极低。

### Prometheus 指标

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|------|------|
| `idgen_generated_total` | Counter | component, name, operation, status | ID 生成计数 |
| `idgen_segment_fetch_total` | Counter | component, name, status | 号段拉取次数 |
| `idgen_segment_fetch_seconds` | Histogram | component, name | 号段拉取延迟 |
| `idgen_segment_remaining` | Gauge | component, name | 当前号段剩余量 |
| `idgen_segment_step` | Gauge | component, name | 当前有效步长 |
| `idgen_prefetch_total` | Counter | component, name, status | 预取尝试次数 |

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

### 号段耗尽或预取超时

**症状**：`ErrSegmentExhausted` 或 `ErrStoreUnavailable`。

**排查**：

- 数据库连接是否正常。
- `fetchTimeout` 是否过短（默认 2 秒）。
- segment 定义的 step 是否过小（高并发场景建议增大）。
- 检查 `Stats()` 中的 `SegmentFetchFails` 计数。

```go
stats, ok := comp.Stats("user_id")
if ok && stats.SegmentFetchFails > 0 {
    log.Warn("segment fetch failures detected",
        zap.Uint64("failures", stats.SegmentFetchFails),
        zap.String("lastError", stats.LastError))
}
```

### 动态步长调整不生效

**原因**：`dynamicStep = false` 或 `targetDuration` 配置不合理。

动态步长的行为：

- 消耗速度快于目标 -> 下次步长翻倍（不超过 `maxStep`）。
- 消耗速度慢于目标（超过 2 倍） -> 下次步长减半（不低于 `minStep`）。
- 消耗速度在目标范围内 -> 步长不变。

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

### 租约丢失后的 ID 唯一性

`LostPolicy = "degraded"` 模式下，租约丢失后继续生成号段 ID（号段 ID 不依赖机器 ID），但 Snowflake 模式无法继续生成。

### 组件关闭

```go
// 优雅关闭：停止预取线程，等待后台 goroutine 结束
err := comp.Stop()
```

## 参考链接

- `internal/component/idgen/component.go` -- Component 定义与生命周期
- `internal/component/idgen/config.go` -- Config 结构体与默认值
- `internal/component/idgen/generator.go` -- segmentGenerator 实现（Next、Reserve、prefetch）
- `internal/component/idgen/segment.go` -- 无锁号段数据结构（atomic.Pointer + atomic.Int64）
- `internal/component/idgen/step.go` -- 动态步长策略（stepPolicy）
- `internal/component/idgen/interface.go` -- Interface、Generator、SegmentStore 接口定义
- `internal/component/idgen/idgetter.go` -- IDGetter 适配器（xflake.Geter 形态）
- `internal/component/idgen/store/gormstore/store.go` -- GORM 号段存储实现
- `internal/component/idgen/memory_store.go` -- 内存号段存储（测试用）
