# Runtime Configuration

Each service has its own runtime config file. Deployment-specific values and secrets should be overridden by environment variables or deployment config.

## Files

```text
configs/
├── gateway/config.toml
├── user/config.toml
└── idgen/config.toml
```

Deployment configs:

```text
deploy/configs/
├── gateway/config.toml
├── user/config.toml
└── idgen/config.toml
```

## Print Default Config

```bash
make service.config SERVICE=gateway
make service.config SERVICE=user
make service.config SERVICE=idgen
```

## gateway Example

```toml
[app.service]
name = "egoadmin-gateway"
platformName = "核心管理平台"

[server.http]
host = "0.0.0.0"
port = 9001
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:9002"
stripPrefix = "/api"

[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"
dialTimeout = "3s"
```

## user Example

```toml
[app.service]
autoMigrate = false
name = "egoadmin-user"
skipPermissionContractCheck = true

[app.user]
adminPassword = "123456"
jwtExpire = 604800
refreshTokenExpire = 2592000
jwtSignKey = "local-egoadmin-jwt-sign-key"
```

## Container Addressing

Inside containers, use Compose service DNS names rather than `127.0.0.1`.

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

[etcd]
addrs = ["etcd:2379"]
```

## Secrets

Never commit production secrets. Override these through deployment configuration:

- MySQL DSN
- Redis password
- MinIO keys
- JWT signing key
- CDN signing secret
- idcodec secret

