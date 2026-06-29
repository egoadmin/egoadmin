# 运行时配置

EgoAdmin 每个服务都有独立配置文件，部署时通过环境变量覆盖敏感信息和环境差异。

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

普通部署中通过环境变量覆盖：

```bash
export EGO_NAME=egoadmin-user
export ATLAS_URL='mysql://user:password@mysql-user:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

派生项目通过 `egoadminctl init --env-prefix DEMOADMIN` 设置新的环境变量前缀。

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

