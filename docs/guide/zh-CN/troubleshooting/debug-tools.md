# 调试工具使用

本页介绍 EgoAdmin 项目中可用的调试工具和技巧，涵盖 Make 命令、Go 调试、中间件调试、网络追踪、日志分析和健康检查。

## 概述

EgoAdmin 提供了一套从代码质量检查到生产问题排查的完整工具链。日常开发中，Make 命令是主要入口；深入排查时，可以使用 pprof、Delve、Redis CLI、MySQL CLI 等工具进行逐层诊断。

调试工具按用途可分为以下几类：

- **代码质量**：lint、type check、test。
- **运行时调试**：pprof、Delve debugger。
- **中间件调试**：Redis、MySQL、GORM。
- **网络与链路**：gRPC 反射、Jaeger、curl 测试。
- **健康与状态**：healthz、readyz、配置打印。

::: warning
pprof、gRPC 反射等调试端口仅在开发环境启用。生产环境应确保 governor 和 debug 端口不暴露到公网。
:::

## Make 命令

Make 是 EgoAdmin 的统一命令入口。所有常用操作都通过 `make <target>` 执行。

### 代码质量

```bash
# 运行全部 linter（gofmt、buf lint、golangci-lint）
make lint

# 格式化代码
make fmt

# 生成 proto / Go / Wire 代码
make gen
```

### 构建

```bash
# 构建全部服务
make build

# 构建单个服务
make build SERVICE=gateway
make build SERVICE=user
make build SERVICE=idgen
```

### 测试

```bash
# 运行 Go 单元测试
make test

# 运行 e2e 测试（需要所有服务和中间件运行）
make e2e E2E_TIMEOUT=20m
```

### 服务检查

```bash
# 检查服务 Wire 注入是否正确
make service.check SERVICE=user

# 查看服务完整解析后的配置
make service.config SERVICE=gateway
```

### 中间件管理

```bash
# 启动/停止全部中间件
make dev-up
make dev-down

# 控制单个中间件
make dev.up-one MIDDLEWARE=mysql-user
make dev.down-one MIDDLEWARE=redis-user
make dev.reset-one MIDDLEWARE=etcd
```

### 数据库迁移

```bash
# 创建新迁移
make migrate.new SERVICE=user NAME=add_field

# 验证迁移文件
make migrate.validate SERVICE=user

# 重新计算迁移 hash
make migrate.hash SERVICE=user

# 应用迁移
make migrate.apply SERVICE=user ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

::: tip
`make migrate.new` 会自动连接本地数据库并生成 diff SQL。确保 `make dev-up` 已完成且目标数据库存在。
:::

## Go 调试

### 运行特定测试

```bash
# 运行单个测试函数
go test -v -run TestXxx ./internal/app/user/...

# 运行特定包的测试
go test -v ./internal/app/user/application/...

# 带 race 检测运行
go test -race ./...

# 带覆盖率运行
go test -cover ./internal/app/user/...
```

::: tip
使用 `-race` 标志可以检测并发数据竞争。建议在 CI 中始终启用 race 检测。
:::

### pprof 性能分析

每个服务的 governor 端口暴露 `/debug/pprof/` 端点。各服务 governor 端口：

| 服务 | Governor 端口 |
|------|---------------|
| gateway | `9003` |
| user | `9103` |
| idgen | `9203` |

**CPU 分析**：

```bash
# 采集 30 秒 CPU profile
go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

# 进入 pprof 交互模式后常用命令
# (pprof) top 20
# (pprof) web
# (pprof) list funcName
```

**堆内存分析**：

```bash
# 查看当前堆内存分配
go tool pprof http://localhost:9103/debug/pprof/heap

# (pprof) top
# (pprof) top -inuse_space
# (pprof) list funcName
```

**Goroutine 转储**：

```bash
# 查看所有 goroutine 的调用栈
go tool pprof http://localhost:9103/debug/pprof/goroutine

# (pprof) top
# (pprof) list
```

**内存分配速率**：

```bash
# allocs 分析，查看分配热点
go tool pprof http://localhost:9103/debug/pprof/allocs
```

::: tip
使用 `go tool pprof -http=:8081 http://localhost:9103/debug/pprof/heap` 可以直接打开 Web UI 查看火焰图。
:::

### Delve 调试器

Delve (dlv) 是 Go 的标准调试器，支持断点、单步执行、变量查看。

```bash
# 以调试模式启动 user 服务
dlv debug ./cmd/user -- --config configs/user/config.toml

# 设置断点
(dlv) break internal/app/user/application/user_service.go:42

# 运行到断点
(dlv) continue

# 查看变量
(dlv) print req
(dlv) print user.Name

# 查看调用栈
(dlv) bt

# 单步执行
(dlv) next
(dlv) step
```

**远程调试**：

```bash
# 在目标机器上以 headless 模式启动
dlv debug ./cmd/user --headless --listen=:2345 -- --config configs/user/config.toml

# 从本地连接
dlv connect localhost:2345
```

::: warning
Delve 会暂停进程执行。不要在生产环境或 e2e 测试中使用 Delve 附加到运行中的服务。
:::

## Redis 调试

### 开启 Redis Debug 模式

在服务配置中启用 Redis debug 日志：

```toml
[client.redis]
debug = true
```

启用后，服务日志 `logs/ego.sys` 会记录每个 Redis 命令的执行时间和参数。

### Redis CLI

```bash
# 连接 Redis
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin

# 常用命令
127.0.0.1:6380> KEYS session:*        # 查看会话 key
127.0.0.1:6380> DBSIZE                # 查看 key 数量
127.0.0.1:6380> INFO memory           # 内存使用情况
127.0.0.1:6380> INFO clients          # 客户端连接数
```

### 实时监控

```bash
# 监控所有 Redis 命令（生产环境慎用，影响性能）
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin monitor
```

::: warning
`redis-cli monitor` 会打印所有命令，对性能有显著影响。仅在开发环境使用，且不要长时间运行。
:::

## 数据库调试

### GORM Debug 模式

在服务配置中启用 GORM debug 日志：

```toml
[client.mysql]
debug = true
```

启用后，每个 SQL 语句的执行时间、参数和返回行数都会记录到日志中。

### MySQL CLI

```bash
# 连接 MySQL
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin

# 切换数据库
mysql> USE egoadmin_user;

# 常用命令
mysql> SHOW TABLES;
mysql> SHOW PROCESSLIST;
mysql> EXPLAIN SELECT * FROM sys_user WHERE username = 'admin';
```

### 慢查询排查

```bash
# 查看 MySQL 慢查询日志
mysql> SHOW VARIABLES LIKE 'slow_query%';
mysql> SHOW VARIABLES LIKE 'long_query_time';
```

::: tip
开发环境的 MySQL 默认开启了 general log。查看容器日志可以看到所有 SQL 执行记录：`docker logs mysql-user`。
:::

### 迁移状态检查

```bash
# 验证迁移文件完整性
make migrate.hash SERVICE=user

# 查看迁移目录
ls -la atlas/migrations/user/

# 检查当前数据库 migration 版本
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin egoadmin_user -e "SELECT * FROM atlas_schema_revisions ORDER BY version DESC LIMIT 5;"
```

## 网络与 gRPC 调试

### Jaeger 链路追踪

EgoAdmin 集成了 OpenTelemetry，当 Jaeger 运行时可以查看完整请求链路：

```bash
# 启动 Jaeger（通过 dev-up 已包含）
make dev-up

# 访问 Jaeger UI
# http://localhost:16686
```

Jaeger 能帮助定位：

- 跨服务调用的耗时分布。
- 某个请求经过了哪些服务和中间件。
- gRPC 调用是否超时或失败。

::: tip
Jaeger OTLP 采集端口是 `4317`（gRPC）。确保服务配置中的 trace endpoint 指向此地址。
:::

### gRPC 反射

开发模式下 gRPC 服务默认启用反射，可以使用 `grpcurl` 测试：

```bash
# 列出所有服务
grpcurl -plaintext 127.0.0.1:9002 list

# 列出某个服务的方法
grpcurl -plaintext 127.0.0.1:9002 list user.v1.UserService

# 调用方法
grpcurl -plaintext -d '{"username": "admin"}' 127.0.0.1:9002 user.v1.UserService/GetUserList
```

### curl 测试 HTTP API

gateway 通过 protoc-gen-go-http 生成的 handler 直接处理 HTTP POST 请求，可以使用 curl 测试：

```bash
# 登录获取 token
curl -X POST http://localhost:9001/api/user.v1.UserService/Login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "123456"}'

# 使用 token 调用 API
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{}'
```

::: tip
gateway 的 HTTP 路由格式为 `/api/<package>.<Service>/<Method>`。请求体是 JSON 格式的 proto 消息。
:::

### etcd 服务发现调试

```bash
# 列出所有已注册的服务
etcdctl --endpoints=127.0.0.1:2379 get --prefix /egoadmin --keys-only

# 查看某个服务的详细信息
etcdctl --endpoints=127.0.0.1:2379 get /egoadmin-user --prefix

# 监听 key 变化
etcdctl --endpoints=127.0.0.1:2379 watch /egoadmin --prefix
```

## 日志分析

### EGO 结构化日志

EgoAdmin 使用 EGO 框架的日志系统，日志输出到 `logs/ego.sys`。

日志格式：

```text
2026-06-29 10:30:00 INF message key=value key2=value2
```

关键字段：

- `level`：日志级别（DBG/INF/WRN/ERR）。
- `comp`：组件名称（如 `server.http`、`client.grpc`、`client.redis`）。
- `access`：请求日志（当 access log 启用时）。

### 开启请求日志

在配置中启用 access log 可以看到每个请求的详细信息：

```toml
[server.http]
enableAccessInterceptorReq = true
enableAccessInterceptorRes = true
```

启用后，日志会记录每个请求的：

- 请求方法和路径。
- 请求参数。
- 响应状态码和 body。
- 耗时。

::: warning
在生产环境开启 access log 会增加日志量。建议仅在排查问题时临时开启。
:::

### 慢日志阈值

Redis 和消息队列组件支持慢日志配置：

```toml
[client.redis]
slowThreshold = "100ms"

[client.asyncq]
slowThreshold = "200ms"
```

超过阈值的操作会被标记为慢操作，方便定位性能瓶颈。

### 日志级别调整

开发时可以临时调低日志级别查看更详细的信息：

```bash
# 通过环境变量调整
export EGO_LOG_LEVEL=debug
make run SERVICE=user
```

## 健康检查

### HTTP 健康检查端点

每个服务暴露两个健康检查端点：

| 服务 | healthz | readyz |
|------|---------|--------|
| gateway | `http://localhost:9001/healthz` | `http://localhost:9001/readyz` |
| user | `http://localhost:9101/healthz` | `http://localhost:9101/readyz` |
| idgen | `http://localhost:9201/healthz` | `http://localhost:9201/readyz` |

```bash
# 快速检查
curl http://localhost:9001/healthz
curl http://localhost:9101/readyz
```

::: warning
healthz 和 readyz 走的是 HTTP 端口，不是 gRPC 端口。例如 user 服务的 HTTP 端口是 `9101`，gRPC 端口是 `9102`。
:::

### Governor 端点

Governor 端口提供运行时信息和调试入口：

| 服务 | Governor 端口 | 常用端点 |
|------|---------------|----------|
| gateway | `9003` | `/debug/pprof/`、`/health` |
| user | `9103` | `/debug/pprof/`、`/health` |
| idgen | `9203` | `/debug/pprof/`、`/health` |

```bash
# Governor 健康检查
curl http://localhost:9103/health

# 内部集成检查：MySQL ping、Redis ping
curl http://localhost:9103/health
```

Governor 的 `/health` 端点会检查所有下游依赖的连通性（MySQL、Redis、etcd），返回各组件状态。

## 环境变量

### EGOADMIN 前缀变量

EgoAdmin 支持通过 `EGOADMIN_*` 环境变量覆盖配置文件中的值：

```bash
# 服务名（用于 etcd 注册）
export EGO_NAME=egoadmin-user

# 数据库迁移 URL
export ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

### 查看默认配置

```bash
# 打印默认配置（确认所有可用的配置项）
go run ./cmd/user config print-default

# 等价于
make service.config SERVICE=user
```

::: tip
`config print-default` 输出的配置包含所有可用 key 和默认值，是排查配置问题的第一步。
:::

### 派生项目环境变量

派生项目可以使用 `egoadminctl init --env-prefix DEMOADMIN` 设置新的环境变量前缀。之后使用 `DEMOADMIN_*` 代替 `EGOADMIN_*`。

## 综合调试流程

遇到问题时，推荐按以下步骤排查：

```text
1. 检查服务健康
   curl http://localhost:9101/healthz

2. 检查下游依赖
   curl http://localhost:9103/health

3. 查看结构化日志
   tail -f logs/ego.sys | grep ERR

4. 启用 debug 模式
   配置 [client.mysql] debug = true / [client.redis] debug = true

5. 使用 pprof 定位性能瓶颈
   go tool pprof http://localhost:9103/debug/pprof/profile?seconds=30

6. 使用 Jaeger 查看链路
   http://localhost:16686

7. 使用 Delve 设置断点调试
   dlv debug ./cmd/user -- --config configs/user/config.toml
```

## 参考链接

- `configs/gateway/config.toml` — gateway 配置
- `configs/user/config.toml` — user 配置
- `configs/idgen/config.toml` — idgen 配置
- `test/compose/docker-compose.dev.yml` — 开发中间件编排
- `cmd/gateway/` — gateway 入口
- `cmd/user/` — user 入口
- `cmd/idgen/` — idgen 入口
- `internal/component/authsession/` — 认证会话组件
- `internal/platform/` — 平台基础设施
- `logs/ego.sys` — 结构化日志输出
