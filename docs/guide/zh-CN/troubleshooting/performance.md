# 性能问题排查

本页介绍如何诊断和解决 EgoAdmin 运行时的性能瓶颈，涵盖 CPU、内存、数据库、缓存和网络延迟等场景。

## 概述

EgoAdmin 的性能问题通常表现为请求延迟升高、CPU 占用异常、内存持续增长或数据库响应缓慢。排查性能问题的核心思路是先量化指标，再定位瓶颈，最后针对性优化。

排查路径推荐：

1. 使用 pprof 获取 CPU / 内存 / goroutine 快照。
2. 检查数据库慢查询日志和 GORM 调试输出。
3. 查看 Redis 慢日志和连接池状态。
4. 通过 Jaeger 分布式追踪定位跨服务延迟。

::: tip
EgoAdmin 通过 EGO 框架自动注册 pprof handler，默认暴露在 governor 端口（如 `:9103`、`:9203`）。开发环境可直接访问，生产环境应限制访问权限。
:::

## 核心用法

### CPU Profiling

当 CPU 占用异常升高时，采集 CPU profile 定位热点函数。

```bash
# 采集 30 秒 CPU profile（交互模式）
go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

# 采集后进入交互界面，常用命令：
# top         - 查看 CPU 占用最高的函数
# top20       - 查看前 20 个热点
# list <func> - 查看具体函数的 CPU 消耗
# web         - 生成可视化调用图（需要 graphviz）
```

在 pprof 交互界面中查看火焰图：

```bash
# 启动 HTTP 服务器查看火焰图
go tool pprof -http=:8080 http://localhost:9103/debug/pprof/profile?seconds=30
```

::: warning
CPU profiling 会产生 5%-10% 的额外开销。生产环境排查时建议限定 `seconds` 参数，避免长时间采集。
:::

### 堆内存分析

当内存持续增长时，对比不同时刻的 heap profile 定位泄漏。

```bash
# 获取当前堆内存 profile
go tool pprof http://localhost:9103/debug/pprof/heap

# 交互模式常用命令：
# top           - 查看内存分配最多的函数
# inuse_space   - 查看当前正在使用的内存
# alloc_space   - 查看累计分配的内存（含已 GC）
# inuse_objects - 查看当前存活对象数
```

对比两个时间点的 heap profile 以发现内存泄漏：

```bash
# 时间点 1：保存 heap profile
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.1

# 等待几分钟或执行一些操作后

# 时间点 2：保存 heap profile
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.2

# 对比差异
go tool pprof -base /tmp/heap.1 /tmp/heap.2
```

### Goroutine 分析

goroutine 数量异常增长通常意味着泄漏或阻塞。

```bash
# 查看 goroutine 堆栈
go tool pprof http://localhost:9103/debug/pprof/goroutine

# 也可以直接通过 HTTP 查看 goroutine 文本堆栈
curl http://localhost:9103/debug/pprof/goroutine?debug=1

# 查看 goroutine 数量概览
curl http://localhost:9103/debug/pprof/goroutine?debug=2
```

::: info
正常的 EgoAdmin 服务在空闲时 goroutine 数量通常在 50-200 之间。如果持续超过 1000，应排查是否有 goroutine 泄漏。
:::

### 火焰图

火焰图可以直观展示函数调用栈的时间分布。

```bash
# 启动交互式 Web 界面查看火焰图
go tool pprof -http=:8080 http://localhost:9103/debug/pprof/profile?seconds=30

# 浏览器打开 http://localhost:8080 后选择 Flame Graph 视图
# 也可以查看 Top、Graph、Peek 等视图
```

## 配置示例

### 数据库慢查询调试

GORM 提供 debug 模式，可以打印所有 SQL 语句及其执行时间。

```toml
# configs/user/config.toml
[client.mysql]
debug = true   # 开启 GORM 调试模式，打印所有 SQL
```

开启后日志输出示例：

```text
[0.234ms] [rows:10] SELECT * FROM `sys_user` WHERE `status` = 1 AND `deleted_at` IS NULL LIMIT 20
[1523.456ms] [rows:1] SELECT * FROM `sys_dept` WHERE id IN (1,2,3,4,5)
```

::: warning
`debug = true` 会在日志中输出所有 SQL 语句，生产环境务必关闭，避免日志量过大和敏感信息泄露。
:::

### Redis 慢查询调试

```toml
# configs/user/config.toml
[client.redis]
debug = true   # 开启 Redis 命令日志
```

开启后可以观察每条 Redis 命令的执行时间。配合 Redis 服务端慢日志阈值：

```bash
# 查看 Redis 当前慢日志阈值（微秒）
redis-cli CONFIG GET slowlog-log-slower-than

# 查看最近 10 条慢日志
redis-cli SLOWLOG GET 10
```

### 连接池配置

数据库连接池配置不当会导致连接耗尽或频繁创建连接。

```toml
# configs/user/config.toml
[client.mysql]
maxOpenConns = 100          # 最大打开连接数
maxIdleConns = 10           # 最大空闲连接数
connMaxLifetime = "3600s"   # 连接最大存活时间
connMaxIdleTime = "300s"    # 空闲连接最大存活时间
```

```toml
# configs/user/config.toml
[client.redis]
poolSize = 100              # Redis 连接池大小
minIdleConns = 10           # 最小空闲连接数
```

::: tip
连接池大小应根据实际并发量设置。`maxOpenConns` 过小会导致请求排队等待连接，过大则可能耗尽数据库连接限制。
:::

## 实际案例

### 案例一：N+1 查询导致接口响应慢

**症状**：用户列表接口响应时间超过 2 秒，但数据库查询本身很快。

**诊断**：

开启 GORM debug 模式后发现，查询用户列表时每条用户记录触发一次部门查询：

```text
[0.5ms] [rows:20] SELECT * FROM sys_user WHERE status=1 LIMIT 20
[0.3ms] [rows:1] SELECT * FROM sys_dept WHERE id = 1
[0.3ms] [rows:1] SELECT * FROM sys_dept WHERE id = 2
... (重复 20 次)
```

**解决**：

使用 `Preload` 或 `Joins` 一次加载关联数据：

```go
// 错误做法：循环查询（N+1）
for _, user := range users {
    db.First(&dept, user.DeptID)
}

// 正确做法：Preload 批量加载
var users []User
db.Preload("Dept").Where("status = ?", 1).Limit(20).Find(&users)

// 或使用 Joins（适合只需要部分字段）
db.Joins("Dept").Where("sys_user.status = ?", 1).Limit(20).Find(&users)
```

### 案例二：内存持续增长

**症状**：服务运行数小时后 RSS 内存持续增长，最终被 OOM killer 终止。

**诊断**：

```bash
# 对比两个时间点的 heap profile
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.before
# 等待 10 分钟
curl -s http://localhost:9103/debug/pprof/heap > /tmp/heap.after
go tool pprof -base /tmp/heap.before /tmp/heap.after
```

通过 pprof 发现某处缓存 map 只写不删，持续增长。

**解决**：

引入 LRU 缓存或设置 TTL，确保过期数据被清理：

```go
// 使用带有大小限制的缓存替代无限增长的 map
// 参考 samber/hot 或 github.com/hashicorp/golang-lru
```

### 案例三：Goroutine 泄漏

**症状**：服务运行一段时间后 goroutine 数量持续上升，日志中出现大量 goroutine 阻塞堆栈。

**诊断**：

```bash
# 查看 goroutine 数量趋势
curl -s http://localhost:9103/debug/pprof/goroutine?debug=1 | head -30

# 统计各函数创建的 goroutine 数量
curl -s http://localhost:9103/debug/pprof/goroutine?debug=2 | grep -c "^goroutine"
```

**解决**：

使用 `goleak` 在测试中检测 goroutine 泄漏：

```go
import "go.uber.org/goleak"

func TestSomething(t *testing.T) {
    defer goleak.VerifyNone(t)
    // test code
}
```

::: tip
`go.uber.org/goleak` 会在测试结束时检查是否有未退出的 goroutine。建议在 CI 中对所有包启用。
:::

### 案例四：Redis 热 key

**症状**：某些 Redis 命令延迟突增，Jaeger 追踪显示 Redis 操作耗时异常。

**诊断**：

```bash
# 查看 Redis 慢日志
redis-cli SLOWLOG GET 20

# 查看当前热点 key（需要 Redis 4.0+）
redis-cli --hotkeys
```

**解决**：

- 对热点 key 增加本地缓存，减少 Redis 访问频率。
- 拆分大 key 为多个小 key。
- 使用 pipeline 批量操作减少 RTT。

### 案例五：跨服务调用延迟

**症状**：某个接口整体响应慢，但单个服务内部处理很快。

**诊断**：

通过 Jaeger 查看分布式追踪，定位到 gateway -> user 的 gRPC 调用耗时过长：

```bash
# 打开 Jaeger UI
open http://localhost:16686

# 搜索 gateway 服务的 trace，找到耗时最长的 span
```

**解决**：

检查 gRPC 客户端配置：

```toml
# configs/gateway/config.toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"       # 读超时
dialTimeout = "3s"       # 连接超时
```

::: warning
`readTimeout` 和 `dialTimeout` 过大会导致请求长时间等待，过小则会误报超时。建议根据 P99 延迟设置合理值。
:::

## 工作原理

EgoAdmin 的性能监控基于 EGO 框架的 governor 机制。governor 是一个独立的 HTTP server，负责暴露 pprof、health check、metrics 等运维端点。

```text
请求流程：
Client → Gateway HTTP → Gateway gRPC → User gRPC → MySQL/Redis
         ↓ governor       ↓ governor
         :9101/:9001      :9103        :9203
         pprof/healthz    pprof/healthz
```

每个服务的 governor 端口定义在配置文件中：

```toml
[server.govern]
host = "0.0.0.0"
port = 9103
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
```

## 常见问题

### pprof 接口无法访问

**症状**：`curl http://localhost:9103/debug/pprof/` 返回连接拒绝。

**诊断**：

```bash
# 检查 governor 端口是否监听
netstat -tlnp | grep 9103

# 检查配置中 govern 端口
grep "govern" configs/user/config.toml
```

**解决**：

确认服务已启动且 governor 端口正确。gateway、user、idgen 各自使用独立的 governor 端口。

### 生产环境 pprof 安全

::: danger
生产环境的 pprof 端点包含敏感的运行时信息（内存内容、goroutine 堆栈），必须限制访问。建议：
- 仅通过内网访问 governor 端口。
- 使用防火墙或反向代理限制访问来源。
- 或在生产配置中禁用 governor。
:::

### golangci-lint 中的性能相关检查

运行 `make lint` 时，linter 会检查常见的性能问题：

```bash
# 常见的性能相关 lint 规则
# - prealloc：建议预分配 slice 容量
# - unconvert：不必要的类型转换
# - unparam：未使用的函数参数
make lint
```

## 参考链接

- [Go pprof 官方文档](https://pkg.go.dev/net/http/pprof)
- [Go Blog: Profiling Go Programs](https://go.dev/blog/pprof)
- [GORM 性能优化](https://gorm.io/docs/performance.html)
- [goleak - Goroutine 泄漏检测](https://github.com/uber-go/goleak)
- [Jaeger 分布式追踪](https://www.jaegertracing.io/)
- [Redis 慢日志文档](https://redis.io/docs/latest/commands/slowlog/)
