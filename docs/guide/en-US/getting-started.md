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

## Frontend Development

```bash
cd web
pnpm install
pnpm run dev
pnpm run build
```

`pnpm run build` performs type checking, Vite build and permission contract generation.

## Health Checks

```bash
curl http://localhost:9001/readyz
curl http://localhost:9101/readyz
curl http://localhost:9201/readyz
```

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
