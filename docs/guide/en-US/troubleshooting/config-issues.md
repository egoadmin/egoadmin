# Config Diagnostics

This page explains EgoAdmin's configuration loading mechanism and how to troubleshoot common configuration errors, helping quickly resolve config-related startup failures and runtime anomalies.

## Overview

EgoAdmin uses TOML-format configuration files with environment variable overrides. Each service (gateway, user, idgen) has its own configuration file. During deployment, environment variables with the `EGOADMIN_` prefix override sensitive information and environment-specific differences.

Configuration issues typically manifest as:

- Service startup failure with `unmarshal` or `missing key` errors in logs.
- Service fails to connect to middleware (wrong DSN format, incorrect address).
- Functional anomalies (JWT signature mismatch, Redis mode mismatch).

::: tip
The first step in troubleshooting config issues: run `make service.config SERVICE=<name>` to see the full resolved configuration.
:::

## Core Usage

### Config Loading Order

EgoAdmin loads configuration with the following priority (highest to lowest):

1. **Environment variables**: `EGOADMIN_` prefixed environment variables.
2. **Config files**: `configs/<service>/config.toml`.
3. **Defaults**: Code-level default values.

This means environment variables always override config file values, and config files override defaults.

```text
Priority: Environment variables > Config file > Defaults
```

### Print Default Configuration

```bash
# Via make command (recommended)
make service.config SERVICE=gateway
make service.config SERVICE=user
make service.config SERVICE=idgen

# Run binary directly
go run ./cmd/user config print-default
go run ./cmd/gateway config print-default
```

### Environment Variable Override Rules

The environment variable prefix is `EGOADMIN_`. Config paths are uppercased, with levels separated by `_`:

```toml
# Config file syntax
[client.mysql]
dsn = "user:pass@tcp(127.0.0.1:3306)/db"
```

```bash
# Corresponding environment variable
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db"
```

::: info
Environment variable names convert TOML config paths to all-uppercase, joined by underscores. For example, `[server.grpc].host` maps to `EGOADMIN_SERVER_GRPC_HOST`.
:::

### Config File Locations

```text
configs/
├── gateway/config.toml       # gateway service config
├── gateway/local-live.toml   # Local environment override
├── user/config.toml          # user service config
├── user/local-live.toml      # Local environment override
├── idgen/config.toml         # idgen service config
└── idgen/local-live.toml     # Local environment override
```

Deployment configs:

```text
deploy/configs/
├── gateway/config.toml
├── user/config.toml
└── idgen/config.toml
```

## Configuration Examples

### Common Environment Variable Overrides

Override sensitive information and environment differences via environment variables during deployment:

```bash
# MySQL connection address (use service DNS names in container environments)
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db?charset=utf8mb4&parseTime=True&loc=Local"

# Redis address
export EGOADMIN_CLIENT_REDIS_ADDR="redis:6380"

# JWT signing key (must be changed in production)
export EGOADMIN_APP_USER_JWTSIGNKEY="production-secret-key-at-least-32-chars"

# gRPC server listen address
export EGOADMIN_SERVER_GRPC_HOST="0.0.0.0"
```

::: warning
The JWT signing key `jwtSignKey` must differ between production and development. Override it via environment variables in production. Never commit the production key to the code repository.
:::

### Container Environment Configuration

In container environments, always use Docker Compose service names instead of `127.0.0.1`:

```toml
# Wrong: 127.0.0.1 inside a container points to the container itself, not the host or other containers
[client.mysql]
dsn = "user:pass@tcp(127.0.0.1:3306)/db"

# Correct: use Docker Compose service names
[client.mysql]
dsn = "user:pass@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

Same for etcd addresses:

```toml
# Correct: use Docker Compose service names
[server.registry]
addrs = ["http://etcd:2379"]
```

### Runtime Config Check

If governor is enabled, you can inspect runtime configuration via HTTP endpoint:

```bash
# View runtime configuration (if governor is enabled)
curl http://localhost:9103/debug/config
```

## Real-World Examples

### Example 1: Wrong DSN Format

**Symptom**: Service panics immediately on startup with `invalid DSN` or `default addr for network 'unknown'`.

**Diagnosis**:

```bash
# Check current DSN configuration
grep "dsn" configs/user/config.toml
```

**Solution**:

MySQL DSN must follow Go `go-sql-driver/mysql` format:

```toml
# Format: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...]
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

Common DSN format errors:

| Wrong | Correct |
|-------|---------|
| `user:pass@127.0.0.1:3306/db` | `user:pass@tcp(127.0.0.1:3306)/db` |
| `user:pass@tcp(127.0.0.1:3306)db` | `user:pass@tcp(127.0.0.1:3306)/db` |
| `user:pass@tcp(:3306)/db` | `user:pass@tcp(127.0.0.1:3306)/db` |

### Example 2: Missing etcd Address

**Symptom**: Service logs show `no etcd endpoints` or service registration fails.

**Diagnosis**:

```bash
grep "addrs" configs/user/config.toml
```

**Solution**:

```toml
[server.registry]
addrs = ["http://127.0.0.1:2379"]   # Local development
# Or container environment
addrs = ["http://etcd:2379"]        # Docker Compose
```

::: warning
`addrs` must be in TOML array format. Even a single address must be wrapped in square brackets.
:::

### Example 3: JWT Signing Key Too Short

**Symptom**: Service logs show `key is too short` or JWT signing/verification fails.

**Diagnosis**:

```bash
grep "jwtSignKey" configs/user/config.toml
```

**Solution**:

JWT signing key length must meet algorithm requirements (HS256 requires at least 32 bytes):

```toml
[app.user]
jwtSignKey = "at-least-32-bytes-long-secret-key-here"
```

::: danger
Production `jwtSignKey` must be overridden via environment variable using a high-entropy random string. Ensure both gateway and user services use the same key.
:::

### Example 4: Redis Mode Mismatch

**Symptom**: Redis operations return `MOVED` or `CLUSTERDOWN` errors.

**Diagnosis**:

```bash
grep "mode\|addrs" configs/user/config.toml
```

**Solution**:

Redis mode must match the actual deployment:

```toml
# Standalone mode (default, common in development)
[client.redis]
mode = "stub"
addrs = ["127.0.0.1:6380"]

# Cluster mode
[client.redis]
mode = "cluster"
addrs = ["redis-node-1:6379", "redis-node-2:6379", "redis-node-3:6379"]
```

::: tip
Redis started via local `make dev-up` is standalone mode, matching `mode = "stub"`. Configuring `cluster` mode against standalone Redis causes operation errors.
:::

### Example 5: Wrong Port Configuration

**Symptom**: Other services get connection timeouts or connection refused after a service starts.

**Diagnosis**:

```bash
# Confirm actual listening ports
netstat -tlnp | grep ego

# Cross-check with config files
grep "port" configs/gateway/config.toml
grep "port" configs/user/config.toml
```

**Solution**:

EgoAdmin default port allocation:

| Service | HTTP | gRPC | Governor |
|---------|------|------|----------|
| gateway | 9001 | 9002 (via grpcEndpoint) | - |
| user | 9101 | 9102 | 9103 |
| idgen | 9201 | 9202 | 9203 |

Gateway's gRPC port is specified via `grpcEndpoint`:

```toml
# configs/gateway/config.toml
[server.http]
grpcEndpoint = "127.0.0.1:9002"
```

### Example 6: Config Validation and Fail-Fast

**Symptom**: Service exits immediately on startup, logs show config validation failure.

EgoAdmin validates critical configuration during startup. If required config is missing or malformed, the service fails fast to prevent unexpected runtime behavior.

**Diagnosis**:

```bash
# View config validation errors in startup logs
go run ./cmd/user 2>&1 | head -50
```

**Solution**:

Cross-check each item against the default configuration output. Use `config print-default` to get the full config template.

## Common Issues

### Config File Not Found

**Symptom**: Service startup reports `config file not found`.

**Solution**:

```bash
# Confirm config files exist
ls configs/user/config.toml
ls configs/gateway/config.toml

# In container environments, confirm correct mount paths
ls deploy/configs/user/config.toml
```

### Environment Variables Not Taking Effect

**Symptom**: After setting environment variables, config values haven't changed.

**Diagnosis**:

```bash
# Confirm environment variable names are correct
env | grep EGOADMIN

# Confirm variable name format (all uppercase, underscore separated)
# [client.mysql].dsn → EGOADMIN_CLIENT_MYSQL_DSN
```

**Solution**:

Environment variable names must match exactly: prefix `EGOADMIN_` + config path in uppercase + `_` separator. Case-sensitive.

### Hot Config Reload

::: warning
EgoAdmin currently does not support hot config reload. You must restart the service after modifying config files. In production, override sensitive config via environment variables and keep config files unchanged.
:::

### Multi-Environment Config Management

Use `local-live.toml` to override development environment differences:

```toml
# configs/user/local-live.toml (development environment override)
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

[client.redis]
addrs = ["127.0.0.1:6380"]
```

Production uses config files under `deploy/configs/` with environment variable overrides.

## Reference Links

- [EgoAdmin Runtime Configuration](/guide/en-US/configuration)
- [EgoAdmin Deployment Guide](/guide/en-US/testing-deployment)
- [TOML Specification](https://toml.io/)
- [Go go-sql-driver/mysql DSN Format](https://github.com/go-sql-driver/mysql#dsn-data-source-name)
