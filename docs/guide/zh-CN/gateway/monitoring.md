# 监控与运维

Gateway 提供健康检查、就绪探针、OpenTelemetry 链路追踪、pprof 性能分析和优雅停机等运维能力。

## 概述

Gateway 的运维体系围绕三个维度构建：可观测性（链路追踪 + 指标）、健康状态（存活 + 就绪探针）和生命周期管理（优雅停机）。Governor 服务独立于业务 HTTP/gRPC 端口，专用于运维端点暴露。OpenTelemetry 通过 OTLP 协议将链路数据发送到 Jaeger 等后端。

```text
负载均衡器
  |-> GET :9001/healthz    存活探针（检查 MySQL + Redis）
  |-> GET :9001/readyz     就绪探针（migration 完成后才为 true）
  |-> :9003/debug/pprof/*  性能分析
  |-> :4317 OTLP           链路追踪数据
```

## 核心用法

### 健康检查（Liveness Probe）

Gateway 的健康检查在 HTTP 服务器上注册 `/healthz` 和 `/readyz` 两个端点：

```go
// internal/platform/health/health.go
func Start(fn CheckFn, c *egin.Component, opts ...Option) *Options {
    o := &Options{cf: fn, ready: false}
    c.GET("/readyz", o.readyz)
    c.GET("/healthz", o.healthz)
    return o
}
```

`/healthz` 存活探针检查 MySQL 和 Redis 是否可达：

```go
// internal/app/gateway/server/server.go
func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
    return health.Start(func() bool {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
        defer cancel()

        // 检查数据库
        dbx, err := db.DB()
        if err != nil {
            return false
        }
        err = dbx.PingContext(ctx)
        if err != nil {
            return false
        }

        // 检查 Redis
        _, err = rds.Ping(ctx)
        if err != nil {
            return false
        }

        return true
    }, c)
}
```

| 端点 | 状态码 | 含义 |
|------|--------|------|
| `/healthz` | 200 | MySQL 和 Redis 都正常 |
| `/healthz` | 502 | MySQL 或 Redis 不可用 |
| `/readyz` | 200 | 服务已就绪，可接受流量 |
| `/readyz` | 502 | 服务未就绪（migration 中或正在启动） |

### 就绪探针（Readiness Probe）

就绪状态默认为 `false`，只有在所有初始化步骤（包括数据库 migration）完成后才设为 `true`：

```go
// internal/app/gateway/server/server.go - newApp()
func newApp(opts Options) (*App, error) {
    // ... 初始化组件 ...

    // 同步 API 字典
    if err := opts.apiSrv.SyncFromCatalog(context.Background(), egoadmin.APICatalog); err != nil {
        return nil, err
    }

    // 校验权限契约
    if err := opts.permission.EnsurePermissionContract(context.Background()); err != nil {
        return nil, err
    }

    // 注册文件上传路由
    // ...

    // 初始化前端 SPA
    web.StartWithFS(egoadmin.FrontendAssets, opts.conf.Web(), opts.http)

    // 所有初始化完成后标记就绪
    opts.health.Ready()

    return &App{Ego: opts.app}, nil
}
```

::: info 就绪延迟
从进程启动到标记就绪之间的时间窗口，负载均衡器的 `/readyz` 探针会返回 502，新的请求不会被路由到此实例。这对滚动部署和零停机重启至关重要。
:::

就绪状态可动态切换：

```go
// 标记就绪（初始化完成后）
opts.health.Ready()

// 标记未就绪（停机 drain 阶段）
opts.health.NotReady()

// 查询当前状态
if opts.health.IsReady() {
    // 服务就绪
}
```

### OpenTelemetry 链路追踪

Gateway 通过 OpenTelemetry 将链路数据以 OTLP 协议发送到后端（如 Jaeger）：

```toml
# configs/gateway/config.toml

[trace]
ServiceName = "egoadmin-gateway-local"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:4317"
```

| 配置项 | 说明 |
|--------|------|
| `ServiceName` | 服务名，显示在 Jaeger UI 中 |
| `OtelType` | 传输协议，`otlp` 使用 OpenTelemetry Protocol |
| `Fraction` | 采样率，`1.00` 表示 100% 采样 |
| `Endpoint` | OTLP Collector 地址（gRPC 端口） |

链路追踪拦截器在 HTTP 和 gRPC 服务器中均已启用：

```toml
[server.http]
enableTraceInterceptor = true
```

追踪上下文通过 gRPC metadata 在服务间自动传播。Gateway 调用 user 服务时，trace parent span 自动传递，形成完整的分布式链路。

### Governor 运维服务

Governor 是独立于业务服务的运维 HTTP 服务器，暴露调试和管理端点：

```go
// internal/app/gateway/server/govern_server.go
func NewGovernServer(_ config.EgoReady) *egovernor.Component {
    return egovernor.Load("server.governor").Build()
}
```

Governor 默认端口为 9003，提供以下端点：

| 端点 | 说明 |
|------|------|
| `/health` | 健康检查（Governor 自身） |
| `/debug/pprof/` | Go pprof 性能分析入口 |
| `/debug/pprof/profile` | CPU profile（默认 30 秒） |
| `/debug/pprof/heap` | 堆内存 profile |
| `/debug/pprof/goroutine` | Goroutine dump |
| `/debug/pprof/trace` | 执行 trace |

### pprof 性能分析

在开发和排障时，可以通过 pprof 获取运行时性能数据：

```bash
# CPU 分析（默认采样 30 秒）
go tool pprof http://localhost:9003/debug/pprof/profile?seconds=30

# 堆内存分析
go tool pprof http://localhost:9003/debug/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:9003/debug/pprof/goroutine

# 查看当前 goroutine 数量
curl http://localhost:9003/debug/pprof/goroutine?debug=1

# 获取火焰图数据
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/profile
```

::: warning 生产环境 pprof
pprof 端点在生产环境中应限制访问。建议通过网络策略或反向代理仅允许内部网络访问 Governor 端口。
:::

### 优雅停机

Gateway 通过 `shutdown.Manager` 统一管理资源关闭顺序。停机流程确保在关闭连接前先标记服务未就绪，让负载均衡器停止发送新请求。

```go
// internal/app/gateway/server/shutdown.go
func configureShutdown(opts Options) {
    opts.shutdown.RegisterCloser("config", opts.conf)
    opts.shutdown.RegisterRegistry(opts.registry)     // 注销 etcd 注册
    opts.shutdown.RegisterDB("mysql", opts.db)        // 关闭 MySQL 连接池
    if opts.redis != nil {
        opts.shutdown.RegisterCloser("redis", opts.redis)  // 关闭 Redis
    }
    if opts.userClient != nil {
        opts.shutdown.RegisterCloser("user-grpc-client", opts.userClient)  // 关闭 gRPC 客户端
    }
    if opts.idgenClient != nil {
        opts.shutdown.RegisterCloser("idgen-grpc-client", opts.idgenClient)
    }
    if opts.idm != nil {
        opts.shutdown.Register("idgen-machine-lease", func(ctx context.Context) error {
            return idgen.StopMachineLeaseBestEffort(ctx, opts.idm, 2*time.Second)
        })
    }
    opts.shutdown.Bind(opts.app)  // 绑定到 ego 生命周期
}
```

优雅停机执行顺序：

```text
1. 收到 SIGTERM/SIGINT 信号
2. 停止接受新的 HTTP/gRPC 连接
3. 等待正在处理的请求完成（带超时）
4. 注销 etcd 服务注册
5. 关闭 gRPC 客户端连接（user, idgen）
6. 释放 idgen 机器租约
7. 关闭 Redis 连接
8. 关闭 MySQL 连接池
9. 写入最终配置/状态
10. 进程退出
```

### Prometheus 指标

Gateway 通过 Ego 框架内置的指标拦截器收集请求指标：

```toml
[server.http]
enableMetricInterceptor = false   # 按需启用
enableAccessInterceptorReq = false  # 请求体日志
enableAccessInterceptorRes = false  # 响应体日志
```

可用指标包括：

| 指标 | 类型 | 说明 |
|------|------|------|
| 请求延迟 | Histogram | 按方法和状态码分类的请求耗时 |
| 请求计数 | Counter | 按方法和状态码分类的请求总数 |
| 活跃连接数 | Gauge | 当前正在处理的连接数 |

## 配置示例

### 完整追踪配置

```toml
# configs/gateway/config.toml

[trace]
ServiceName = "egoadmin-gateway-local"
OtelType = "otlp"
Fraction = 1.00

[trace.otlp]
Endpoint = "127.0.0.1:4317"
```

生产环境建议降低采样率：

```toml
[trace]
ServiceName = "egoadmin-gateway-prod"
OtelType = "otlp"
Fraction = 0.10  # 10% 采样
```

### Governor 配置

```toml
[server.governor]
host = "0.0.0.0"
port = 9003
enableLocalMainIP = false
```

### 服务发现注册配置

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"
```

| 配置项 | 说明 |
|--------|------|
| `serviceTTL` | 服务租约 TTL，超时未续约则从 etcd 注销 |
| `prefix` | 服务注册路径前缀 |

## 实战示例

### 搭建本地 Jaeger 链路追踪

```bash
# 启动 Jaeger（all-in-one 模式）
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# 启动 gateway
make run SERVICE=gateway

# 发送请求
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"page": 1, "pageSize": 10}'

# 打开 Jaeger UI 查看链路
open http://localhost:16686
```

### 健康检查脚本

```bash
#!/bin/bash
# 健康检查脚本，用于 Docker/K8s 探针

# 存活探针
healthz=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/healthz)
if [ "$healthz" != "200" ]; then
    echo "Liveness check failed: $healthz"
    exit 1
fi

# 就绪探针
readyz=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/readyz)
if [ "$readyz" != "200" ]; then
    echo "Readiness check failed: $readyz"
    exit 1
fi

echo "All checks passed"
```

### Docker Compose 中的探针配置

```yaml
# deploy/docker-compose.yaml
services:
  gateway:
    image: egoadmin/gateway:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9001/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 30s
    ports:
      - "9001:9001"
      - "9003:9003"
```

## 工作原理

### 健康检查状态机

```text
进程启动
  |
  v
ready = false  ->  /readyz 返回 502
  |                   (负载均衡器不发送新请求)
  v
Atlas migration 完成
  |
  v
API 字典同步完成
  |
  v
权限契约校验完成
  |
  v
前端 SPA 初始化完成
  |
  v
health.Ready()  ->  /readyz 返回 200
  |                   (开始接受新请求)
  v
收到 SIGTERM
  |
  v
health.NotReady()  ->  /readyz 返回 502
  |                      (负载均衡器停止发送新请求)
  v
等待活跃请求完成（drain）
  |
  v
关闭资源 -> 进程退出
```

链路追踪上下文通过 gRPC metadata 在服务间自动传播。Gateway HTTP 入口创建 root span，调用 user 服务时通过 gRPC metadata 传递 trace context，user 服务创建 child span。所有 span 最终上报到 OTLP Collector，在 Jaeger UI 中展示完整链路。

## 常见问题

### 链路追踪数据缺失

**现象**：Jaeger UI 中看不到 Gateway 的 trace 数据。

**排查步骤**：

1. 确认 Jaeger/OTLP Collector 正在运行：`curl http://localhost:16686/`
2. 检查 `trace.otlp.endpoint` 配置是否正确
3. 确认 `enableTraceInterceptor = true`
4. 检查 `Fraction` 是否为 `0`（0 表示不采样）
5. 验证网络连通性：`telnet 127.0.0.1 4317`

### 健康检查返回 502

**现象**：`/healthz` 或 `/readyz` 返回 502。

**排查步骤**：

对于 `/healthz`：

1. 检查 MySQL 是否可达：`mysqladmin ping`
2. 检查 Redis 是否可达：`redis-cli ping`
3. 检查连接超时配置（默认 10 秒）

对于 `/readyz`：

1. 检查数据库 migration 是否卡住
2. 检查 API 字典同步是否失败
3. 检查权限契约文件是否完整
4. 查看启动日志中的错误信息

### 优雅停机不生效

**现象**：发送 SIGTERM 后请求立即中断。

**排查步骤**：

1. 确认 shutdown.Manager 已正确注册所有资源
2. 检查是否有 `opts.shutdown.Bind(opts.app)` 调用
3. 确认负载均衡器的探针间隔合理（建议 10 秒以内）
4. 检查 drain 超时配置

### pprof 端点无法访问

**现象**：`curl http://localhost:9003/debug/pprof/` 无响应。

**排查步骤**：

1. 确认 Governor 端口配置正确
2. 检查防火墙是否允许 9003 端口
3. 确认 `NewGovernServer` 在 Wire 注入链中

### 服务注册自动注销

**现象**：服务运行中突然从 etcd 注销。

**排查步骤**：

1. 检查 etcd 集群健康状态
2. 确认 `serviceTTL` 配置合理（建议 10 秒以上）
3. 检查网络是否有抖动导致续约失败
4. 查看 etcd 客户端日志中的连接错误

## 参考链接

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Jaeger 官方文档](https://www.jaegertracing.io/docs/)
- [Go pprof 文档](https://pkg.go.dev/net/http/pprof)
- [Kubernetes 探针配置](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- 项目内相关源码：
  - `internal/platform/health/health.go` — 健康检查和就绪探针
  - `internal/app/gateway/server/govern_server.go` — Governor 运维服务
  - `internal/app/gateway/server/shutdown.go` — 优雅停机配置
  - `internal/app/gateway/server/server.go` — 服务初始化和就绪标记
  - `configs/gateway/config.toml` — 追踪和 Governor 配置
