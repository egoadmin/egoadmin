# 运行时配置

EgoAdmin 每个服务都有独立配置文件，部署时通过环境变量覆盖敏感信息和环境差异。

## 完整配置结构

EgoAdmin 使用 TOML 格式配置文件，所有配置都在以下顶层 section 下：

```text
[app]          # 应用业务配置
[server]       # 服务端配置（HTTP、gRPC、governor）
[client]       # 客户端配置（MySQL、Redis、gRPC clients）
[component]    # 组件配置（logincrypto、idgen、asyncq、jetcache、dtm）
[etcd]         # etcd 注册中心配置
[trace]        # 链路追踪配置
[jaeger]       # Jaeger 追踪配置
```

::: tip
配置字段的大写/小写遵循 TOML 惯例。section 层级使用点分隔，例如 `[server.http]` 等同于嵌套 `server → http`。
:::

## 配置文件

```text
configs/
├── gateway/config.toml
├── gateway/local-live.toml
├── user/config.toml
├── user/local-live.toml
├── idgen/config.toml
└── idgen/local-live.toml
```

部署配置位于：

```text
deploy/configs/
├── gateway/config.toml
├── user/config.toml
└── idgen/config.toml
```

## 查看默认配置

```bash
make service.config SERVICE=gateway
make service.config SERVICE=user
make service.config SERVICE=idgen
```

等价于：

```bash
go run ./cmd/user config print-default
```

## gateway 配置

```toml
[app.service]
name = "egoadmin-gateway"
platformName = "核心管理平台"
webPath = "/tmp/egoadmin/core/frontend/html"
bucketName = "egoadmin"

[app.web]
fileBaseUrl = ""
offlineOnPageLeave = false

[server.http]
host = "0.0.0.0"
port = 9001
mode = "release"
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:9002"
stripPrefix = "/api"

[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"
```

## user 配置

```toml
[app.service]
autoMigrate = false
name = "egoadmin-user"
platformName = "核心管理平台"
skipPermissionContractCheck = true
bucketName = "egoadmin"

[app.user]
adminPassword = "123456"
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
useCaptcha = false
multiLoginEnabled = true
maxLoginClient = 2
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660
```

## idgen 配置

```toml
[app.service]
autoMigrate = false
name = "egoadmin-idgen"
platformName = "核心管理平台"

[server.grpc]
name = "egoadmin-idgen"
host = "127.0.0.1"
port = 9202
```

## 组件配置

### logincrypto

登录加密组件，负责密码加密和登录态验证：

```toml
[component.logincrypto]
# 密码加密方式：bcrypt / sm4
algorithm = "bcrypt"
# bcrypt cost factor
bcryptCost = 10
```

### idgen

ID 生成器配置，使用 idcodec 编码：

```toml
[component.idgen]
# idcodec 密钥，用于编码/解码 ID
secret = "local-idgen-secret"
# ID 序列步长
step = 1000
# worker ID 来源：static / etcd
workerIdSource = "static"
workerId = 1
```

::: warning
生产环境 `secret` 必须通过环境变量注入，不要提交到仓库。
:::

### asyncq

异步队列组件配置：

```toml
[component.asyncq]
# 是否启用异步队列
enabled = true
# 并发 worker 数量
workers = 5
# 任务重试次数
maxRetry = 3
# 重试间隔（秒）
retryInterval = 30
```

### jetcache

缓存组件配置：

```toml
[component.jetcache]
# 是否启用本地缓存
local.enabled = true
# 本地缓存容量
local.capacity = 1000
# 本地缓存过期时间（秒）
local.ttl = 60
# 是否启用远程缓存（Redis）
remote.enabled = true
# 远程缓存默认过期时间（秒）
remote.ttl = 300
# 缓存 key 前缀
remote.prefix = "egoadmin"
```

### dtm

分布式事务组件配置：

```toml
[component.dtm]
enabled = false
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

## 服务端配置

### HTTP 服务

```toml
[server.http]
host = "0.0.0.0"
port = 9001
mode = "release"           # debug / release / test
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:9002"
stripPrefix = "/api"
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `host` | 监听地址 | `0.0.0.0` |
| `port` | 监听端口 | `9001` |
| `mode` | Gin 运行模式 | `release` |
| `ginRelativePath` | API 路由前缀 | `/api/*action` |
| `grpcEndpoint` | gRPC 反代端点 | `127.0.0.1:9002` |
| `stripPrefix` | 路由去除前缀 | `/api` |

### gRPC 服务

```toml
[server.grpc]
name = "egoadmin-user"
host = "0.0.0.0"
port = 9002
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `name` | 服务名，用于注册发现 | 服务名 |
| `host` | 监听地址 | `0.0.0.0` |
| `port` | 监听端口 | `9002` |

### Governor 服务

Governor 是内嵌的治理端口，提供健康检查和调试接口：

```toml
[server.governor]
host = "0.0.0.0"
port = 9003
enable = true
```

提供以下端点：

| 端点 | 说明 |
|------|------|
| `/healthz` | 存活检查 |
| `/readyz` | 就绪检查 |
| `/debug/pprof/` | Go pprof 性能分析 |
| `/config` | 当前生效配置（脱敏） |

## 客户端配置

### MySQL

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
maxOpenConns = 100
maxIdleConns = 10
connMaxLifetime = "3600s"
connMaxIdleTime = "600s"
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `dsn` | 数据库连接字符串 | 无（必填） |
| `maxOpenConns` | 最大打开连接数 | `100` |
| `maxIdleConns` | 最大空闲连接数 | `10` |
| `connMaxLifetime` | 连接最大存活时间 | `3600s` |
| `connMaxIdleTime` | 空闲连接最大存活时间 | `600s` |

### Redis

```toml
[client.redis]
addr = "127.0.0.1:6379"
password = ""
db = 0
maxRetries = 3
poolSize = 100
minIdleConns = 10
dialTimeout = "3s"
readTimeout = "3s"
writeTimeout = "3s"
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `addr` | Redis 地址 | `127.0.0.1:6379` |
| `password` | 密码 | 空 |
| `db` | 数据库编号 | `0` |
| `poolSize` | 连接池大小 | `100` |
| `minIdleConns` | 最小空闲连接数 | `10` |

### gRPC 客户端

```toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `addr` | 服务地址（支持 etcd 发现） | 无（必填） |
| `readTimeout` | 读超时 | `3s` |
| `dialTimeout` | 拨号超时 | `3s` |

::: tip
gRPC 客户端使用 etcd 服务发现时，地址格式为 `etcd:///<service-name>`。不需要配置 IP 地址。
:::

## 链路追踪配置

### Jaeger

```toml
[trace]
# 启用链路追踪
enable = true
# 采样率：0.0 ~ 1.0
sampleRate = 1.0

[jaeger]
# Jaeger agent 地址
host = "127.0.0.1"
port = 6831
# Jaeger collector 地址（可选，替代 agent）
collectorEndpoint = ""
```

生产部署：

```toml
[trace]
enable = true
sampleRate = 0.1

[jaeger]
host = "jaeger"
port = 6831
```

::: tip
生产环境建议采样率 0.1 ~ 0.5，避免高流量下追踪数据过大。
:::

## etcd 注册中心配置

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
```

容器内必须使用 Compose service DNS 名：

```toml
[etcd]
addrs = ["etcd:2379"]
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `addrs` | etcd 节点地址列表 | `["127.0.0.1:2379"]` |

## 数据库迁移配置

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

## 环境变量覆盖

EgoAdmin 支持 `EGOADMIN_` 前缀的环境变量覆盖配置。环境变量使用下划线分隔 TOML 层级。

**覆盖规则**：

| 配置路径 | 环境变量名 |
|----------|-----------|
| `app.service.name` | `EGOADMIN_APP_SERVICE_NAME` |
| `server.http.port` | `EGOADMIN_SERVER_HTTP_PORT` |
| `client.mysql.dsn` | `EGOADMIN_CLIENT_MYSQL_DSN` |
| `client.redis.password` | `EGOADMIN_CLIENT_REDIS_PASSWORD` |
| `etcd.addrs` | `EGOADMIN_ETCD_ADDRS` |
| `jaeger.host` | `EGOADMIN_JAEGER_HOST` |

**部署示例**：

```bash
# 基础环境
export EGOADMIN_APP_SERVICE_NAME=egoadmin-user
export EGOADMIN_SERVER_HTTP_PORT=9001
export EGOADMIN_SERVER_GRPC_PORT=9002

# 数据库
export EGOADMIN_CLIENT_MYSQL_DSN='egoadmin:password@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'

# Redis
export EGOADMIN_CLIENT_REDIS_ADDR=redis:6379
export EGOADMIN_CLIENT_REDIS_PASSWORD=secret

# etcd
export EGOADMIN_ETCD_ADDRS='["etcd:2379"]'

# Jaeger
export EGOADMIN_JAEGER_HOST=jaeger
export EGOADMIN_TRACE_ENABLE=true
```

Atlas 迁移配置使用独立环境变量前缀 `ATLAS_`：

```bash
export ATLAS_URL='mysql://user:password@mysql-user:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

::: tip
派生项目通过 `egoadminctl init --env-prefix DEMOADMIN` 设置新的环境变量前缀（例如 `DEMOADMIN_` 替代 `EGOADMIN_`）。
:::

**常用环境变量速查**：

| 场景 | 环境变量 | 示例值 |
|------|---------|--------|
| MySQL 连接 | `EGOADMIN_CLIENT_MYSQL_DSN` | `user:pass@tcp(host:3306)/db` |
| Redis 地址 | `EGOADMIN_CLIENT_REDIS_ADDR` | `redis:6379` |
| Redis 密码 | `EGOADMIN_CLIENT_REDIS_PASSWORD` | `secret` |
| etcd 地址 | `EGOADMIN_ETCD_ADDRS` | `["etcd:2379"]` |
| HTTP 端口 | `EGOADMIN_SERVER_HTTP_PORT` | `9001` |
| gRPC 端口 | `EGOADMIN_SERVER_GRPC_PORT` | `9002` |
| JWT 密钥 | `EGOADMIN_APP_USER_JWTSIGNKEY` | `production-secret` |
| 日志级别 | `EGOADMIN_APP_SERVICE_LOGLEVEL` | `info` |

## 配置验证与快速失败

EgoAdmin 在服务启动时执行配置验证。缺失必填字段或类型错误时，服务会立即退出并输出错误信息。

**必填配置项**：

```text
- [client.mysql].dsn        — 数据库连接串
- [app.service].name        — 服务名称
- [etcd].addrs              — 注册中心地址（启用服务发现时）
```

**快速失败示例**：

```text
FATAL config validation failed: client.mysql.dsn is required
exit status 1
```

```text
FATAL config validation failed: server.http.port must be between 1 and 65535
exit status 1
```

**打印完整默认配置**：

```bash
# 打印服务的完整默认配置（含所有 section 和默认值）
go run ./cmd/user config print-default
go run ./cmd/gateway config print-default
go run ./cmd/idgen config print-default

# 等价于 make 命令
make service.config SERVICE=user
make service.config SERVICE=gateway
make service.config SERVICE=idgen
```

::: tip
使用 `config print-default` 可查看所有可用配置项及其默认值。合并本地配置和环境变量后，可通过 `/config` governor 端点查看生效配置（敏感字段已脱敏）。
:::

## 容器环境地址规则

容器内服务互联必须使用 Compose service DNS 名，不使用 `127.0.0.1`。

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

[etcd]
addrs = ["etcd:2379"]
```

## 敏感配置

必须通过部署环境注入：

- MySQL DSN。
- Redis password。
- MinIO access key / secret key。
- JWT sign key。
- CDN sign secret。
- image processor secret。
- idcodec secret。

不要提交生产 secret 到仓库。

