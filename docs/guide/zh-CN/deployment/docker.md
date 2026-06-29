# Docker 容器化

EgoAdmin 使用多阶段 Docker 构建和 Docker Compose 统一管理本地开发与生产部署环境。

## 概述

EgoAdmin 的每个微服务（gateway、user、idgen）各自拥有独立的 Dockerfile，采用两阶段构建：Go 编译阶段和最小化运行时镜像。Docker Compose 负责编排全部服务和中间件依赖。

| 镜像 | 说明 | 构建文件 |
|------|------|----------|
| `egoadmin-gateway` | 网关服务，内嵌前端静态资源 | `cmd/gateway/Dockerfile` |
| `egoadmin-user` | 用户服务 | `cmd/user/Dockerfile` |
| `egoadmin-idgen` | ID 生成服务 | `cmd/idgen/Dockerfile` |

## 核心用法

### 构建镜像

```bash
# 构建所有服务镜像
make image.build

# 只构建指定服务
make image.build SERVICE=user
```

构建产出的镜像标签格式为 `${DOCKER_REGISTRY}/egoadmin-<service>:latest`。

### 启动完整部署栈

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

这会启动全部应用服务和中间件：

```text
deploy/
├── docker-compose.yml          # 主入口
└── compose/
    ├── app.yml                 # gateway、user、idgen
    ├── mysql.yml               # MySQL 实例
    ├── redis.yml               # Redis 实例
    ├── etcd.yml                # etcd 集群
    ├── minio.yml               # MinIO 对象存储
    ├── dtm.yml                 # DTM 分布式事务
    ├── jaeger.yml              # Jaeger 链路追踪
    ├── meilisearch.yml         # Meilisearch 全文搜索
    └── image-processor.yml     # 图片处理服务
```

### 管理开发中间件

```bash
# 启动本地开发中间件（MySQL、Redis、etcd 等）
make dev-up

# 停止本地开发中间件
make dev-down
```

开发中间件定义在 `test/compose/*.yml`，不包含应用服务，适合本地开发时配合 `make run` 使用。

### 停止部署栈

```bash
make deploy-down
```

## 配置示例

### 多阶段 Dockerfile

以 gateway 为例，完整的两阶段构建模式如下：

```dockerfile
# ---- 编译阶段 ----
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

# ---- 运行时阶段 ----
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

::: tip 编译参数
`-ldflags` 注入的构建信息会通过 EGO 框架暴露在 `/env` 端点上，方便运维确认部署版本。
:::

### 部署 Compose 应用配置

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

### 环境变量覆盖

所有配置项均可通过 `EGOADMIN_*` 环境变量覆盖，命名规则为将 TOML 配置路径中的 `.` 替换为 `_`，并转为大写。

```bash
# 等价于 [client.mysql] dsn = "..."
export EGOADMIN_CLIENT_MYSQL_DSN="user:pass@tcp(mysql:3306)/db"

# 等价于 [trace.otlp] Endpoint = "..."
export EGOADMIN_TRACE_OTLP_ENDPOINT="jaeger:4317"
```

::: warning 容器内服务发现
在容器内必须使用 Docker Compose 服务名（如 `mysql-gateway`、`redis-user`、`etcd`）代替 `127.0.0.1`。`127.0.0.1` 只在本地裸进程开发时使用。
:::

### 配置文件挂载

生产部署时，配置文件通过 volume 挂载到容器内：

```yaml
volumes:
  - ../configs/gateway/config.toml:/app/config/config.toml:ro
```

::: tip 只读挂载
配置文件以 `:ro` 只读模式挂载，防止运行时意外修改。敏感信息（数据库密码、JWT secret）优先使用环境变量注入。
:::

## 实际应用

### 本地开发流程

```text
开发者本机
  ├── make dev-up           # 启动中间件（MySQL/Redis/etcd/MinIO...）
  ├── make run SERVICE=gateway   # 本地编译运行 gateway
  ├── make run SERVICE=user      # 本地编译运行 user
  └── 浏览器 → http://localhost:9001
```

### 完整部署流程

```text
CI/CD 或运维
  ├── make image.build         # 构建 Docker 镜像
  ├── docker push ...          # 推送到镜像仓库
  └── make deploy-up           # 启动完整部署栈
       ├── gateway (HTTP :9001, gRPC :9002)
       ├── user    (HTTP :9101, gRPC :9102)
       ├── idgen   (gRPC :9202)
       └── 中间件 (MySQL/Redis/etcd/MinIO/DTM/Jaeger...)
```

### 端口规划

| 服务 | HTTP 端口 | gRPC 端口 | Governor 端口 |
|------|-----------|-----------|---------------|
| gateway | 9001 | 9002 | 9003 |
| user | 9101 | 9102 | 9103 |
| idgen | 9201 | 9202 | 9203 |

::: info Governor 端口
Governor 端口提供 `/healthz`、`/readyz`、`/debug/pprof/*` 和 `/env` 等运维端点，默认不暴露到宿主机。需要调试时可在 compose 文件中添加端口映射。
:::

## 工作原理

### 两阶段构建流程

```text
builder 阶段 (golang:1.26.2-alpine)
  ├── go mod download     # 缓存依赖层
  ├── COPY . .            # 复制源码
  └── go build            # 静态编译 → egoadmin 二进制

runtime 阶段 (alpine:3.18)
  ├── 安装 ca-certificates、tzdata
  ├── 设置 Asia/Shanghai 时区
  ├── 复制 Atlas CLI（数据库迁移）
  ├── 复制编译产物 → /app/app
  ├── 复制迁移脚本 → /app/atlas/
  └── 入口脚本：先运行 Atlas 迁移，再启动服务
```

### 入口脚本逻辑

运行时容器的 `/entrypoint.sh` 执行顺序：

1. 如果 `EGOADMIN_ATLAS_MIGRATE` 不为 `false`，运行 `atlas/apply.sh` 执行数据库迁移。
2. 设置 `EGOADMIN_ATLAS_MIGRATED=true` 标记迁移完成。
3. `exec ./app` 启动服务进程（PID 1，支持信号转发和优雅退出）。

```bash
#!/bin/sh
set -e
if [ "${EGOADMIN_ATLAS_MIGRATE:-true}" = "true" ]; then
  cd /app && ./atlas/apply.sh
  export EGOADMIN_ATLAS_MIGRATED=true
fi
exec ./app "$@"
```

### 健康检查

每个服务都配置了 Docker 健康检查，使用 `/readyz` 端点：

```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -q -O - http://127.0.0.1:9001/readyz >/dev/null"]
  interval: 10s
  timeout: 5s
  retries: 12
  start_period: 60s
```

`/readyz` 端点在数据库迁移完成、服务就绪后才返回 200，确保依赖链的有序启动。

### Compose 依赖链

```text
mysql-* ─┐
redis-* ─┤
etcd     ├──→ idgen ──→ user ──→ gateway
minio    ┘       │          │
jaeger ──────────┘          │
image-processor ─────────────┘
```

`depends_on` + `condition: service_healthy` 保证中间件健康后再启动应用服务。

## 常见问题

### 构建镜像时网络超时

如果 Go module 下载超时，检查 `GOPROXY` 设置：

```bash
# Dockerfile 中已默认设置，可按需修改
ARG GOPROXY=https://goproxy.cn,direct
```

也可以在构建时覆盖：

```bash
docker build --build-arg GOPROXY=https://proxy.golang.org,direct \
  -f cmd/gateway/Dockerfile -t egoadmin-gateway:dev .
```

### 容器无法连接中间件

确认使用 Compose 服务名而非 `127.0.0.1`：

```bash
# 错误（容器内）
EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(127.0.0.1:3306)/egoadmin_gateway"

# 正确（容器内）
EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(mysql-gateway:3306)/egoadmin_gateway"
```

### Atlas 迁移失败导致服务无法启动

临时跳过迁移以排查问题：

```bash
EGOADMIN_ATLAS_MIGRATE=false docker compose up gateway
```

::: warning
跳过迁移会导致数据库 schema 与代码不一致，仅用于排查环境问题。
:::

### 镜像体积优化

最终的 alpine 镜像仅包含运行时所需文件：

| 内容 | 说明 |
|------|------|
| `./app` | 服务二进制 |
| `./atlas/` | Atlas CLI + 迁移脚本 |
| `/usr/local/bin/atlas` | 数据库迁移工具 |
| 系统依赖 | ca-certificates、tzdata |

总镜像体积通常在 50-80 MB 范围内（取决于二进制大小）。

## 参考链接

- [Docker 官方文档](https://docs.docker.com/)
- [Docker Compose 文档](https://docs.docker.com/compose/)
- [Alpine Linux 镜像](https://hub.docker.com/_/alpine)
- [Atlas 迁移工具](https://atlasgo.io/)
- [测试与部署](/guide/zh-CN/testing-deployment)
- [运行时配置](/guide/zh-CN/configuration)
