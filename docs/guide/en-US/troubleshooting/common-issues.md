# Common Issues

This page collects the most frequent problems developers encounter when working with EgoAdmin, organized by failure scenario with diagnostic steps and solutions.

## Overview

EgoAdmin consists of three microservices (gateway, user, idgen) running alongside MySQL, Redis, etcd, MinIO, DTM, and Jaeger. Issues typically occur in four stages: service startup, inter-service communication, authentication, and frontend build.

Recommended troubleshooting order:

1. Confirm all middleware is ready (`make dev-up` or `make deploy-up` completes without errors).
2. Confirm service health checks pass (`curl http://localhost:<port>/healthz`).
3. Check service logs in `logs/ego.sys` for the specific error.
4. Follow the relevant section below for targeted diagnosis.

::: tip
Run `make service.config SERVICE=<name>` to see the full resolved configuration for a service, confirming whether values are correct.
:::

## Service Startup Failures

### Port Conflicts

**Symptom**: Service exits immediately on startup with `bind: address already in use`.

**Diagnosis**:

```bash
netstat -tlnp | grep 9001
netstat -tlnp | grep 9101
netstat -tlnp | grep 9201
```

**Solution**:

Update the port in `configs/<service>/config.toml`, or stop the process occupying the port.

```toml
# gateway/config.toml
[server.http]
port = 9001

# user/config.toml
[server.http]
port = 9101
[server.grpc]
port = 9102
```

::: warning
The gateway gRPC port is specified by `grpcEndpoint` (default `127.0.0.1:9002`). User and idgen each have their own HTTP and gRPC ports.
:::

### Missing Middleware

**Symptom**: Service logs show `connection refused` or `context deadline exceeded` when connecting to MySQL, Redis, or etcd.

**Diagnosis**:

```bash
# Check middleware container status
docker ps

# Check etcd
curl http://127.0.0.1:2379/health

# Check MySQL
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin -e "SELECT 1"
```

**Solution**:

```bash
# Start all middleware
make dev-up

# Restart a specific middleware
make dev.reset-one MIDDLEWARE=mysql-user
```

::: tip
`make dev-up` must complete before starting any services. Container names and ports are defined in `test/compose/docker-compose.dev.yml`.
:::

### Configuration Errors

**Symptom**: Service panics on startup with `unmarshal` errors.

**Diagnosis**:

```bash
# View default configuration
go run ./cmd/user config print-default
make service.config SERVICE=gateway
```

**Solution**:

Compare the default output against `configs/<service>/config.toml`. Common mistakes:

- Port specified as string instead of integer.
- Missing required `[app.service]` section.
- TOML array syntax error (missing quotes inside `[]`).

### Atlas Migration Failures

**Symptom**: Service startup logs show `migration` or `atlas` related errors.

**Diagnosis**:

```bash
# Check atlas installation
atlas version

# Check migration directory
ls atlas/migrations/user/

# Validate migration files
make migrate.validate SERVICE=user
```

**Solution**:

1. Confirm atlas binary is installed: `make install`.
2. Confirm `[app.dbMigration].url` in `configs/<service>/config.toml` points to the correct database.
3. Confirm migration directory matches the service (user uses `atlas/migrations/user`, gateway uses `atlas/migrations/gateway`).

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
```

::: warning
When migration hash validation fails, run `make migrate.hash SERVICE=user` to regenerate `atlas.sum`.
:::

## Database Connection Issues

### Connection Timeout

**Symptom**: Logs show `dial tcp: i/o timeout` or `connection timed out`.

**Diagnosis**:

```bash
# Confirm MySQL container is running
docker ps | grep mysql

# Test port reachability
nc -zv 127.0.0.1 3307

# Check DSN format
grep "dsn" configs/user/config.toml
```

**Solution**:

Local development DSN example:

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

Container environments must use Docker Compose service names:

```toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

### Authentication Failure

**Symptom**: Logs show `Access denied for user`.

**Solution**:

- Confirm the DSN username and password match the MySQL container initialization parameters.
- Default development credentials are `egoadmin:egoadmin`, defined in `test/compose/docker-compose.dev.yml`.
- For container environments, override via `ATLAS_URL`:

```bash
export ATLAS_URL='mysql://user:password@mysql-user:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'
```

### Schema Mismatch

**Symptom**: SQL execution reports `Table 'xxx' doesn't exist` or `Unknown column`.

**Solution**:

```bash
# Apply migrations
make migrate.apply SERVICE=user ATLAS_URL='mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local'

# Or check autoMigrate config
grep autoMigrate configs/user/config.toml
```

## Authentication and Permission Errors

### JWT Token Expired

**Symptom**: API returns `401 Unauthorized`, frontend redirects to login page.

**Diagnosis**:

Check `jwtExpire` configuration (in seconds):

```toml
[app.user]
jwtExpire = 604800           # access token validity, 7 days
refreshTokenExpire = 2592000  # refresh token validity, 30 days
```

**Solution**:

The frontend should implement automatic refresh token renewal. During development, you can temporarily increase `jwtExpire` to avoid frequent expiration.

::: warning
`jwtSignKey` must differ between production and development environments. Override production values via environment variables; never commit them to the repository.
:::

### Permission Denied (403)

**Symptom**: API returns `403 Forbidden`, frontend buttons are visible but operations fail.

**Diagnosis**:

```text
Permission chain troubleshooting order:
1. Does the user have the required role?
2. Is the target menu assigned to that role?
3. Is the menu bound to the correct API?
4. Are Casbin policies loaded correctly?
```

**Solution**:

- Navigate to System -> Role Management and confirm the role has the target menu permission.
- Navigate to Menu Management and confirm the menu's API identifier matches the backend OpenAPI annotation.
- Check that `web/dist/permission-contract.json` exists and is in sync with current menus:

```bash
test -f web/dist/permission-contract.json && echo ok || echo missing
```

::: tip
When saving role permissions, the backend validates grantable API IDs against `permission-contract.json`. If the frontend has not been built, this file will be missing and permission saves will fail.
:::

### Unexpected Session Invalidation

**Symptom**: User is kicked out during operation and must log in again.

**Diagnosis**:

```toml
[app.user]
heartbeatOfflineEnabled = true
heartbeatOfflineSeconds = 660   # heartbeat timeout in seconds
multiLoginEnabled = true
maxLoginClient = 2
```

**Solution**:

- In development, you can temporarily set `heartbeatOfflineEnabled = false` if switching browsers frequently.
- Check that Redis is running, as session data is stored there:

```bash
redis-cli -h 127.0.0.1 -p 6380 -a egoadmin ping
```

### Login Crypto Challenge Expired

**Symptom**: Login fails with challenge expired or timestamp skew errors in browser console.

**Solution**:

```toml
[component.logincrypto]
challengeTTL = "3m0s"     # challenge validity period
timestampSkew = "2m0s"    # timestamp deviation tolerance
rsaKeyBits = 4096
```

The frontend must complete login within `challengeTTL` after obtaining the challenge. You can temporarily increase `challengeTTL` for slow debugging sessions.

## Microservice Communication Failures

### Service Discovery Issues

**Symptom**: Gateway calls to user/idgen time out, logs show etcd-related errors.

**Diagnosis**:

```bash
# Confirm etcd is healthy
curl http://127.0.0.1:2379/health

# Confirm service is registered (check etcd keys)
etcdctl get --prefix /egoadmin --keys-only

# Confirm EGO_NAME configuration
grep "name" configs/user/config.toml
```

**Solution**:

Each service's `EGO_NAME` must match the address referenced in gateway client configuration:

```toml
# user/config.toml
[app.service]
name = "egoadmin-user"  # EGO_NAME

# gateway/config.toml - matching client config
[client.grpc.user]
addr = "etcd:///egoadmin-user"  # must match
```

::: warning
If etcd addresses are misconfigured, service registration silently fails. Verify all services point to the same etcd instance.
:::

### gRPC Client Timeout

**Symptom**: Gateway logs show `context deadline exceeded` when calling user or idgen.

**Solution**:

Adjust timeout settings for the corresponding client in gateway configuration:

```toml
[client.grpc.user]
addr = "etcd:///egoadmin-user"
readTimeout = "3s"    # per-request timeout
dialTimeout = "3s"    # connection establishment timeout

[client.grpc.idgen]
addr = "etcd:///egoadmin-idgen"
readTimeout = "3s"
dialTimeout = "5s"    # longer for idgen, due to ID allocation initialization
```

### Container DNS Resolution Failure

**Symptom**: Inter-service calls fail in container environments, logs show `no such host`.

**Solution**:

Use Docker Compose service names inside containers, never `127.0.0.1`:

```toml
# Correct - container environment
[client.grpc.user]
addr = "etcd:///egoadmin-user"

[etcd]
addrs = ["etcd:2379"]

[client.mysql]
dsn = "egoadmin:egoadmin@tcp(mysql-user:3306)/egoadmin_user?..."

# Wrong - do not use 127.0.0.1 in containers
[etcd]
addrs = ["127.0.0.1:2379"]
```

## API Call Failures

### Validation Errors

**Symptom**: API returns `400 Bad Request` with `invalid` in the response body.

**Diagnosis**:

Check proto file validation tags:

```protobuf
string name = 1 [(buf.validate.field).string.min_len = 1];
```

**Solution**:

- Confirm request field types are correct (e.g., `uint64` is a string in JSON, not a number).
- Confirm all required fields are provided.
- Use `JSON.stringify` in the frontend to inspect the actual payload being submitted.

### Proto Field Access Errors

**Symptom**: Reading proto fields in Go code returns zero values.

**Solution**:

Proto-generated fields are private; always use `GetXxx()` accessors:

```go
// Wrong
name := req.Name

// Correct
name := req.GetName()
```

### Copier Mapping Failures

**Symptom**: Field copying from proto objects to domain objects is incorrect or fields are empty.

**Solution**:

Check that `copier` struct tags match:

```go
type User struct {
    Name string `copier:"Name"`
}
```

Proto-generated field names are PascalCase; ensure copier tags match. If field types differ (e.g., `uint64` vs `int64`), copier may skip the copy, requiring manual assignment.

## Frontend Issues

### Build Failures

**Symptom**: `pnpm run build` exits with errors.

**Diagnosis**:

```bash
cd web
node -v     # needs 18+
pnpm -v     # needs 11.5.x

# Clean reinstall
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

**Solution**:

- Confirm Node.js and pnpm versions meet requirements (see `engines` field in `web/package.json`).
- If type checking fails, run `pnpm run type-check` to see specific errors.
- Confirm you have not modified auto-generated files (e.g., `*.gen.ts`).

### Proxy Errors

**Symptom**: API requests return 502 or connection refused during frontend development.

**Solution**:

Confirm the backend service is running, and check the proxy target in `web/vite.config.ts`:

```text
The dev server proxies /api/* to the backend gateway by default.
The default backend address is http://127.0.0.1:9001
```

::: tip
Both the frontend dev server (`pnpm run dev`) and the backend gateway must be running for the proxy to work.
:::

### Route 404

**Symptom**: After login, visiting a page shows 404 or a blank page.

**Solution**:

EgoAdmin frontend uses dynamic routing driven by menu data:

1. Check that menu data is successfully fetched after login.
2. Check that routes are correctly generated in the `permission` store.
3. Check the route path configuration in Menu Management.

### White Screen

**Symptom**: Page opens with a white screen, console shows errors.

**Diagnosis**:

Open browser Developer Tools -> Console and check the specific error message.

**Common causes**:

- Component not registered: check proper import in `main.ts`.
- Permission data loading failure: check if network requests return 401.
- Vue Router misconfiguration: check if route `component` references are correct.

## Migration Issues

### Hash Mismatch

**Symptom**: `make migrate.apply` reports hash mismatch error.

**Solution**:

```bash
# Recalculate hash
make migrate.hash SERVICE=user

# Commit the updated atlas.sum
git add atlas/migrations/user/atlas.sum
```

### Migration File Conflicts

**Symptom**: `atlas.sum` conflict when multiple developers create migrations simultaneously.

**Solution**:

```bash
# Pull latest code and re-validate
git pull
make migrate.validate SERVICE=user

# If conflicts remain, resolve manually then re-hash
make migrate.hash SERVICE=user
```

## Reference Links

- `configs/gateway/config.toml` — gateway configuration
- `configs/user/config.toml` — user configuration
- `configs/idgen/config.toml` — idgen configuration
- `test/compose/docker-compose.dev.yml` — development middleware compose
- `deploy/configs/` — deployment configuration directory
- `atlas/migrations/` — database migration files
- `web/vite.config.ts` — frontend proxy configuration
- `internal/component/authsession/` — authentication session component
- `internal/component/logincrypto/` — login crypto component
