# 性能优化

EgoAdmin 提供多层性能优化策略，涵盖数据库连接池、缓存、并发控制、gRPC 调优和内存优化。

## 概述

EgoAdmin 的性能优化分为五个层面：

| 层面 | 机制 | 关键配置 |
|------|------|----------|
| 数据库 | 连接池调优、索引优化 | `client.mysql` |
| 缓存 | JetCache 本地+远程双层缓存 | `client.jetcache` |
| 并发 | Redis 分布式锁、异步队列 | `client.asyncq` |
| gRPC | 连接池、超时、重试 | `client.grpc.*` |
| 内存 | 预分配、避免拷贝 | 编译器 lint 规则 |

## 核心用法

### 数据库连接池

GORM 的连接池参数直接影响数据库吞吐量：

```bash
# 查看当前连接数
mysql> SHOW STATUS LIKE 'Threads_connected';

# 查看连接池使用情况
mysql> SHOW PROCESSLIST;
```

### JetCache 缓存

EgoAdmin 使用 JetCache 实现双层缓存（本地内存 + Redis），配置项控制 TTL、本地缓存大小和刷新策略：

```toml
[client.jetcache]
name = "default"
# 远程缓存（Redis）过期时间
remoteExpiry = "1h0m0s"
# 本地缓存条目数
localSize = 256
# 本地缓存过期时间
localExpiry = "1m0s"
# 异步刷新间隔
refreshDuration = "2m0s"
# 停止刷新的窗口
stopRefreshAfter = "1h0m0s"
# 缓存未命中时的短 TTL
notFoundExpiry = "1m0s"
# 启用缓存指标
enableMetrics = true
# 禁用本地缓存同步（多实例场景按需开启）
enableSyncLocal = false
# 序列化编解码器
codec = "msgpack"
```

::: tip 缓存层级设计
本地缓存（local）提供微秒级读取，适合高频读取但一致性要求不极高的数据。远程缓存（Redis）提供跨实例一致性。JetCache 默认先查本地、再查远程、最后查数据库。
:::

### 异步任务队列

EgoAdmin 使用 asyncq 基于 Redis 的异步任务队列：

```toml
[client.asyncq]
redisAddr = "127.0.0.1:6379"
redisPassword = "egoadmin"
redisDB = 0
enableClient = true
enableServer = false
# 工作协程数
concurrency = 10
# 队列优先级配置
queues = { critical = 6, default = 3, low = 1 }
# 最大重试次数
maxRetry = 3
# 重试延迟策略
retryDelayFunc = "exponential"
# 单任务超时
taskTimeout = "30s"
# 慢日志阈值
slowLogThreshold = "1s"
```

### 基准测试

使用 Go 内置 benchmark 工具测量热点路径：

```bash
# 运行所有基准测试
go test -bench=. ./...

# 运行指定包的基准测试
go test -bench=. -benchmem ./internal/app/gateway/application/...

# 生成 CPU profile
go test -bench=. -cpuprofile=cpu.prof ./internal/app/user/domain/...

# 生成内存 profile
go test -bench=. -memprofile=mem.prof ./...

# 分析 profile
go tool pprof cpu.prof
go tool pprof mem.prof
```

## 配置示例

### 数据库连接池

```toml
# configs/gateway/config.toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=1s&timeout=1s&writeTimeout=3s"
```

通过环境变量覆盖连接池参数：

```bash
export EGOADMIN_CLIENT_MYSQL_MAXOPENCONNS=100
export EGOADMIN_CLIENT_MYSQL_MAXIDLECONNS=10
export EGOADMIN_CLIENT_MYSQL_CONNMAXLIFETIME="1h"
```

连接池参数说明：

| 参数 | 推荐值 | 说明 |
|------|--------|------|
| maxOpenConns | 50-100 | 最大打开连接数，根据 MySQL max_connections 调整 |
| maxIdleConns | 10-20 | 最大空闲连接数，设为 maxOpenConns 的 10-20% |
| connMaxLifetime | 1h | 连接最大存活时间，避免使用服务端已关闭的连接 |
| readTimeout | 1s | 读超时 |
| writeTimeout | 3s | 写超时 |

::: warning 连接泄漏
如果 `maxOpenConns` 设为 0（不限制），高并发场景下可能耗尽 MySQL 连接。务必设置合理上限。
:::

### JetCache 完整配置

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

### gRPC 客户端调优

```toml
# configs/gateway/config.toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"
```

::: info 服务发现
gRPC 地址使用 etcd 服务发现格式 `etcd:///egoadmin-user`，EGO 框架自动解析为实际实例地址并维护连接池。
:::

### DSN 超时参数

MySQL DSN 中的超时参数对性能影响显著：

```bash
# 推荐的 DSN 参数
?readTimeout=1s&timeout=1s&writeTimeout=3s
```

| 参数 | 说明 | 推荐值 |
|------|------|--------|
| `timeout` | 连接建立超时 | 1s |
| `readTimeout` | 查询读取超时 | 1s |
| `writeTimeout` | 写入超时 | 3s |

## 实际应用

### 生产环境调优清单

```text
数据库
  ├── 设置合理的连接池上限
  ├── 确认慢查询日志已开启
  ├── 为高频查询字段添加索引
  └── 使用 EXPLAIN 分析关键 SQL

缓存
  ├── 为高频读取数据启用 JetCache
  ├── 合理设置 localExpiry 避免缓存雪崩
  ├── 使用 notFoundExpiry 缓存空结果
  └── 监控缓存命中率（enableMetrics = true）

gRPC
  ├── 根据网络延迟调整 readTimeout
  ├── 使用 etcd 服务发现实现负载均衡
  └── 避免在热路径中传递大数据

内存
  ├── 运行 prealloc linter 检查切片预分配
  ├── 使用 go test -bench -memprofile 分析分配热点
  └── 避免在循环中频繁分配临时对象
```

### 缓存预热

服务启动后，可以通过以下方式预热缓存：

```bash
# 启动服务后主动访问高频接口
curl -s http://localhost:9001/api/permission/menu
curl -s http://localhost:9001/api/user/info

# JetCache 的 refreshDuration 会自动刷新热点缓存
```

### 分布式锁使用模式

使用 Redis 分布式锁保护共享资源：

```text
请求 → 获取锁 → 执行操作 → 释放锁
         ↓ (获取失败)
       等待或返回错误
```

asyncq 的 `enableServer = true` 时，当前实例同时作为任务消费端。生产环境建议只在部分实例开启：

```toml
# 生产环境实例 A：消费者
[client.asyncq]
enableClient = true
enableServer = true
concurrency = 20

# 生产环境实例 B：仅提交
[client.asyncq]
enableClient = true
enableServer = false
```

## 工作原理

### JetCache 双层缓存流程

```text
请求 → 查询 local cache
         ├── 命中 → 返回（微秒级）
         └── 未命中 → 查询 remote cache (Redis)
                        ├── 命中 → 写入 local → 返回（毫秒级）
                        └── 未命中 → 查询数据库
                                       ├── 有数据 → 写入 remote + local → 返回
                                       └── 无数据 → 写入 notFoundExpiry 缓存 → 返回空
```

`refreshDuration` 配置启用后台异步刷新：缓存到期前由后台协程提前刷新，避免缓存击穿。

### 连接池工作原理

```text
应用请求 → 从池中获取空闲连接
             ├── 有空闲连接 → 使用 → 归还
             ├── 未达上限 → 创建新连接 → 使用 → 归还
             └── 已达上限 → 等待（直到 connMaxLifetime 释放旧连接）
```

`connMaxLifetime` 确保连接不会长期持有导致服务端超时断开。

### gRPC 连接复用

EGO 框架为每个 gRPC 客户端维护连接池，基于 etcd 服务发现自动实现负载均衡：

```text
gateway → gRPC client pool
            ├── channel 1 → user instance 1
            ├── channel 2 → user instance 2
            └── channel N → user instance N
```

## 常见问题

### 数据库连接数过高

```bash
# 检查 MySQL 当前连接数
mysql> SHOW STATUS LIKE 'Threads_connected';

# 检查每个服务的连接池配置
grep -A3 "client.mysql" configs/*/config.toml
```

调整连接池参数：

```toml
[client.mysql]
maxOpenConns = 50
maxIdleConns = 10
connMaxLifetime = "1h"
```

### 缓存命中率低

检查 JetCache 指标：

```bash
# 确认 enableMetrics 已开启
grep "enableMetrics" configs/*/config.toml

# 查看缓存命中统计（通过 EGO metrics 端点）
curl -s http://localhost:9003/metrics | grep jetcache
```

::: tip 调优建议
如果缓存命中率低于 80%，考虑增大 `localSize`、调整 `localExpiry`，或检查缓存 key 设计是否合理。
:::

### 内存占用过高

使用 pprof 定位内存分配热点：

```bash
# 采集堆内存 profile
go tool pprof -top -inuse_space http://localhost:9003/debug/pprof/heap

# 查看分配次数最多的函数
go tool pprof -top -alloc_objects http://localhost:9003/debug/pprof/heap

# 在浏览器中查看火焰图
go tool pprof -http=:8080 http://localhost:9003/debug/pprof/heap
```

### gRPC 调用超时

检查 gRPC 客户端配置和网络延迟：

```bash
# 测试 gRPC 服务端口连通性
grpcurl -plaintext etcd:2379 list

# 检查 etcd 注册的实例
etcdctl get --prefix /egoadmin/

# 调整超时
# 如果跨机房调用，适当增大 readTimeout 和 dialTimeout
```

### asyncq 任务积压

检查 Redis 队列长度和 worker 状态：

```bash
# 查看队列中待处理任务
redis-cli LLEN asyncq:default
redis-cli LLEN asyncq:critical
redis-cli LLEN asyncq:low

# 增加 worker 并发数
# [client.asyncq]
# concurrency = 20

# 确认 enableServer = true 在至少一个实例上
```

## 参考链接

- [GORM 连接池文档](https://gorm.io/docs/generic_interface.html#Connection-Pool)
- [Go pprof 文档](https://pkg.go.dev/runtime/pprof)
- [JetCache 配置说明](https://github.com/alibaba/jetcache)
- [Redis 分布式锁](https://redis.io/docs/manual/patterns/distributed-locks/)
- [监控与告警](/guide/zh-CN/deployment/monitoring)
- [Docker 容器化](/guide/zh-CN/deployment/docker)
