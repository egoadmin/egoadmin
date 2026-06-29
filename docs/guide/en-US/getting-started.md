# Getting Started

This guide helps you start the full EgoAdmin stack locally and understand the daily development commands.

## Requirements

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26 | Backend build and tests |
| Docker | 24+ | Local middleware and deployment stack |
| Docker Compose | v2 | `make dev-up` / `make deploy-up` |
| GNU Make | latest | Unified project command entry |
| Node.js | 18+ | Frontend build |
| pnpm | 11.5.x | Frontend package manager |

::: tip
Run `make install` to install project tools such as Buf, Wire, Atlas and protobuf plugins.
:::

## One-Command Deployment

```bash
git clone https://github.com/egoadmin/egoadmin.git
cd egoadmin

export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

Open:

```text
http://localhost:9001
```

Default administrator:

| Username | Password |
|----------|----------|
| `admin` | `123456` |

::: warning
The default password is only for local testing and demo. In production, change the admin password and JWT secret in `configs/user/config.toml` or via environment variables.
:::

Stop the stack:

```bash
make deploy-down
```

## Local Development

### Install Tools

```bash
make install
```

### Start Middleware

```bash
make dev-up
```

Common local endpoints:

| Middleware | Endpoint | Notes |
|------------|----------|-------|
| MySQL gateway | `127.0.0.1:3306` | `egoadmin_gateway` |
| MySQL user | `127.0.0.1:3307` | `egoadmin_user` |
| MySQL idgen | `127.0.0.1:3308` | `egoadmin_idgen` |
| Redis gateway | `127.0.0.1:6379` | gateway cache/queue |
| Redis user | `127.0.0.1:6380` | user cache/queue |
| etcd | `127.0.0.1:2379` | service discovery |
| MinIO | `127.0.0.1:9000` | object storage |
| DTM | `127.0.0.1:36789` | distributed transaction manager |

What each middleware does:

| Middleware | Purpose | Used by |
|------------|---------|---------|
| MySQL | Relational storage, one database per service, schemas managed by Atlas | gateway, user, idgen |
| Redis | JWT session store, Casbin policy cache, async task queue, JetCache multi-level cache | gateway, user |
| etcd | Service registration and discovery; gRPC clients resolve backends via `etcd:///` prefix | gateway, user, idgen |
| MinIO | Object storage for file uploads with TUS resumable upload and CDN signed delivery | gateway |
| Jaeger | OpenTelemetry trace collection via OTLP gRPC | gateway, user, idgen |
| DTM | Distributed transaction manager coordinating Saga/TCC cross-service writes | gateway, user |

::: tip
If you are only changing user service business logic, you can start just MySQL user, Redis user, and etcd, then run `make run SERVICE=user`. No need to start all middleware.
:::

Control one middleware service:

```bash
make dev.up-one MIDDLEWARE=mysql-user
make dev.down-one MIDDLEWARE=mysql-user
make dev.reset-one MIDDLEWARE=mysql-user
```

### Run Services

```bash
make run
```

Default startup order:

```text
idgen -> user -> gateway
```

Run a single service:

```bash
make run SERVICE=idgen
make run SERVICE=user
make run SERVICE=gateway
```

::: tip
When starting gateway, if `web/dist/index.html` does not exist, the Makefile will automatically trigger a frontend build. Set `SKIP_WEB_BUILD=1` to skip it, but ensure `web/dist` already exists.
:::

### Environment Variable Overrides

EgoAdmin supports overriding any TOML config via environment variables. Convert the TOML path to uppercase underscore format and add a service prefix:

```text
TOML Path                        Environment Variable
─────────────────────────────    ──────────────────────────────────
[server.http] port = 9001       EGOADMIN_SERVER_HTTP_PORT=9001
[client.mysql] dsn = "..."      EGOADMIN_CLIENT_MYSQL_DSN="..."
[app.user] jwtExpire = 604800   EGOADMIN_APP_USER_JWTEXPIRE=604800
[etcd] addrs = ["..."]          EGOADMIN_ETCD_ADDRS="127.0.0.1:2379"
```

Common override examples:

```bash
# Change user service MySQL connection
export EGOADMIN_CLIENT_MYSQL_DSN="root:secret@tcp(db-server:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

# Change JWT expiration (seconds)
export EGOADMIN_APP_USER_JWTEXPIRE=86400

# Change etcd address
export EGOADMIN_ETCD_ADDRS="etcd-1:2379,etcd-2:2379"

# Change HTTP port
export EGOADMIN_SERVER_HTTP_PORT=8001

make run SERVICE=user
```

::: warning
Environment variable overrides are read once at startup, not hot-reloaded. Array types (like `addrs`) use comma separation. Duration types (like `connectTimeout`) use Go duration format, e.g. `"3s"`.
:::

For Docker containers, prefer environment variables over mounted config files:

```bash
docker run -p 9101:9101 \
  -e EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(mysql:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local" \
  -e EGOADMIN_CLIENT_REDIS_ADDR="redis:6380" \
  -e EGOADMIN_ETCD_ADDRS="etcd:2379" \
  -e EGOADMIN_REGISTRY_SERVICETTL="30s" \
  ghcr.io/egoadmin/egoadmin-user:latest
```

::: tip
You can also write environment variables to a `.env` file in the project root. Services will load it automatically at startup.
:::

## Frontend Development

The frontend project is located in `web/` using Vue 3, TypeScript, Element Plus, Pinia, Vue Router and Vite.

```bash
cd web
pnpm install
pnpm run dev
```

Build frontend:

```bash
cd web
pnpm run build
```

`pnpm run build` performs three steps:

```text
vue-tsc --noEmit           # type checking
vite build                 # production build
node scripts/generate-contract.cjs  # permission contract
```

Build artifacts:

| File/Directory | Purpose |
|----------------|---------|
| `web/dist/index.html` | Embedded by gateway via `go:embed` |
| `web/dist/assets/*` | Frontend static assets |
| `web/dist/permission-contract.json` | Role permission boundary validation |

::: tip
When developing only the frontend, `pnpm run dev` starts a Vite dev server with hot reload. The dev server proxies API requests to the gateway at `localhost:9001`.
:::

## Health Checks

Each service exposes unauthenticated HTTP health endpoints:

```bash
# gateway
curl http://localhost:9001/healthz   # liveness: 200 if process is alive
curl http://localhost:9001/readyz    # readiness: 200 after migration + dependencies ready

# user
curl http://localhost:9101/healthz
curl http://localhost:9101/readyz

# idgen
curl http://localhost:9201/healthz
curl http://localhost:9201/readyz
```

::: warning
`user` and `idgen` health checks use the HTTP port (9101, 9201), not the gRPC port (9102, 9202). The governor port (9103, 9203) also exposes the same health endpoints.
:::

Check if a service is truly ready:

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:9101/readyz
# Expected: 200
```

::: danger
If readyz keeps returning 503, a dependency (database, etcd, etc.) is not ready. Check the service logs to identify which component failed to connect.
:::

### Quick Verification Checklist

After completing local dev setup, verify in this order:

```bash
# 1. Middleware health
curl -s http://localhost:2379/health          # etcd
mysqladmin -h 127.0.0.1 -P 3306 ping         # MySQL gateway
mysqladmin -h 127.0.0.1 -P 3307 ping         # MySQL user
redis-cli -p 6379 -a egoadmin ping           # Redis gateway

# 2. Service readiness
curl -s http://localhost:9001/readyz          # gateway
curl -s http://localhost:9101/readyz          # user
curl -s http://localhost:9201/readyz          # idgen

# 3. Frontend page accessible
curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/
# Expected: 200

# 4. Login API call
curl -X POST http://localhost:9001/api/user.v1.UserService/GetLoginCrypto \
  -H 'Content-Type: application/json' \
  -d '{}'
# Expected: JSON with challenge and publicKey

# 5. Permission contract file exists
test -f web/dist/permission-contract.json && echo "permission contract OK"
```

::: tip
The full login flow is: `GetLoginCrypto` -> get RSA public key to encrypt password -> `Login` -> receive Bearer token. The frontend wraps this flow. Manual verification only needs to confirm `GetLoginCrypto` returns successfully.
:::

## Common Commands

| Goal | Command |
|------|---------|
| Install tools | `make install` |
| Generate proto/Go/Wire | `make gen` |
| Run all services | `make run` |
| Run one service | `make run SERVICE=user` |
| Build services | `make build` |
| Start middleware | `make dev-up` |
| Generate migration | `make migrate.new SERVICE=user NAME=add_field` |
| Run tests | `go test ./...` |
| Run e2e tests | `make e2e E2E_TIMEOUT=20m` |
| Frontend type check | `cd web && pnpm run type-check` |
| Frontend build | `cd web && pnpm run build` |

## Single-Service Docker

```bash
# gateway
docker run -p 9001:9001 \
  -e ATLAS_URL='mysql://user:pass@host:3306/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local' \
  ghcr.io/egoadmin/egoadmin-gateway:latest

# user
docker run -p 9101:9101 \
  -e ATLAS_URL='mysql://user:pass@host:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local' \
  ghcr.io/egoadmin/egoadmin-user:latest

# idgen
docker run -p 9201:9201 \
  -e ATLAS_URL='mysql://user:pass@host:3306/egoadmin_idgen?charset=utf8mb4&parseTime=True&loc=Local' \
  ghcr.io/egoadmin/egoadmin-idgen:latest
```

## Troubleshooting

### First-Time Setup Issues

| Symptom | Investigation |
|---------|---------------|
| gateway fails to start | Check if `web/dist/index.html` exists; verify `configs/gateway/config.toml` |
| user/idgen registration fails | Check etcd availability; verify `EGO_NAME` is `egoadmin-user` / `egoadmin-idgen` |
| Database migration fails | Check `app.dbMigration.url` points to the correct service database |
| Permission save fails | Check `web/dist/permission-contract.json` was generated; verify `routeMenu.ts` API bindings |
| Frontend ID precision issues | Protobuf `uint64` IDs must be treated as strings on the frontend |

### Gateway: `embed: pattern "index.html" not found`

Gateway uses `go:embed` to embed frontend static assets. If `web/dist/index.html` does not exist, compilation fails.

```bash
# Option 1: Build frontend first
cd web && pnpm install && pnpm run build

# Option 2: Skip frontend build (backend-only development)
export SKIP_WEB_BUILD=1
make run SERVICE=gateway
```

::: danger
`SKIP_WEB_BUILD=1` requires the `web/dist` directory to contain at least an `index.html` file. Create a placeholder if needed: `mkdir -p web/dist && touch web/dist/index.html`.
:::

### etcd Connection Timeout

Symptom: `context deadline exceeded` or `connection refused` in gateway logs.

```bash
# Check if etcd is running
curl http://localhost:2379/health

# Start only etcd if needed
make dev.up-one MIDDLEWARE=etcd

# Check service registrations
etcdctl get --prefix egoadmin --keys-only
```

### MySQL Connection Refused

Symptom: `dial tcp 127.0.0.1:3307: connect: connection refused`

```bash
# Check if MySQL container is running
docker ps | grep mysql-user

# Restart MySQL container
make dev.reset-one MIDDLEWARE=mysql-user

# Verify connection manually
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin egoadmin_user -e "SELECT 1"
```

### Redis Connection Failure

Symptom: `redis: connection refused` or `WRONGPASS`

```bash
# Verify Redis is reachable
redis-cli -p 6380 -a egoadmin ping

# Note: user and gateway use different Redis instances
# gateway: 127.0.0.1:6379
# user:    127.0.0.1:6380
```

### Migration Mismatch

```bash
# Check current migration status
atlas migrate status --dir file://atlas/migrations/user \
  --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"

# In development, reset the database
make dev.reset-one MIDDLEWARE=mysql-user
```

::: warning
Never reset databases in production. Use `atlas migrate apply` or contact your DBA.
:::

### Blank Frontend Page

```bash
# 1. Verify frontend was built
test -f web/dist/index.html && echo "OK" || echo "Build frontend first"

# 2. Check gateway logs for HTTP requests
# 3. Check browser console for API errors
# 4. Verify backend readyz returns 200
curl http://localhost:9101/readyz
```
