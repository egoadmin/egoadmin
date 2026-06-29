# Runtime Configuration

Each service has its own runtime config file. Deployment-specific values and secrets should be overridden by environment variables or deployment config.

## Full Config Structure

EgoAdmin uses TOML format config files. All configuration lives under these top-level sections:

```text
[app]          # Application business config
[server]       # Server config (HTTP, gRPC, governor)
[client]       # Client config (MySQL, Redis, gRPC clients)
[component]    # Component config (logincrypto, idgen, asyncq, jetcache, dtm)
[etcd]         # etcd registry config
[trace]        # Distributed tracing config
[jaeger]       # Jaeger tracing config
```

::: tip
Section hierarchy uses dot notation, e.g., `[server.http]` is equivalent to nested `server -> http`.
:::

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

## idgen Example

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

## Component Configs

### logincrypto

Login encryption component for password hashing and login state verification:

```toml
[component.logincrypto]
# Encryption algorithm: bcrypt / sm4
algorithm = "bcrypt"
# bcrypt cost factor
bcryptCost = 10
```

### idgen

ID generator configuration using idcodec encoding:

```toml
[component.idgen]
# idcodec secret for encoding/decoding IDs
secret = "local-idgen-secret"
# ID sequence step size
step = 1000
# Worker ID source: static / etcd
workerIdSource = "static"
workerId = 1
```

::: warning
In production, `secret` must be injected via environment variables. Never commit it to the repository.
:::

### asyncq

Async queue component configuration:

```toml
[component.asyncq]
# Enable async queue
enabled = true
# Concurrent worker count
workers = 5
# Task max retry count
maxRetry = 3
# Retry interval in seconds
retryInterval = 30
```

### jetcache

Cache component configuration:

```toml
[component.jetcache]
# Enable local cache
local.enabled = true
# Local cache capacity
local.capacity = 1000
# Local cache TTL in seconds
local.ttl = 60
# Enable remote cache (Redis)
remote.enabled = true
# Remote cache default TTL in seconds
remote.ttl = 300
# Cache key prefix
remote.prefix = "egoadmin"
```

### dtm

Distributed transaction component configuration:

```toml
[component.dtm]
enabled = false
server = "etcd:///egoadmin-dtm"

[component.dtm.branch.user]
target = "etcd:///egoadmin-user"
```

## Server Configs

### HTTP Server

```toml
[server.http]
host = "0.0.0.0"
port = 9001
mode = "release"           # debug / release / test
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:9002"
stripPrefix = "/api"
```

| Field | Description | Default |
|-------|-------------|---------|
| `host` | Listen address | `0.0.0.0` |
| `port` | Listen port | `9001` |
| `mode` | Gin run mode | `release` |
| `ginRelativePath` | API route prefix | `/api/*action` |
| `grpcEndpoint` | gRPC reverse proxy endpoint | `127.0.0.1:9002` |
| `stripPrefix` | Route strip prefix | `/api` |

### gRPC Server

```toml
[server.grpc]
name = "egoadmin-user"
host = "0.0.0.0"
port = 9002
```

| Field | Description | Default |
|-------|-------------|---------|
| `name` | Service name for registration/discovery | service name |
| `host` | Listen address | `0.0.0.0` |
| `port` | Listen port | `9002` |

### Governor Server

Governor is a built-in governance port providing health checks and debugging endpoints:

```toml
[server.governor]
host = "0.0.0.0"
port = 9003
enable = true
```

Available endpoints:

| Endpoint | Description |
|----------|-------------|
| `/healthz` | Liveness check |
| `/readyz` | Readiness check |
| `/debug/pprof/` | Go pprof profiling |
| `/config` | Current active config (desensitized) |

## Client Configs

### MySQL

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
maxOpenConns = 100
maxIdleConns = 10
connMaxLifetime = "3600s"
connMaxIdleTime = "600s"
```

| Field | Description | Default |
|-------|-------------|---------|
| `dsn` | Database connection string | none (required) |
| `maxOpenConns` | Max open connections | `100` |
| `maxIdleConns` | Max idle connections | `10` |
| `connMaxLifetime` | Connection max lifetime | `3600s` |
| `connMaxIdleTime` | Idle connection max lifetime | `600s` |

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

| Field | Description | Default |
|-------|-------------|---------|
| `addr` | Redis address | `127.0.0.1:6379` |
| `password` | Password | empty |
| `db` | Database number | `0` |
| `poolSize` | Connection pool size | `100` |
| `minIdleConns` | Min idle connections | `10` |

### gRPC Clients

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

| Field | Description | Default |
|-------|-------------|---------|
| `addr` | Service address (supports etcd discovery) | none (required) |
| `readTimeout` | Read timeout | `3s` |
| `dialTimeout` | Dial timeout | `3s` |

::: tip
When using etcd service discovery, the gRPC client address format is `etcd:///<service-name>`. No IP address configuration needed.
:::

## Trace / Jaeger Config

```toml
[trace]
# Enable distributed tracing
enable = true
# Sample rate: 0.0 ~ 1.0
sampleRate = 1.0

[jaeger]
# Jaeger agent address
host = "127.0.0.1"
port = 6831
# Jaeger collector endpoint (optional, alternative to agent)
collectorEndpoint = ""
```

Production deployment:

```toml
[trace]
enable = true
sampleRate = 0.1

[jaeger]
host = "jaeger"
port = 6831
```

::: tip
In production, set sample rate to 0.1 ~ 0.5 to avoid excessive tracing data under high traffic.
:::

## etcd Registry Config

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
```

Inside containers, use Compose service DNS names:

```toml
[etcd]
addrs = ["etcd:2379"]
```

| Field | Description | Default |
|-------|-------------|---------|
| `addrs` | etcd node address list | `["127.0.0.1:2379"]` |

## Database Migration Config

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

## Environment Variable Overrides

EgoAdmin supports `EGOADMIN_` prefixed environment variables to override configuration. Environment variable names use underscores to separate TOML hierarchy levels.

**Override rules**:

| Config Path | Environment Variable |
|-------------|---------------------|
| `app.service.name` | `EGOADMIN_APP_SERVICE_NAME` |
| `server.http.port` | `EGOADMIN_SERVER_HTTP_PORT` |
| `client.mysql.dsn` | `EGOADMIN_CLIENT_MYSQL_DSN` |
| `client.redis.password` | `EGOADMIN_CLIENT_REDIS_PASSWORD` |
| `etcd.addrs` | `EGOADMIN_ETCD_ADDRS` |
| `jaeger.host` | `EGOADMIN_JAEGER_HOST` |

**Deployment examples**:

```bash
# Basic environment
export EGOADMIN_APP_SERVICE_NAME=egoadmin-user
export EGOADMIN_SERVER_HTTP_PORT=9001
export EGOADMIN_SERVER_GRPC_PORT=9002

# Database
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

Atlas migration uses a separate `ATLAS_` prefix:

```bash
export ATLAS_URL='mysql://user:password@mysql-user:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

::: tip
Derived projects use `egoadminctl init --env-prefix DEMOADMIN` to set a new env prefix (e.g., `DEMOADMIN_` instead of `EGOADMIN_`).
:::

**Common environment variable quick reference**:

| Scenario | Env Variable | Example Value |
|----------|-------------|---------------|
| MySQL connection | `EGOADMIN_CLIENT_MYSQL_DSN` | `user:pass@tcp(host:3306)/db` |
| Redis address | `EGOADMIN_CLIENT_REDIS_ADDR` | `redis:6379` |
| Redis password | `EGOADMIN_CLIENT_REDIS_PASSWORD` | `secret` |
| etcd address | `EGOADMIN_ETCD_ADDRS` | `["etcd:2379"]` |
| HTTP port | `EGOADMIN_SERVER_HTTP_PORT` | `9001` |
| gRPC port | `EGOADMIN_SERVER_GRPC_PORT` | `9002` |
| JWT signing key | `EGOADMIN_APP_USER_JWTSIGNKEY` | `production-secret` |
| Log level | `EGOADMIN_APP_SERVICE_LOGLEVEL` | `info` |

## Config Validation and Fail-Fast

EgoAdmin validates configuration on service startup. Missing required fields or type errors cause immediate exit with error messages.

**Required config fields**:

```text
- [client.mysql].dsn        — database connection string
- [app.service].name        — service name
- [etcd].addrs              — registry address (when service discovery is enabled)
```

**Fail-fast examples**:

```text
FATAL config validation failed: client.mysql.dsn is required
exit status 1
```

```text
FATAL config validation failed: server.http.port must be between 1 and 65535
exit status 1
```

## Print Default Config Details

```bash
# Print full default config with all sections and default values
go run ./cmd/user config print-default
go run ./cmd/gateway config print-default
go run ./cmd/idgen config print-default

# Equivalent make commands
make service.config SERVICE=user
make service.config SERVICE=gateway
make service.config SERVICE=idgen
```

::: tip
Use `config print-default` to see all available config fields and their defaults. After merging local config and env vars, use the `/config` governor endpoint to view the effective configuration (sensitive fields are desensitized).
:::

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

