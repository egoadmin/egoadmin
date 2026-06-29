# 配置问题诊断

本页介绍 EgoAdmin 配置加载机制和常见配置错误的排查方法，帮助快速定位配置相关的启动失败和运行异常。

## 概述

EgoAdmin 使用 TOML 格式的配置文件，支持环境变量覆盖。每个服务（gateway、user、idgen）拥有独立的配置文件，部署时通过 `EGOADMIN_` 前缀的环境变量覆盖敏感信息和环境差异。

配置问题通常表现为：

- 服务启动失败，日志报 `unmarshal` 或 `missing key` 错误。
- 服务连接中间件失败（DSN 格式错误、地址错误）。
- 功能异常（JWT 签名不匹配、Redis 模式错误）。

::: tip
排查配置问题的第一步：运行 `make service.config SERVICE=<name>` 查看当前生效的完整配置。
:::

## 核心用法

### 配置加载顺序

EgoAdmin 的配置按以下优先级加载（从高到低）：

1. **环境变量**：`EGOADMIN_` 前缀的环境变量。
2. **配置文件**：`configs/<service>/config.toml`。
3. **默认值**：代码中的默认值。

这意味着环境变量始终覆盖配置文件中的同名配置，配置文件覆盖默认值。

```text
优先级：环境变量 > 配置文件 > 默认值
```

### 查看默认配置

```bash
# 通过 make 命令查看（推荐）
make service.config SERVICE=gateway
make service.config SERVICE=user
make service.config SERVICE=idgen

# 直接运行二进制查看
go run ./cmd/user config print-default
go run ./cmd/gateway config print-default
```

### 环境变量覆盖规则

环境变量前缀为 `EGOADMIN_`，配置路径转大写，层级用 `_` 分隔：

```toml
# 配置文件中的写法
[client.mysql]
dsn = "user:pass@tcp(127.0.0.1:3306)/db"
```

```bash
# 对应的环境变量
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db"
```

::: info
环境变量名将 TOML 配置路径转为全大写，用下划线连接。例如 `[server.grpc].host` 对应 `EGOADMIN_SERVER_GRPC_HOST`。
:::

### 配置文件位置

```text
configs/
├── gateway/config.toml       # gateway 服务配置
├── gateway/local-live.toml   # 本地环境覆盖配置
├── user/config.toml          # user 服务配置
├── user/local-live.toml      # 本地环境覆盖配置
├── idgen/config.toml         # idgen 服务配置
└── idgen/local-live.toml     # 本地环境覆盖配置
```

部署环境配置：

```text
deploy/configs/
├── gateway/config.toml
├── user/config.toml
└── idgen/config.toml
```

## 配置示例

### 常用环境变量覆盖

部署时通过环境变量覆盖敏感信息和环境差异：

```bash
# MySQL 连接地址（容器环境使用服务 DNS 名）
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"

# Redis 地址
export EGOADMIN_CLIENT_REDIS_ADDR="redis:6380"

# JWT 签名密钥（生产环境必须修改）
export EGOADMIN_APP_USER_JWTSIGNKEY="production-secret-key-at-least-32-chars"

# gRPC 服务器监听地址
export EGOADMIN_SERVER_GRPC_HOST="0.0.0.0"
```

::: warning
JWT 签名密钥 `jwtSignKey` 在生产和开发环境必须不同。生产环境通过环境变量覆盖，绝对不要将生产密钥提交到代码仓库。
:::

### 容器环境配置

容器环境中必须使用 Docker Compose 服务名，不能使用 `127.0.0.1`：

```toml
# 错误：容器内 127.0.0.1 指向容器自身，不是宿主机或其它容器
[client.mysql]
dsn = "user:pass@tcp(127.0.0.1:3306)/db"

# 正确：使用 Docker Compose 服务名
[client.mysql]
dsn = "user:pass@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

etcd 地址同理：

```toml
# 正确：使用 Docker Compose 服务名
[server.registry]
addrs = ["http://etcd:2379"]
```

### 运行时配置检查

如果 governor 已启用，可以通过 HTTP 端点查看运行时配置：

```bash
# 查看运行时配置（如果 governor 启用）
curl http://localhost:9103/debug/config
```

## 实际案例

### 案例一：DSN 格式错误

**症状**：服务启动后立即 panic，日志出现 `invalid DSN` 或 `default addr for network 'unknown'`。

**诊断**：

```bash
# 查看当前 DSN 配置
grep "dsn" configs/user/config.toml
```

**解决**：

MySQL DSN 必须遵循 Go `go-sql-driver/mysql` 格式：

```toml
# 格式：[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...]
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

常见 DSN 格式错误：

| 错误写法 | 正确写法 |
|----------|---------|
| `user:pass@127.0.0.1:3306/db` | `user:pass@tcp(127.0.0.1:3306)/db` |
| `user:pass@tcp(127.0.0.1:3306)db` | `user:pass@tcp(127.0.0.1:3306)/db` |
| `user:pass@tcp(:3306)/db` | `user:pass@tcp(127.0.0.1:3306)/db` |

### 案例二：缺少 etcd 地址

**症状**：服务启动后日志报 `no etcd endpoints` 或服务注册失败。

**诊断**：

```bash
grep "addrs" configs/user/config.toml
```

**解决**：

```toml
[server.registry]
addrs = ["http://127.0.0.1:2379"]   # 本地开发
# 或容器环境
addrs = ["http://etcd:2379"]        # Docker Compose
```

::: warning
`addrs` 必须是 TOML 数组格式，即使只有一个地址也要用方括号包裹。
:::

### 案例三：JWT 签名密钥过短

**症状**：服务启动后日志报 `key is too short` 或 JWT 签发/验证失败。

**诊断**：

```bash
grep "jwtSignKey" configs/user/config.toml
```

**解决**：

JWT 签名密钥长度必须满足算法要求（HS256 至少 32 字节）：

```toml
[app.user]
jwtSignKey = "at-least-32-bytes-long-secret-key-here"
```

::: danger
生产环境的 `jwtSignKey` 必须通过环境变量覆盖，使用高熵随机字符串，并确保 gateway 和 user 服务使用相同的密钥。
:::

### 案例四：Redis 模式不匹配

**症状**：Redis 操作报 `MOVED` 或 `CLUSTERDOWN` 错误。

**诊断**：

```bash
grep "mode\|addrs" configs/user/config.toml
```

**解决**：

Redis 模式必须与实际部署匹配：

```toml
# 单机模式（默认，开发环境常用）
[client.redis]
mode = "stub"
addrs = ["127.0.0.1:6380"]

# 集群模式
[client.redis]
mode = "cluster"
addrs = ["redis-node-1:6379", "redis-node-2:6379", "redis-node-3:6379"]
```

::: tip
本地 `make dev-up` 启动的 Redis 是单机模式，对应 `mode = "stub"`。如果配置为 `cluster` 模式连接单机 Redis，操作会报错。
:::

### 案例五：端口配置错误

**症状**：服务启动后其它服务连接超时或连接拒绝。

**诊断**：

```bash
# 确认服务实际监听端口
netstat -tlnp | grep ego

# 对照配置文件
grep "port" configs/gateway/config.toml
grep "port" configs/user/config.toml
```

**解决**：

EgoAdmin 默认端口分配：

| 服务 | HTTP | gRPC | Governor |
|------|------|------|----------|
| gateway | 9001 | 9002（通过 grpcEndpoint） | - |
| user | 9101 | 9102 | 9103 |
| idgen | 9201 | 9202 | 9203 |

gateway 的 gRPC 端口通过 `grpcEndpoint` 指定：

```toml
# configs/gateway/config.toml
[server.http]
grpcEndpoint = "127.0.0.1:9002"
```

### 案例六：配置验证与快速失败

**症状**：服务启动时直接退出，日志中显示配置校验失败。

EgoAdmin 在服务启动阶段会验证关键配置项。如果必填配置缺失或格式错误，服务会快速失败（fail fast），避免运行时出现不可预期的行为。

**诊断**：

```bash
# 查看启动日志中的配置校验错误
go run ./cmd/user 2>&1 | head -50
```

**解决**：

对照默认配置输出逐项检查。使用 `config print-default` 命令获取完整的配置模板。

## 常见问题

### 配置文件找不到

**症状**：服务启动报 `config file not found`。

**解决**：

```bash
# 确认配置文件存在
ls configs/user/config.toml
ls configs/gateway/config.toml

# 如果是容器环境，确认挂载路径正确
ls deploy/configs/user/config.toml
```

### 环境变量不生效

**症状**：设置环境变量后配置值未变化。

**诊断**：

```bash
# 确认环境变量名称正确
env | grep EGOADMIN

# 确认变量名格式（全大写，下划线分隔）
# [client.mysql].dsn → EGOADMIN_CLIENT_MYSQL_DSN
```

**解决**：

环境变量名必须严格匹配：前缀 `EGOADMIN_` + 配置路径全大写 + `_` 连接。大小写敏感。

### 配置热更新

::: warning
EgoAdmin 当前不支持配置热更新。修改配置文件后必须重启服务才能生效。生产环境推荐通过环境变量覆盖敏感配置，配置文件保持不变。
:::

### 多环境配置管理

建议通过 `local-live.toml` 覆盖开发环境差异：

```toml
# configs/user/local-live.toml（开发环境覆盖）
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

[client.redis]
addrs = ["127.0.0.1:6380"]
```

生产环境使用 `deploy/configs/` 下的配置文件和环境变量覆盖。

## 参考链接

- [EgoAdmin 运行时配置](/guide/zh-CN/configuration)
- [EgoAdmin 部署指南](/guide/zh-CN/testing-deployment)
- [TOML 语法规范](https://toml.io/)
- [Go go-sql-driver/mysql DSN 格式](https://github.com/go-sql-driver/mysql#dsn-data-source-name)
