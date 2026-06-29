# 机器租约管理

EgoAdmin 的 IDGen 组件通过分布式机器租约为 Snowflake 算法提供唯一的 worker ID 协调。每个 IDGen 进程在启动时申请一个机器 ID 租约，通过后台续约维持有效性，进程退出时优雅释放。

## 概述

机器租约管理由 `ProcessMachineLeaseManager` 实现，负责一个进程的完整租约生命周期。系统支持两种分配器后端：

| 分配器 | 包路径 | 适用场景 |
|--------|--------|----------|
| Redis 分配器 | `machine/redis` | 中小规模部署，使用 Redis 分布式锁 |
| gRPC 分配器 | `machine/grpcallocator` | 大规模部署，通过 IDGen 服务远程管理 |

| 配置项 | 说明 |
|--------|------|
| `group` | 租约组名，同组共享机器 ID 空间 |
| `maxMachineID` | 最大机器 ID，范围 `[0, maxMachineID]` |
| `ttl` | 租约 TTL，过期后自动回收 |
| `renewInterval` | 续约间隔，必须小于 TTL |
| `lostPolicy` | 租约丢失策略：`fail_closed`（拒绝服务）或 `degraded`（降级） |

::: tip 默认策略
默认使用 `fail_closed` 策略。租约丢失后拒绝生成 ID，避免 Snowflake 机器 ID 冲突导致重复 ID。
:::

## 核心用法

### 租约生命周期

```text
进程启动
    |
    v
Start() -> AllocateLease（申请机器 ID）
    |
    v
[后台 cron 续约: cron.idgen.machine.renew]
    |
    v
Renew() -> 更新 TTL，续租成功
    |
    v
进程退出
    |
    v
Stop() / StopWithoutRelease() -> 释放租约 / 等待 TTL 过期
```

### 租约管理器接口

```go
type MachineLeaseManager interface {
    Start(ctx context.Context) error       // 启动，申请租约
    Stop(ctx context.Context) error        // 停止，释放远程租约
    Renew(ctx context.Context) error       // 续约
    Lease() (MachineLease, bool)           // 获取当前租约
    Health(ctx context.Context) error      // 健康检查
}
```

### 租约数据结构

```go
type MachineLease struct {
    Namespace     string        // 租约命名空间
    InstanceID    string        // 实例标识（hostname:pid 或稳定 ID）
    SessionID     string        // 会话 ID（随机生成，每次分配不同）
    MachineID     int           // 分配的机器 ID
    TTL           time.Duration // 租约 TTL
    RenewInterval time.Duration // 续约间隔
    ExpiresAt     time.Time     // 过期时间
}
```

### 优雅停机

推荐使用 `StopWithoutRelease` 停止续约但不调用远程释放接口。这避免了当所有服务同时停止时，IDGen 服务已经不可用导致的噪音错误：

```go
// 在 configureShutdown 中注册
opts.shutdown.RegisterFunc("idgen-machine", func(ctx context.Context) error {
    return opts.idm.StopWithoutRelease(ctx)
})
```

TTL 过期后租约自动回收，机器 ID 可被其他进程复用。

## 配置示例

### 基本配置

```toml
# 机器租约配置
[component.idgen.machine]
group = "egoadmin-local"           # 租约组名，同组共享机器 ID 空间
maxMachineID = 1023                # 最大机器 ID（范围 [0, 1023]）
ttl = "60s"                        # 租约 TTL
renewInterval = "10s"              # 续约间隔（必须小于 TTL）
renewTimeout = "5s"                # 续约超时
minRenewWindows = 5                # 最小续约窗口数
reallocateBackoff = "2s"           # 分配失败后重试退避
stableInstanceID = ""              # 稳定实例 ID，留空则使用 hostname:pid
lostPolicy = "fail_closed"         # 租约丢失策略
```

### 最小配置

```toml
[component.idgen.machine]
group = "egoadmin-local"
```

::: tip 默认值
未配置的字段使用 `DefaultMachineConfig()` 中的默认值：`maxMachineID = 1023`，`ttl = 60s`，`renewInterval = 10s`，`lostPolicy = "fail_closed"`。
:::

### 默认值速查

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `group` | `"default"` | 租约组名 |
| `maxMachineID` | `1023` | 最大机器 ID |
| `ttl` | `60s` | 租约 TTL |
| `renewInterval` | `10s`（自动计算） | 续约间隔，TTL/3 |
| `renewTimeout` | `5s` | 续约超时 |
| `minRenewWindows` | `5` | 最小续约窗口数 |
| `reallocateBackoff` | `2s` | 重分配退避 |
| `lostPolicy` | `"fail_closed"` | 租约丢失策略 |

### 实例 ID 策略

| 场景 | 推荐配置 |
|------|----------|
| 本地开发 | 留空 `stableInstanceID`，使用 `hostname:pid` |
| Docker Compose | 使用容器名 |
| Kubernetes | 使用 Pod 名称（`metadata.name`） |

## 实战示例

### Redis 分配器

Redis 分配器使用 Lua 脚本实现原子性的机器 ID 分配。进程启动时通过 `SET NX PX` 尝试获取一个空闲的机器 ID：

```go
import "github.com/egoadmin/egoadmin/internal/component/idgen/machine/redis"

// 创建 Redis 分配器
allocator := redis.New(redisClient, redis.WithKeyPrefix("idgen"))

// 创建租约管理器
manager, err := idgen.NewMachineLeaseManager(
    "component.idgen.machine",
    config,
    allocator,
    logger,
)

// 启动
err := manager.Start(ctx)
```

Redis 键结构：

```text
idgen:{namespace}:machine:id:{machineID}     -> "{instanceID}|{sessionID}"  (TTL)
idgen:{namespace}:machine:instance:{instanceID} -> "{machineID}|{sessionID}"  (TTL)
```

### gRPC 分配器

gRPC 分配器通过 IDGen 服务远程管理租约，适合大规模部署：

```go
import "github.com/egoadmin/egoadmin/internal/component/idgen/machine/grpcallocator"

// 通过 Wire 注入
allocator := grpcallocator.New(machineLeaseClient)

manager, err := idgen.NewMachineLeaseManager(
    "component.idgen.machine",
    config,
    allocator,
    logger,
)
```

### 后台续约

续约通过 EGO cron 任务（`cron.idgen.machine.renew`）自动执行：

```go
func NewMachineLeaseRenewCron(manager MachineLeaseManager) ecron.Ecron {
    return ecron.Load("cron.idgen.machine.renew").Build(
        ecron.WithJob(func(ctx context.Context) error {
            if manager == nil {
                return nil
            }
            return manager.Renew(ctx)
        }),
    )
}
```

续约逻辑：

1. 检查当前租约是否有效（未过期）。
2. 调用分配器的 `Renew` 方法续租。
3. 成功：更新本地 `ExpiresAt`，重置失败计数。
4. 失败：根据 `lostPolicy` 决定是否降级或标记租约丢失。
5. 续约失败超过 TTL：尝试重新分配新机器 ID。

## 工作原理

### Redis 分配器 Lua 脚本

分配脚本（`allocateScript`）的原子逻辑：

```lua
-- 1. 检查实例是否已有未过期的租约
local instanceKey = keyPrefix .. ":" .. namespace .. ":machine:instance:" .. instanceID
local existing = redis.call("GET", instanceKey)
if existing then
    -- 复用已有机器 ID，更新 sessionID
    redis.call("PSETEX", machineKey, ttlMillis, value)
    redis.call("PSETEX", instanceKey, ttlMillis, machineID .. "|" .. sessionID)
    return { existingID, sessionID }
end

-- 2. 遍历 [0, maxMachineID]，尝试 SET NX PX 获取空闲 ID
for id = 0, maxMachineID do
    local machineKey = keyPrefix .. ":" .. namespace .. ":machine:id:" .. id
    if redis.call("SET", machineKey, value, "PX", ttlMillis, "NX") then
        redis.call("PSETEX", instanceKey, ttlMillis, id .. "|" .. sessionID)
        return { id, sessionID }
    end
end

-- 3. 所有 ID 均被占用
return { -1, sessionID }
```

续约脚本（`renewScript`）校验 `instanceID|sessionID` 后更新 TTL；释放脚本（`releaseScript`）校验后删除键。所有操作均通过 Lua 脚本保证原子性。

### MySQL 分配器

IDGen 服务端使用 MySQL 管理机器租约（`idgen_machine_lease` 表），分配逻辑：

1. **复用已有租约**：查询同一 `instanceID` 的未过期租约，如有则复用。
2. **抢占过期 ID**：遍历 `[0, maxMachineID]`，使用 `SELECT ... FOR UPDATE` 锁定行。如已过期则更新为当前实例。
3. **创建新 ID**：如行不存在，直接创建。
4. **溢出**：所有 ID 均被占用且未过期，返回 `ErrMachineIDOverflow`。

```go
// 查找可复用的实例租约
existing, err := r.findReusableInstanceLease(ctx, req, now)
if existing != nil {
    // 更新 sessionID 和过期时间
    return modelToLease(existing), nil
}

// 遍历机器 ID
for id := int32(0); id <= int32(req.MaxMachineID); id++ {
    // SELECT ... FOR UPDATE
    row := machineLeaseModel{}
    db.Clauses(clause.Locking{Strength: "UPDATE"}).
        Where("namespace = ? AND machine_id = ?", req.Namespace, id).
        First(&row)

    if row.ExpiresAt.After(now) {
        continue  // 未过期，跳过
    }
    // 抢占已过期的 ID
    row.InstanceID = req.InstanceID
    row.SessionID = sessionID
    row.ExpiresAt = now.Add(req.TTL)
    // UPDATE ...
    return modelToLease(&row), nil
}
return ErrMachineIDOverflow
```

### 续约错误处理

续约失败时的处理逻辑：

```text
Renew() 失败
    |
    v
ErrMachineLeaseLost？ ----是----> 标记租约丢失，尝试 reallocate
    |
    否
    |
    v
租约是否仍有效（未过期）？ --是--> 警告日志，保留当前租约
    |
    否
    |
    v
标记租约丢失，尝试 reallocate
```

关键设计：续约失败但租约未过期时，不会立即标记丢失。这避免了网络抖动导致的不必要降级。

### 退避重分配

当重新分配失败时，使用 `reallocateBackoff` 防止高频重试：

```go
func (m *ProcessMachineLeaseManager) reallocate(ctx context.Context) error {
    now := time.Now()
    if next := m.nextAllocateAt.Load(); next > 0 && now.Before(time.Unix(0, next)) {
        return ErrMachineLeaseLost  // 退避期内不重试
    }
    if err := m.allocate(ctx); err != nil {
        m.nextAllocateAt.Store(now.Add(m.config.ReallocateBackoff).UnixNano())
        return err
    }
    return nil
}
```

### 冲突解决

当租约过期后，机器 ID 被释放供其他进程复用：

- **Redis**：键通过 TTL 自动过期，`SET NX PX` 保证原子性。
- **MySQL**：通过 `expires_at` 判断过期，`SELECT ... FOR UPDATE` 保证事务安全。

SessionID 用于区分同一机器 ID 的不同租约期。续约时校验 `sessionID`，不匹配则认为租约已被抢占。

### TTL 与续约窗口

配置校验规则确保续约窗口足够：

```text
TTL >= (minRenewWindows + 1) * renewInterval
```

例如：`ttl = 60s`，`renewInterval = 10s`，`minRenewWindows = 5`：
- TTL 覆盖 6 次续约机会（60s / 10s = 6）。
- 至少 5 次续约窗口（`minRenewWindows = 5`）。
- 即使连续 5 次续约失败，第 6 次仍有机会续约成功。

### 失败策略

| 策略 | 行为 | 适用场景 |
|------|------|----------|
| `fail_closed`（默认） | 租约丢失后拒绝生成 ID | 强一致性要求 |
| `degraded` | 租约丢失后继续使用已分配的号段 | 可用性优先 |

::: warning fail_closed 是默认策略
在 `fail_closed` 模式下，租约丢失会导致所有 `Next()` 和 `Reserve()` 调用返回 `ErrMachineLeaseLost`。这是有意设计，确保 Snowflake ID 的唯一性。
:::

### 过期租约清理

IDGen 服务端通过定时任务清理过期的机器租约记录：

```go
const (
    machineCleanupCronName         = "cron.idgen.machine.cleanup"
    defaultMachineCleanupLimit     = 1000
    defaultMachineCleanupRetention = 7 * 24 * time.Hour  // 保留 7 天
)
```

可在配置中调整：

```toml
# 自定义清理策略
[idgen]
machineLeaseCleanupRetention = "72h"
machineLeaseCleanupLimit = 500
```

### Prometheus 指标

| 指标名称 | 类型 | 标签 | 说明 |
|---------|------|------|------|
| `idgen_machine_lease_renew_total` | Counter | component, status | 续约操作计数 |
| `idgen_machine_lease_status` | Gauge | component | 租约状态（1=有效, 0=丢失） |
| `idgen_health_status` | Gauge | component | 组件健康状态 |

## 常见问题

### 机器 ID 冲突

::: warning
确保每个 worker 进程拥有唯一的机器租约。
:::

**原因**：多个进程使用相同的 `stableInstanceID`，或 Redis/MySQL 中旧租约未正确过期。

**排查**：

```bash
# 检查 Redis 中的机器 ID 占用
redis-cli KEYS "idgen:*:machine:id:*"

# 检查 MySQL 中的活跃租约
SELECT namespace, machine_id, instance_id, session_id, expires_at
FROM idgen_machine_lease
WHERE expires_at > NOW()
ORDER BY namespace, machine_id;
```

**解决**：在容器环境中使用唯一的 `stableInstanceID`（如 Kubernetes Pod 名称）。

### 租约续约失败

**症状**：日志出现 `idgen machine lease lost`。

**排查**：

- Redis 或 MySQL 连接是否正常。
- 后台 cron 任务 `cron.idgen.machine.renew` 是否正常运行。
- `renewTimeout` 是否足够（默认 5 秒）。
- `renewInterval` 是否小于 TTL。

```bash
# 检查 Redis 连通性
redis-cli PING

# 检查 MySQL 连通性
mysql -h 127.0.0.1 -u egoadmin -e "SELECT 1"
```

### 租约丢失后 ID 唯一性

`LostPolicy = "degraded"` 模式下，租约丢失后继续生成号段 ID（号段 ID 不依赖机器 ID），但 Snowflake 模式无法继续生成。

如果需要强唯一性保证，使用默认的 `fail_closed` 策略。

### 时钟漂移

::: danger
Snowflake 算法对系统时钟敏感。如果发生时钟回拨，可能产生重复 ID。
:::

**排查**：

```bash
# 检查 NTP 同步状态
timedatectl status
ntpq -p
```

**缓解**：号段模式（EgoAdmin 默认）不受时钟漂移影响，因为它不依赖系统时钟生成 ID。

### JS 精度丢失

::: warning
Go 的 `int64` / `uint64` 超过 JavaScript `Number.MAX_SAFE_INTEGER`（2^53 - 1）时会丢失精度。
:::

**解决方案**：使用 idcodec 编码为字符串，或在 proto/JSON 中将 ID 字段定义为 `string`。

### 所有机器 ID 被占用

**症状**：`ErrMachineIDOverflow`。

**原因**：所有 `[0, maxMachineID]` 范围内的机器 ID 都被未过期的租约占用。

**排查**：

- 检查是否有僵尸进程持有租约（进程崩溃但 TTL 未过期）。
- 增大 `maxMachineID`（默认 1023）。
- 缩短 `ttl` 以加速过期回收。
- 检查 `CleanupExpired` 定时任务是否正常运行。

### 优雅停机模式

```go
// 推荐方式：使用 StopWithoutRelease
// 避免所有服务同时停止时，IDGen 服务已不可用导致的释放失败噪音
opts.shutdown.RegisterFunc("idgen-machine", func(ctx context.Context) error {
    return opts.idm.StopWithoutRelease(ctx)
})

// 如果需要立即释放（如独占测试环境）
err := machineManager.Stop(ctx)
```

`StopWithoutRelease` 停止续约但不调用远程释放接口。TTL 过期后租约自动回收。`Stop` 会尝试调用远程释放接口，适合独占环境的快速回收。

## 参考链接

- `internal/component/idgen/machine_manager.go` -- ProcessMachineLeaseManager 实现（Allocate、Renew、Release、StopWithoutRelease）
- `internal/component/idgen/machine_cron.go` -- ecron 续约定时任务
- `internal/component/idgen/shutdown.go` -- StopMachineLeaseBestEffort 优雅停机
- `internal/component/idgen/machine/redis/allocator.go` -- Redis 分配器实现
- `internal/component/idgen/machine/redis/scripts.go` -- Redis Lua 脚本（allocate、renew、release）
- `internal/component/idgen/machine/grpcallocator/allocator.go` -- gRPC 分配器实现
- `internal/component/idgen/interface.go` -- MachineAllocator、MachineLeaseManager 接口定义
- `internal/component/idgen/config.go` -- MachineConfig 结构体与默认值
- `internal/app/idgen/adapter/persistence/mysql/machine_repository.go` -- MySQL 机器租约仓库
- `internal/app/idgen/adapter/persistence/mysql/machine_model.go` -- 数据库模型
- `internal/app/idgen/server/cron.go` -- 过期租约清理定时任务
- `internal/platform/idgen/provider.go` -- platform 层 Wire 组装
