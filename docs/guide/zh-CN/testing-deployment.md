# 测试与部署

## 测试分层

| 层级 | 命令 | 用途 |
|------|------|------|
| 单元测试 | `go test ./...` | 领域、用例、工具函数 |
| 竞态检测 | `go test -race ./...` | 并发安全 |
| 服务结构检查 | `make service.check SERVICE=user` | 目录、ProviderSet、注册、健康检查 |
| 前端类型检查 | `cd web && pnpm run type-check` | TypeScript |
| 前端构建 | `cd web && pnpm run build` | 打包 + 权限合约 |
| e2e | `make e2e E2E_TIMEOUT=20m` | gateway 用户可见链路 |

## 服务结构检查

```bash
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
```

检查内容包括：

- `cmd/<service>/Dockerfile`
- `internal/app/<service>/server`
- `controller`
- `application`
- `domain`
- `adapter`
- `schema`
- `atlas/migrations/<service>/atlas.sum`
- gRPC service registration
- HTTP `/healthz` 和 `/readyz`
- ProviderSet
- 架构依赖约束

## e2e 拓扑

```text
test client
  -> gateway HTTP compatibility
  -> user gRPC via etcd
  -> service DB/Redis
```

e2e 测试位于：

```text
test/e2e/gateway/
```

适合覆盖：

- 登录/登出。
- 用户管理。
- 角色权限。
- 部门树。
- 上传/CDN。
- 权限拒绝。
- 数据权限。

## 构建

```bash
make build
make build SERVICE=user
make build.alpine SERVICE=gateway
make build.alpine-arm64 SERVICE=idgen
```

gateway 构建前会确保 `web/dist/index.html` 存在。

## 镜像

```bash
make image.build
make image.build SERVICE=user
```

发布镜像：

| 镜像 | 说明 |
|------|------|
| `ghcr.io/egoadmin/egoadmin-gateway` | 网关服务（内嵌前端） |
| `ghcr.io/egoadmin/egoadmin-user` | 用户服务 |
| `ghcr.io/egoadmin/egoadmin-idgen` | ID 生成服务 |

## 部署

```bash
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

停止：

```bash
make deploy-down
```

部署编排：

```text
deploy/
├── docker-compose.yml
└── compose/
    ├── app.yml
    ├── mysql.yml
    ├── redis.yml
    ├── etcd.yml
    ├── minio.yml
    ├── dtm.yml
    ├── jaeger.yml
    ├── meilisearch.yml
    └── image-processor.yml
```

## 发布附件

| 文件 | 说明 |
|------|------|
| `vendor.tar.gz` | Go vendor 依赖，离线构建可用 |
| `openapi.tar.gz` | OpenAPI 接口规范 |
| `egoadmin-tools-linux-amd64.tar.gz` | protoc、buf、wire、atlas 等工具包 |

下载工具包：

```bash
curl -L https://github.com/egoadmin/egoadmin/releases/latest/download/egoadmin-tools-linux-amd64.tar.gz | tar xz
export PATH="$PWD/egoadmin-tools-linux-amd64:$PATH"
```

## 发布前检查清单

```bash
make gen
make lint
go test -race ./...
cd web && pnpm run build
make service.check SERVICE=gateway
make service.check SERVICE=user
make service.check SERVICE=idgen
make e2e E2E_TIMEOUT=20m
```

涉及数据库变更时补充：

```bash
make migrate.validate SERVICE=gateway
make migrate.validate SERVICE=user
make migrate.validate SERVICE=idgen
```

