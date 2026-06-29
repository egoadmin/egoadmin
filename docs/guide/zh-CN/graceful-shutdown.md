# 优雅停机与生命周期

本页介绍 EgoAdmin 如何通过 `shutdown.Manager` 协调进程退出，确保连接 draining、资源有序关闭和健康检查联动。

## 概述

微服务进程收到 `SIGTERM` 或 `SIGINT` 后，如果直接退出，会导致正在处理的请求被中断、数据库连接泄漏、注册中心残留过期实例。EgoAdmin 通过 `internal/platform/shutdown` 包提供统一的停机编排，使每个服务遵循相同的生命周期语义。

`shutdown.Manager` 负责两件事：在 EGO 停止服务器**之前**将 readiness 标记为不可用并等待 drain 时间；在 EGO 停止服务器**之后**按注册的逆序关闭非服务器资源（数据库、Redis、gRPC 客户端等）。EGO 框架自身负责 HTTP/gRPC/governor 服务器的优雅关闭，`Manager` 不重复这部分逻辑。

每个服务在 `newApp()` 中完成以下步骤：启动关键组件、调用 `configureShutdown()` 注册关闭资源、通过 `opts.app.Serve()` 交给 EGO 管理服务器生命周期、最后标记 `health.Ready()`。这保证了只有全部就绪后才开始接收流量。

## 核心用法

### 在服务中接入 shutdown.Manager

典型的服务构建流程如下：

```go
func newApp(opts Options) (*App, error) {
    // 启动关键组件（idm、idgen 等）
    if opts.idm != nil {
        if err := opts.idm.Start(context.Background()); err != nil {
            cleanup()
            return nil, fmt.Errorf("start idgen machine manager: %w", err)
        }
    }
    if opts.idgen != nil {
        if err := opts.idgen.Start(); err != nil {
            cleanup()
            return nil, fmt.Errorf("start idgen: %w", err)
        }
    }

    // 注册关闭资源（必须在 Serve 之前）
    configureShutdown(opts)

    // 交给 EGO 管理服务器生命周期
    opts.app.Registry(opts.registry)
    opts.app.Serve(opts.http, opts.grpc, opts.govern)

    // 所有就绪检查通过后标记 ready
    opts.health.Ready()

    return &App{Ego: opts.app}, nil
}
```

### 注册关闭资源

在 `configureShutdown()` 中按依赖顺序注册，`Manager` 会按逆序关闭：

```go
func configureShutdown(opts Options) {
    if opts.shutdown == nil {
        return
    }
    opts.shutdown.RegisterCloser("config", opts.conf)
    opts.shutdown.RegisterRegistry(opts.registry)
    opts.shutdown.RegisterDB("mysql", opts.db)
    if opts.redis != nil {
        opts.shutdown.RegisterCloser("redis", opts.redis)
    }
    if opts.jetcache != nil {
        opts.shutdown.RegisterCloser("jetcache", opts.jetcache)
    }
    if opts.idgenClient != nil {
        opts.shutdown.RegisterCloser("idgen-grpc-client", opts.idgenClient)
    }
    if opts.idm != nil {
        opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
            return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
        })
    }
    if opts.idgen != nil {
        opts.shutdown.RegisterCloser("idgen", opts.idgen)
    }
    opts.shutdown.Bind(opts.app)
}
```

::: tip 注册顺序原则
- 先注册的资源后关闭。依赖其他资源的组件应该**后**注册。
- `registry` 应较早注册，这样它在其他资源关闭后才与注册中心断开。
- 数据库、Redis 等基础设施在依赖它们的 worker/gRPC 客户端之前注册。
:::

### 注册 API 参考

`shutdown.Manager` 提供以下注册方法：

| 方法 | 适用场景 | 说明 |
|------|----------|------|
| `Register(name, CloseFunc)` | 自定义关闭逻辑 | 接收 `context.Context`，可利用 `closeTimeout` |
| `RegisterCloser(name, Closer)` | 实现 `Close() error` 的组件 | 自动包装为 `CloseFunc`，忽略 context |
| `RegisterRegistry(registry)` | etcd 注册中心 | 确保摘除注册在其他资源关闭之后 |
| `RegisterDB(name, db)` | `egorm.Component` 等数据库 | 内部调用 `sql.DB.Close()` |

所有注册方法都支持 `nil` 安全——传入 `nil` 时不注册任何内容，也不会 panic。

### 各服务 configureShutdown 对比

三个服务的关闭注册存在差异，反映了各自的资源拓扑：

| 资源 | gateway | user | idgen |
|------|---------|------|-------|
| config | Yes | Yes | Yes |
| registry | Yes | Yes | Yes |
| mysql | Yes | Yes | Yes |
| redis | 条件 | 条件 | -- |
| jetcache | -- | 条件 | -- |
| user-grpc-client | 条件 | -- | -- |
| idgen-grpc-client | 条件 | 条件 | -- |
| idgen-machine-lease | 条件 | 条件 | -- |
| idgen | 条件 | 条件 | 条件 |

"条件" 表示组件不为 `nil` 时才注册。这确保了不同部署拓扑（如无 Redis 模式）下的正确性。

## 配置示例

### TOML 配置

```toml
[app.shutdown]
# EGO 整体停止超时（包含 drain + 服务器关闭 + 资源关闭）
stopTimeout = "20s"

# readiness 标记为不可用后，等待 drain 的时间
# 此期间新请求会被负载均衡器摘除，存量请求有机会完成
drainTimeout = "2s"

# 每个非服务器资源的关闭超时
closeTimeout = "5s"
```

### 环境变量覆盖

配置值可通过 `EGOADMIN_*` 前缀的环境变量覆盖，参见 [运行时配置](./configuration.md)。

### 配置字段说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `stopTimeout` | duration | `20s` | EGO 框架的总停止超时，包含 drain 和所有清理 |
| `drainTimeout` | duration | `2s` | readiness 置为 false 后到开始停止服务器之间的等待时间 |
| `closeTimeout` | duration | `5s` | `afterStop` 阶段每个资源关闭的单次超时 |

::: warning 超时归一化
如果 `drainTimeout` 或 `closeTimeout` 大于 `stopTimeout`，会被自动截断为 `stopTimeout` 的值。配置为负数时会被修正为默认值。
:::

## 实战示例

### 为新组件添加 Close 支持

任何实现了 `Close() error` 接口的组件都可以直接注册：

```go
// 自定义组件
type CacheClient struct {
    // ...
}

func (c *CacheClient) Close() error {
    return c.conn.Close()
}

// 在 configureShutdown 中注册
opts.shutdown.RegisterCloser("my-cache", opts.cacheClient)
```

也可以用 `RegisterFunc` 注册任意关闭逻辑：

```go
opts.shutdown.Register("my-worker-pool", func(ctx context.Context) error {
    opts.workerPool.Shutdown()
    return opts.workerPool.WaitTimeout(ctx)
})
```

### IDGen 机器租约的特殊处理

IDGen 服务在多实例部署时通过数据库行锁分配机器编号。停机时如果主动释放租约，可能与 idgen 服务自身正在停止的路径产生竞争。因此 shutdown 路径使用 `StopWithoutRelease`，仅停止本地续租，不调用远程释放接口。租约 TTL 会在超时后自然失效。

```go
if opts.idm != nil {
    opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
        return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
    })
}
```

`StopMachineLeaseBestEffort` 的行为：

1. 如果 manager 实现了 `StopWithoutRelease`，则调用它（不释放远程租约）。
2. 否则回退到普通 `Stop`。
3. 对于预期的停机错误（`ErrStoreUnavailable`、`ErrMachineLeaseLost`、`context.Canceled`、`context.DeadlineExceeded`），静默处理不返回错误。

::: warning make run 全停场景
本地 `make run` 同时停止 idgen、user 和 gateway。idgen 先停会导致 user/gateway 的 machine lease 续租失败。使用 `StopWithoutRelease` 避免了这些噪音错误。
:::

## 工作原理

### 停机时序

```
进程收到 SIGTERM/SIGINT
    |
    v
EGO 框架调用 beforeStop hooks
    |
    v
shutdown.Manager.beforeStop()
    ├── health.NotReady()           // readiness 置为 false
    ├── 日志 "service marked not ready"
    └── sleep(drainTimeout)         // 等待负载均衡器摘除
    |
    v
EGO 框架停止所有服务器（HTTP/gRPC/governor）
    ├── 等待存量请求完成
    ├── 拒绝新连接
    └── 受 stopTimeout 约束
    |
    v
EGO 框架调用 afterStop hooks
    |
    v
shutdown.Manager.afterStop()
    ├── 遍历 closer 列表（逆序）
    ├── 每个资源：context.WithTimeout(closeTimeout) -> fn(ctx)
    ├── 日志每个资源的关闭结果
    └── errors.Join 聚合所有错误
    |
    v
elog.Flush()                       // 刷新日志缓冲
    |
    v
进程退出
```

### Bind 方法如何挂钩 EGO 生命周期

```go
func (m *Manager) Bind(app *ego.Ego) {
    ego.WithStopTimeout(m.config.StopTimeout)(app)
    ego.WithBeforeStopClean(m.beforeStop)(app)
    ego.WithAfterStopClean(m.afterStop, elog.DefaultLogger.Flush, elog.EgoLogger.Flush)(app)
}
```

`Bind` 将 `stopTimeout` 传递给 EGO，然后注册 `beforeStop` 和 `afterStop` 钩子。`afterStop` 中还包含了日志刷新，确保停机日志不丢失。

### 资源关闭的错误处理

`afterStop` 不会因为某个资源关闭失败而中断。所有错误通过 `errors.Join` 聚合后返回给 EGO 框架。每个资源关闭的错误都会单独记录 ERROR 日志。

### readiness 与健康检查

`health.Options` 提供两个 HTTP 端点：

| 端点 | 行为 |
|------|------|
| `/healthz` | 自定义 check 函数（如 ping DB/Redis），返回 200 或 502 |
| `/readyz` | 基于 `ready` 标志位，用于 Kubernetes readiness probe |

停机时 `/readyz` 立即返回 502，负载均衡器（如 Kubernetes Service）将实例从端点列表摘除。`drainTimeout` 给摘除生效留出传播时间。

### Wire 集成

`shutdown.Manager` 通过 Wire 依赖注入。Wire ProviderSet 定义在 `internal/platform/shutdown/provider.go`：

```go
var ProviderSet = wire.NewSet(
    NewConfig,    // 从 platform/config.Config 解析 [app.shutdown]
    NewLogger,    // 返回 elog.EgoLogger
    NewManager,   // 构造 *Manager
)
```

`NewConfig` 从平台配置中读取 `[app.shutdown]` 节点的 `stopTimeout`、`drainTimeout`、`closeTimeout` 字段，值为 Go duration 字符串（如 `"20s"`、`"500ms"`）。如果配置缺失或字段为空，使用默认值。

每个服务的 Wire injector 注入 `*shutdown.Manager`，然后在 `newApp` 中通过 `configureShutdown` 注册资源并调用 `Bind`。

### 测试覆盖

shutdown 包的核心测试用例：

| 测试 | 验证内容 |
|------|----------|
| `TestManagerBeforeStopMarksNotReadyAndDrains` | beforeStop 将 readiness 置为 false 并等待 drainTimeout |
| `TestManagerAfterStopClosesResourcesInReverseOrder` | afterStop 按注册逆序关闭资源 |
| `TestManagerAfterStopAggregatesCloseErrors` | 多个资源关闭失败时错误通过 errors.Join 聚合 |

health 包测试：

| 测试 | 验证内容 |
|------|----------|
| `Ready()` / `NotReady()` | 状态切换和并发安全 |
| `/readyz` 端点 | ready 返回 200，not ready 返回 502 |

idgen 包测试：

| 测试 | 验证内容 |
|------|----------|
| `TestStopMachineLeaseBestEffort` | nil manager 安全、正常停止 |
| `TestStopMachineLeaseBestEffortUsesStopWithoutRelease` | 优先调用 StopWithoutRelease |
| `TestMachineLeaseManager_StopWithoutReleaseKeepsRemoteLease` | 不调用远程释放 |

## 常见问题

### 为什么停机日志中出现 "service marked not ready"？

这是预期行为。`beforeStop` 在 drain 阶段开始时输出此日志，表示服务已标记为不可接收新流量。不是错误。

### 为什么 make run 停机时出现 machine lease 相关 ERROR？

如果 idgen 服务先于 user/gateway 停止，续租请求会失败。正常情况下 `StopMachineLeaseBestEffort` 会将这些错误视为预期并静默处理。如果仍然看到 ERROR 级别日志，检查：

1. 是否直接调用了 `Stop()` 而非 `StopWithoutRelease`。
2. `isExpectedMachineLeaseShutdownError` 中是否遗漏了新的错误类型。

### 资源注册顺序错误会导致什么？

如果先注册 gRPC 客户端再注册数据库，关闭时会先关闭数据库再关闭客户端。此时如果客户端在关闭过程中需要访问数据库（如记录日志），就会失败。解决方法：按依赖顺序注册，数据库、Redis 等基础设施先于业务客户端注册。

### drainTimeout 应该设多长？

取决于负载均衡器摘除实例的时间：

- Kubernetes：`preStop` hook 时间 + readiness probe 间隔 * failureThreshold，通常 5-15 秒足够。
- 直连模式：设为 0，因为没有外部摘除机制。
- 一般建议：`drainTimeout` 不超过 `stopTimeout` 的一半，为服务器关闭和资源清理留出时间。

### 如何测试停机行为？

```bash
# 单元测试：验证 drain 和逆序关闭
go test ./internal/platform/shutdown/...

# 单元测试：验证 health 状态切换
go test ./internal/platform/health/...

# 单元测试：验证 idgen 停机路径
go test ./internal/component/idgen/...

# 集成测试：启动服务后发送 SIGTERM
make run SERVICE=user &
sleep 5
kill -TERM $(pgrep egoadmin-user)
```

### 新增资源后忘记注册会怎样？

该资源在进程退出时不会被显式关闭。数据库连接池通常会在进程退出时由操作系统回收，但这可能导致连接残留在 MySQL 侧直到 wait_timeout 超时。gRPC 客户端不关闭可能导致对端看到连接异常断开。建议在 `configureShutdown` 中为每个非临时资源都注册关闭逻辑。

### closeTimeout 设太短会怎样？

如果某个资源（如数据库）的关闭在 `closeTimeout` 内未完成，`context.DeadlineExceeded` 会导致该资源关闭失败。`afterStop` 会记录 ERROR 日志并继续关闭下一个资源。设置合理的 `closeTimeout`（默认 5 秒）通常足够。

### stopTimeout 和 drainTimeout 的关系是什么？

`stopTimeout` 是 EGO 框架的总停止超时，`drainTimeout` 是其中的一个阶段。实际可用的服务器关闭 + 资源关闭时间约为 `stopTimeout - drainTimeout`。如果 `drainTimeout` 设得太大（接近 `stopTimeout`），服务器关闭和资源清理阶段可能被压缩甚至超时。

::: danger 避免的配置
不要将 `drainTimeout` 设为等于或大于 `stopTimeout`。虽然归一化逻辑会截断它，但这通常意味着配置意图有误。
:::

### 多实例部署时需要注意什么？

在 Kubernetes 滚动更新时，新旧实例会短暂共存。旧实例进入 drain 阶段后，`/readyz` 返回 502，但旧连接可能仍然有请求在飞行中。确保 `drainTimeout` 足够长，让所有存量请求完成。对于长耗时请求（如文件上传），需要配合 EGO 的 `stopTimeout` 一起调整。

### 如何自定义 health check 函数？

`health.Start` 的第一个参数是 `CheckFn`，返回 `true` 表示健康。典型实现：

```go
func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
    return health.Start(func() bool {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
        defer cancel()

        dbx, err := db.DB()
        if err != nil {
            return false
        }
        if err = dbx.PingContext(ctx); err != nil {
            return false
        }
        if _, err = rds.Ping(ctx); err != nil {
            return false
        }
        return true
    }, c)
}
```

健康检查函数被调用时，服务可能处于 ready 或 not ready 状态。`/healthz` 始终调用此函数，不受 readiness 标志影响。

## 参考链接

- `internal/platform/shutdown/manager.go` -- Manager 核心逻辑
- `internal/platform/shutdown/config.go` -- Config 定义与默认值
- `internal/platform/shutdown/provider.go` -- Wire ProviderSet
- `internal/platform/shutdown/manager_test.go` -- 单元测试
- `internal/platform/health/health.go` -- readiness 与 healthz 端点
- `internal/component/idgen/shutdown.go` -- StopMachineLeaseBestEffort
- `internal/component/idgen/machine_manager.go` -- StopWithoutRelease
- `internal/app/user/server/shutdown.go` -- user 服务 configureShutdown 示例
- `internal/app/gateway/server/shutdown.go` -- gateway 服务 configureShutdown 示例
- `internal/app/user/server/server.go` -- user 服务 newApp 生命周期
