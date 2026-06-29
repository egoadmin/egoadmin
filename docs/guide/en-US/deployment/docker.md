# Docker Containerization

EgoAdmin uses multi-stage Docker builds and Docker Compose to unify local development and production deployment environments.

## Overview

Each EgoAdmin microservice (gateway, user, idgen) has its own Dockerfile using a two-stage build: a Go compilation stage and a minimal runtime image. Docker Compose orchestrates all services and middleware dependencies.

| Image | Description | Build File |
|-------|-------------|------------|
| `egoadmin-gateway` | Gateway service with embedded frontend | `cmd/gateway/Dockerfile` |
| `egoadmin-user` | User service | `cmd/user/Dockerfile` |
| `egoadmin-idgen` | ID generation service | `cmd/idgen/Dockerfile` |

## Core Usage

### Building Images

```bash
# Build all service images
make image.build

# Build a specific service only
make image.build SERVICE=user
```

The resulting image tag format is `${DOCKER_REGISTRY}/egoadmin-<service>:latest`.

### Starting the Full Deployment Stack

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

This starts all application services and middleware:

```text
deploy/
├── docker-compose.yml          # Main entry point
└── compose/
    ├── app.yml                 # gateway, user, idgen
    ├── mysql.yml               # MySQL instances
    ├── redis.yml               # Redis instances
    ├── etcd.yml                # etcd cluster
    ├── minio.yml               # MinIO object storage
    ├── dtm.yml                 # DTM distributed transactions
    ├── jaeger.yml              # Jaeger distributed tracing
    ├── meilisearch.yml         # Meilisearch full-text search
    └── image-processor.yml     # Image processing service
```

### Managing Development Middleware

```bash
# Start local development middleware (MySQL, Redis, etcd, etc.)
make dev-up

# Stop local development middleware
make dev-down
```

Development middleware is defined in `test/compose/*.yml` and does not include application services. It is designed for use with `make run` during local development.

### Stopping the Deployment Stack

```bash
make deploy-down
```

## Configuration Examples

### Multi-Stage Dockerfile

Using gateway as an example, the complete two-stage build pattern looks like this:

```dockerfile
# ---- Build Stage ----
FROM golang:1.26.2-alpine AS builder

ARG SERVICE=gateway
ARG BUILD_TAG=0.0.1
ARG BUILD_VERSION=0.0.1

ARG GOPROXY=https://goproxy.cn,direct

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories \
    && apk add --no-cache gcc musl-dev git

ENV GOPROXY=${GOPROXY}
ENV CGO_ENABLED=0

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN BUILD_TIME=$(date +%Y-%m-%d_%H:%M:%S) && \
    LAST_COMMIT_HASH=$(git rev-parse HEAD 2>/dev/null || true) && \
    go build -tags netgo -o egoadmin \
    -ldflags "-extldflags '-static' \
    -X github.com/gotomicro/ego/core/eapp.appName=egoadmin-${SERVICE} \
    -X github.com/gotomicro/ego/core/eapp.buildVersion=${BUILD_VERSION} \
    -X github.com/gotomicro/ego/core/eapp.buildTime=${BUILD_TIME}" \
    ./cmd/${SERVICE}

# ---- Runtime Stage ----
FROM alpine:3.18

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories \
    && apk add --no-cache \
    ca-certificates \
    iputils \
    tzdata \
    && rm -f /etc/localtime \
    && ln -sv /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app/

ENV ATLAS_SERVICE=gateway
ENV EGO_NAME=egoadmin-gateway

COPY --from=arigaio/atlas:1.2.2-community-alpine /atlas /usr/local/bin/atlas
COPY atlas ./atlas
COPY --from=builder /build/egoadmin ./app

RUN chmod +x ./app ./atlas/apply.sh && \
    printf '%s\n' \
    '#!/bin/sh' \
    'set -e' \
    'if [ "${EGOADMIN_ATLAS_MIGRATE:-true}" = "true" ]; then' \
    '  cd /app && ./atlas/apply.sh' \
    '  export EGOADMIN_ATLAS_MIGRATED=true' \
    'fi' \
    'exec ./app "$@"' \
    > /entrypoint.sh && \
    chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
```

::: tip Build Parameters
Build info injected via `-ldflags` is exposed through the EGO framework at the `/env` endpoint, making it easy for operations to verify deployed versions.
:::

### Deployment Compose Application Config

```yaml
# deploy/compose/app.yml
services:
  idgen:
    image: ${DOCKER_REGISTRY:-egoadmin}/egoadmin-idgen:${DOCKER_TAG:-latest}
    restart: always
    command: ["--config=/app/config/config.toml"]
    environment:
      EGO_NAME: egoadmin-idgen
      EGOADMIN_SERVER_GRPC_NAME: egoadmin-idgen
      EGOADMIN_SERVER_GRPC_HOST: idgen
      EGOADMIN_CLIENT_MYSQL_DSN: ${MYSQL_USER:-egoadmin}:${MYSQL_PASSWORD:-egoadmin}@tcp(mysql-idgen:3306)/${MYSQL_IDGEN_DATABASE:-egoadmin_idgen}?charset=utf8mb4&parseTime=True&loc=Local
      EGOADMIN_ETCD_ADDRS: etcd:2379
      EGOADMIN_TRACE_OTLP_ENDPOINT: jaeger:4317
    volumes:
      - ../configs/idgen/config.toml:/app/config/config.toml:ro
    depends_on:
      mysql-idgen:
        condition: service_healthy
      etcd:
        condition: service_healthy
      jaeger:
        condition: service_started
    ports:
      - "${IDGEN_HTTP_PORT:-9201}:9201"
      - "${IDGEN_GRPC_PORT:-9202}:9202"
      - "${IDGEN_GOVERNOR_PORT:-9203}:9203"
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O - http://127.0.0.1:9201/readyz >/dev/null"]
      interval: 10s
      timeout: 5s
      retries: 12
      start_period: 30s

  user:
    image: ${DOCKER_REGISTRY:-egoadmin}/egoadmin-user:${DOCKER_TAG:-latest}
    restart: always
    environment:
      EGOADMIN_CLIENT_MYSQL_DSN: ${MYSQL_USER:-egoadmin}:${MYSQL_PASSWORD:-egoadmin}@tcp(mysql-user:3306)/${MYSQL_USER_DATABASE:-egoadmin_user}?charset=utf8mb4&parseTime=True&loc=Local
      EGOADMIN_CLIENT_REDIS_ADDR: redis-user:6379
      EGOADMIN_CLIENT_REDIS_PASSWORD: ${REDIS_PASSWORD:-egoadmin}
      EGOADMIN_ETCD_ADDRS: etcd:2379
      EGOADMIN_TRACE_OTLP_ENDPOINT: jaeger:4317
    ports:
      - "${USER_HTTP_PORT:-9101}:9101"
      - "${USER_GRPC_PORT:-9102}:9102"
      - "${USER_GOVERNOR_PORT:-9103}:9103"

  gateway:
    image: ${DOCKER_REGISTRY:-egoadmin}/egoadmin-gateway:${DOCKER_TAG:-latest}
    restart: always
    environment:
      EGOADMIN_CLIENT_MYSQL_DSN: ${MYSQL_USER:-egoadmin}:${MYSQL_PASSWORD:-egoadmin}@tcp(mysql-gateway:3306)/${MYSQL_GATEWAY_DATABASE:-egoadmin_gateway}?charset=utf8mb4&parseTime=True&loc=Local
      EGOADMIN_CLIENT_REDIS_ADDR: redis-gateway:6379
      EGOADMIN_CLIENT_REDIS_PASSWORD: ${REDIS_PASSWORD:-egoadmin}
      EGOADMIN_ETCD_ADDRS: etcd:2379
      EGOADMIN_TRACE_OTLP_ENDPOINT: jaeger:4317
    ports:
      - "${GATEWAY_HTTP_PORT:-9001}:9001"
      - "${GATEWAY_GRPC_PORT:-9002}:9002"
      - "${GATEWAY_GOVERNOR_PORT:-9003}:9003"
```

### Environment Variable Overrides

All configuration options can be overridden via `EGOADMIN_*` environment variables. The naming convention replaces `.` with `_` in the TOML path and converts to uppercase.

```bash
# Equivalent to [client.mysql] dsn = "..."
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db"

# Equivalent to [trace.otlp] Endpoint = "..."
export EGOADMIN_TRACE_OTLP_ENDPOINT="jaeger:4317"
```

::: warning Service Discovery in Containers
Inside containers, you must use Docker Compose service names (e.g., `mysql-gateway`, `redis-user`, `etcd`) instead of `127.0.0.1`. Only use `127.0.0.1` when running bare processes locally.
:::

### Config File Mounting

In production, config files are mounted into the container as volumes:

```yaml
volumes:
  - ../configs/gateway/config.toml:/app/config/config.toml:ro
```

::: tip Read-Only Mount
Config files are mounted as read-only (`:ro`) to prevent accidental runtime modifications. Sensitive information (database passwords, JWT secrets) should preferentially be injected via environment variables.
:::

## Real-World Examples

### Local Development Flow

```text
Developer machine
  ├── make dev-up                 # Start middleware (MySQL/Redis/etcd/MinIO...)
  ├── make run SERVICE=gateway    # Compile and run gateway locally
  ├── make run SERVICE=user       # Compile and run user locally
  └── Browser → http://localhost:9001
```

### Full Deployment Flow

```text
CI/CD or operations
  ├── make image.build            # Build Docker images
  ├── docker push ...             # Push to image registry
  └── make deploy-up              # Start full deployment stack
       ├── gateway (HTTP :9001, gRPC :9002)
       ├── user    (HTTP :9101, gRPC :9102)
       ├── idgen   (gRPC :9202)
       └── Middleware (MySQL/Redis/etcd/MinIO/DTM/Jaeger...)
```

### Port Layout

| Service | HTTP Port | gRPC Port | Governor Port |
|---------|-----------|-----------|---------------|
| gateway | 9001 | 9002 | 9003 |
| user | 9101 | 9102 | 9103 |
| idgen | 9201 | 9202 | 9203 |

::: info Governor Port
The governor port exposes `/healthz`, `/readyz`, `/debug/pprof/*`, and `/env` operational endpoints. It is not exposed to the host by default. Add port mappings in the compose file when debugging is needed.
:::

## How It Works

### Two-Stage Build Pipeline

```text
builder stage (golang:1.26.2-alpine)
  ├── go mod download     # Dependency caching layer
  ├── COPY . .            # Copy source code
  └── go build            # Static compile → egoadmin binary

runtime stage (alpine:3.18)
  ├── Install ca-certificates, tzdata
  ├── Set Asia/Shanghai timezone
  ├── Copy Atlas CLI (database migration)
  ├── Copy build artifact → /app/app
  ├── Copy migration scripts → /app/atlas/
  └── Entrypoint: run Atlas migration, then start service
```

### Entrypoint Script Logic

The `/entrypoint.sh` in the runtime container executes in this order:

1. If `EGOADMIN_ATLAS_MIGRATE` is not `false`, run `atlas/apply.sh` for database migration.
2. Set `EGOADMIN_ATLAS_MIGRATED=true` to mark migration completion.
3. `exec ./app` starts the service process (PID 1, supporting signal forwarding and graceful shutdown).

```bash
#!/bin/sh
set -e
if [ "${EGOADMIN_ATLAS_MIGRATE:-true}" = "true" ]; then
  cd /app && ./atlas/apply.sh
  export EGOADMIN_ATLAS_MIGRATED=true
fi
exec ./app "$@"
```

### Health Checks

Each service configures a Docker health check using the `/readyz` endpoint:

```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -q -O - http://127.0.0.1:9001/readyz >/dev/null"]
  interval: 10s
  timeout: 5s
  retries: 12
  start_period: 60s
```

The `/readyz` endpoint only returns 200 after database migration completes and the service is ready, ensuring orderly startup of the dependency chain.

### Compose Dependency Chain

```text
mysql-* ─┐
redis-* ─┤
etcd     ├──→ idgen ──→ user ──→ gateway
minio    ┘       │          │
jaeger ──────────┘          │
image-processor ─────────────┘
```

`depends_on` with `condition: service_healthy` ensures middleware is healthy before application services start.

## Common Issues

### Build Timeout Due to Network

If Go module downloads time out, check the `GOPROXY` setting:

```bash
# Already set by default in Dockerfile, override as needed
ARG GOPROXY=https://goproxy.cn,direct
```

You can also override at build time:

```bash
docker build --build-arg GOPROXY=https://proxy.golang.org,direct \
  -f cmd/gateway/Dockerfile -t egoadmin-gateway:dev .
```

### Container Cannot Connect to Middleware

Make sure to use Compose service names instead of `127.0.0.1`:

```bash
# Wrong (inside container)
EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_gateway"

# Correct (inside container)
EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(mysql-gateway:3306)/egoadmin_gateway"
```

### Atlas Migration Failure Prevents Service Startup

Temporarily skip migration to diagnose:

```bash
EGOADMIN_ATLAS_MIGRATE=false docker compose up gateway
```

::: warning
Skipping migration causes database schema and code to be out of sync. Use this only for diagnosing environment issues.
:::

### Image Size Optimization

The final alpine image contains only runtime essentials:

| Content | Description |
|---------|-------------|
| `./app` | Service binary |
| `./atlas/` | Atlas CLI + migration scripts |
| `/usr/local/bin/atlas` | Database migration tool |
| System deps | ca-certificates, tzdata |

Total image size is typically in the 50-80 MB range (depending on binary size).

## Reference Links

- [Docker Official Documentation](https://docs.docker.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Alpine Linux Image](https://hub.docker.com/_/alpine)
- [Atlas Migration Tool](https://atlasgo.io/)
- [Testing & Deployment](/guide/en-US/testing-deployment)
- [Runtime Configuration](/guide/en-US/configuration)
