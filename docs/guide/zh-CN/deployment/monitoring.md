# 监控与告警

EgoAdmin 提供内置的健康检查、分布式追踪、性能分析和结构化日志能力，支撑生产环境的服务可观测性。

## 概述

EgoAdmin 的监控体系覆盖四个维度：

| 维度 | 工具 | 端点/配置 |
|------|------|-----------|
| 健康检查 | `/healthz` + `/readyz` | Governor HTTP 端口 |
| 分布式追踪 | OpenTelemetry + Jaeger | `trace.otlp` 配置 |
| 性能分析 | pprof | Governor `/debug/pprof/*` |
| 结构化日志 | EGO 日志框架 | `logs/ego.sys` |

## 核心用法

### 健康检查

EgoAdmin 每个服务暴露两个健康检查端点：

```text
GET /healthz    # 存活检查（liveness）
GET /readyz     # 就绪检查（readiness）
```

- `/healthz` 检查 MySQL 连接和 Redis 连接是否正常。
- `/readyz` 在数据库迁移完成、服务初始化就绪后才返回 200。

```go
// internal/platform/health/health.go
// 注册健康检查到 Governor HTTP 服务
health.Start(checkFn, govHTTPComponent)

// 迁移完成后标记服务就绪
healthOpts.Ready()

// 优雅退出时标记服务不可用（drain 阶段）
healthOpts.NotReady()
```

::: tip Kubernetes 探针
在 K8s 中，将 `/healthz` 配置为 livenessProbe，`/readyz` 配置为 readinessProbe。readinessProbe 失败时 K8s 会停止向该 Pod 发送新流量。
:::

### 分布式追踪

EgoAdmin 通过 OpenTelemetry 实现分布式追踪，支持将 trace 数据发送到 Jaeger 等后端。

```bash
# 启动 Jaeger（开发环境）
make dev-up
# 访问 Jaeger UI
# http://localhost:16686
```

查看指定服务的追踪数据：

```text
Jaeger UI → Service: egoadmin-gateway-local → Find Traces
```

### pprof 性能分析

Governor 端口暴露 pprof 端点，用于 CPU、堆内存和 goroutine 分析：

```bash
# 采集 30 秒 CPU profile
go tool pprof http://localhost:9003/debug/pprof/profile?seconds=30

# 查看堆内存分配
go tool pprof http://localhost:9003/debug/pprof/heap

# 查看 goroutine
go tool pprof http://localhost:9003/debug/pprof/goroutine

# 交互式 Web UI
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/profile?seconds=30
```

### 日志查看

EGO 框架将日志输出到文件和 stdout，格式为结构化 JSON：

```bash
# 查看系统日志
tail -f logs/ego.sys

# 过滤错误级别
grep '"level":"error"' logs/ego.sys

# 使用 jq 格式化
tail -1 logs/ego.sys | jq .
```

## 配置示例

### 追踪配置

```toml
# configs/gateway/config.toml
[trace]
# 服务名称，显示在 Jaeger UI 中
ServiceName = "egoadmin-gateway-local"
# 追踪类型：otlp（OpenTelemetry 协议）
OtelType = "otlp"
# 采样率：1.0 = 100% 采集，0.1 = 10% 采集
Fraction = 1.00

[trace.otlp]
# OTLP gRPC 接收端地址
Endpoint = "127.0.0.1:4317"
```

::: warning 生产环境采样率
生产环境建议将 `Fraction` 设为 `0.1` 或更低（10% 或更少的请求被追踪），避免大量追踪数据影响性能和存储。开发和测试环境可设为 `1.00`。
:::

### 环境变量覆盖

追踪配置可通过环境变量覆盖，适用于不同部署环境：

```bash
# 容器内使用 Jaeger 服务名
export EGOADMIN_TRACE_SERVICE_NAME="egoadmin-gateway-prod"
export EGOADMIN_TRACE_FRACTION="0.1"
export EGOADMIN_TRACE_OTLP_ENDPOINT="jaeger:4317"
```

### Governor 配置

Governor 端口提供运维端点，配置在 `server.governor` 中：

```toml
# configs/gateway/local-live.toml
[server.governor]
host = "0.0.0.0"
port = 9003
```

每个服务的 Governor 端口：

| 服务 | Governor 端口 |
|------|---------------|
| gateway | 9003 |
| user | 9103 |
| idgen | 9203 |

### 健康检查配置

Docker Compose 中的健康检查配置：

```yaml
# deploy/compose/app.yml
services:
  gateway:
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O - http://127.0.0.1:9001/readyz >/dev/null"]
      interval: 10s
      timeout: 5s
      retries: 12
      start_period: 60s
```

## 实际应用

### 生产监控架构

```text
                         ┌──────────┐
                         │  Jaeger  │
                         │  :16686  │
                         └────▲─────┘
                              │ OTLP gRPC
┌──────────┐  ┌──────────┐  ┌┴─────────┐
│ gateway  │──│   user   │──│  idgen   │
│ :9003    │  │ :9103    │  │ :9203    │
│ pprof    │  │ pprof    │  │ pprof    │
│ /healthz │  │ /healthz │  │ /healthz │
│ /readyz  │  │ /readyz  │  │ /readyz  │
└──────────┘  └──────────┘  └──────────┘
```

### 日常运维命令

```bash
# 检查所有服务健康状态
curl -s http://localhost:9001/healthz  # gateway
curl -s http://localhost:9101/healthz  # user
curl -s http://localhost:9201/healthz  # idgen

# 检查服务就绪状态
curl -s http://localhost:9001/readyz   # gateway

# 查看环境信息
curl -s http://localhost:9003/env      # gateway governor

# 采集 CPU profile
go tool pprof -top http://localhost:9003/debug/pprof/profile?seconds=30

# 查看 goroutine 泄漏
curl -s http://localhost:9003/debug/pprof/goroutine?debug=2
```

### E2E 测试中的监控

e2e 测试使用 health 端点等待服务就绪：

```text
test/compose 启动中间件
  → make run SERVICE=gateway
  → 轮询 /readyz 直到 200
  → 运行 e2e 测试
  → make run SERVICE=gateway 关闭（触发 NotReady）
```

## 工作原理

### 健康检查实现

```go
// health.Start 注册两个端点
func Start(fn CheckFn, c *egin.Component, opts ...Option) *Options {
    o := &Options{
        cf:    fn,
        ready: false,  // 初始状态为未就绪
    }

    // /readyz — 检查 ready 标志位
    c.GET("/readyz", o.readyz)

    // /healthz — 执行自定义检查函数（MySQL ping + Redis ping）
    c.GET("/healthz", o.healthz)

    return o
}
```

生命周期：

```text
启动 → ready=false → 迁移完成 → Ready() → ready=true → 开始接收流量
                                      ↓
                           停机信号 → NotReady() → ready=false → drain → exit
```

### 追踪数据流

```text
应用代码 → EGO trace 拦截器 → OpenTelemetry SDK → OTLP gRPC → Jaeger
              ↓
         注入 trace-id 到 HTTP/gRPC header
         跨服务传播 context
```

服务间调用时，trace context 通过 gRPC metadata 自动传播，无需手动处理。

### pprof 端点

EGO Governor 内置注册了标准 pprof handler：

| 端点 | 说明 |
|------|------|
| `/debug/pprof/` | 索引页 |
| `/debug/pprof/profile` | CPU profile（默认 30 秒） |
| `/debug/pprof/heap` | 堆内存分配 |
| `/debug/pprof/goroutine` | goroutine 列表 |
| `/debug/pprof/trace` | 执行追踪 |
| `/debug/pprof/allocs` | 累计内存分配 |
| `/debug/pprof/block` | 阻塞分析 |
| `/debug/pprof/mutex` | 锁竞争分析 |

### 日志结构

EGO 框架输出的结构化日志格式：

```json
{
  "level": "info",
  "ts": 1719600000.123,
  "caller": "server/egin.go:123",
  "msg": "access",
  "method": "POST",
  "path": "/api/user/login",
  "status": 200,
  "cost": 0.045,
  "traceId": "abc123def456"
}
```

## 常见问题

### /readyz 返回 502

服务尚未完成初始化或数据库迁移失败：

```bash
# 查看服务日志
docker compose logs gateway | grep -i "migrate\|error"

# 跳过迁移临时排查
EGOADMIN_ATLAS_MIGRATE=false docker compose up gateway
```

### Jaeger 中看不到追踪数据

检查追踪配置和 OTLP 端点连通性：

```bash
# 确认 Jaeger 正在运行
docker compose ps jaeger

# 测试 OTLP 端点
grpcurl -plaintext jaeger:4317 list

# 检查采样率
grep "Fraction" configs/*/config.toml
```

::: info OTLP 端口
Jaeger 的 OTLP gRPC 接收端口是 `4317`，不是 `16686`。`16686` 是 Jaeger UI 的 Web 端口。
:::

### pprof 端点无法访问

确认 Governor 端口未被防火墙阻挡：

```bash
# 检查端口监听
netstat -tlnp | grep 9003

# 或在 Docker 中端口未映射
# deploy/compose/app.yml 中添加：
# ports:
#   - "${GATEWAY_GOVERNOR_PORT:-9003}:9003"
```

### 日志文件过大

配置日志轮转，或在开发环境禁用文件日志：

```bash
# 查看当前日志大小
du -sh logs/ego.sys

# 清理旧日志
find logs/ -name "*.log" -mtime +7 -delete
```

### 追踪数据占用过多内存

降低采样率以减少追踪数据量：

```toml
[trace]
# 只追踪 1% 的请求
Fraction = 0.01
```

或使用环境变量在运行时调整：

```bash
export EGOADMIN_TRACE_FRACTION="0.01"
```

## 参考链接

- [OpenTelemetry 文档](https://opentelemetry.io/docs/)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)
- [Go pprof 文档](https://pkg.go.dev/net/http/pprof)
- [Kubernetes 探针](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Docker 容器化](/guide/zh-CN/deployment/docker)
- [性能优化](/guide/zh-CN/deployment/performance)
