# 快速开始

本章面向第一次接触 EgoAdmin 的开发者，目标是让你完成三件事：

- 在本机启动完整微服务栈。
- 能登录管理后台并验证基础链路。
- 理解日常开发时应该运行哪些命令。

## 环境要求

| 组件 | 推荐版本 | 用途 |
|------|----------|------|
| Go | 1.26 | 后端编译、测试、工具运行 |
| Docker | 24+ | 本地中间件和部署栈 |
| Docker Compose | v2 | `make dev-up` / `make deploy-up` |
| Make | GNU Make | 项目统一命令入口 |
| Node.js | 18+ | 前端构建 |
| pnpm | 11.5.x | 前端包管理，见 `web/package.json` |

::: tip
执行 `make install` 会安装项目需要的开发工具，包括 Buf、Wire、Atlas、protoc 插件等。普通开发不建议手动逐个安装。
:::

## 一键部署

完整部署会启动 gateway、user、idgen 和 MySQL、Redis、etcd、MinIO、DTM、Jaeger、Meilisearch 等中间件。

```bash
git clone https://github.com/egoadmin/egoadmin.git
cd egoadmin

export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

访问：

```text
http://localhost:9001
```

默认管理员：

| 用户名 | 密码 |
|--------|------|
| `admin` | `123456` |

::: warning
默认密码只适合本地体验和初始化演示。生产环境需要修改 `configs/user/config.toml` 或环境变量中的管理员密码和 JWT secret。
:::

停止部署栈：

```bash
make deploy-down
```

## 本地开发启动

### 1. 安装工具链

```bash
make install
```

该命令实际代理到 `tools.install`，用于安装项目依赖的命令行工具。

### 2. 启动本地中间件

```bash
make dev-up
```

常用中间件端口：

| 中间件 | 默认地址 | 说明 |
|--------|----------|------|
| MySQL gateway | `127.0.0.1:3306` | `egoadmin_gateway` |
| MySQL user | `127.0.0.1:3307` | `egoadmin_user` |
| MySQL idgen | `127.0.0.1:3308` | `egoadmin_idgen` |
| Redis gateway | `127.0.0.1:6379` | gateway 缓存/队列 |
| Redis user | `127.0.0.1:6380` | user 缓存/队列 |
| etcd | `127.0.0.1:2379` | 服务注册与发现 |
| MinIO | `127.0.0.1:9000` | 对象存储 |
| Jaeger OTLP | `127.0.0.1:4317` | 链路追踪采集 |
| DTM HTTP | `127.0.0.1:36789` | 分布式事务管理器 |

单独控制某个中间件：

```bash
make dev.up-one MIDDLEWARE=mysql-user
make dev.down-one MIDDLEWARE=mysql-user
make dev.reset-one MIDDLEWARE=mysql-user
```

### 3. 启动服务

启动全部服务：

```bash
make run
```

默认启动顺序：

```text
idgen -> user -> gateway
```

只启动单个服务：

```bash
make run SERVICE=idgen
make run SERVICE=user
make run SERVICE=gateway
```

::: tip
启动 gateway 时，如果 `web/dist/index.html` 不存在，Makefile 会自动触发前端构建。需要跳过自动构建时可设置 `SKIP_WEB_BUILD=1`，但必须确保 `web/dist` 已存在。
:::

## 前端开发

前端项目位于 `web/`，使用 Vue 3、TypeScript、Element Plus、Pinia、Vue Router 和 Vite。

```bash
cd web
pnpm install
pnpm run dev
```

构建前端：

```bash
cd web
pnpm run build
```

`pnpm run build` 包含三个动作：

```text
vue-tsc --noEmit
vp build
node scripts/generate-contract.cjs
```

构建产物：

| 文件/目录 | 用途 |
|-----------|------|
| `web/dist/index.html` | gateway 通过 `go:embed` 内嵌 |
| `web/dist/assets/*` | 前端静态资源 |
| `web/dist/permission-contract.json` | 角色权限边界校验 |

## 健康检查

每个服务都暴露 HTTP 健康检查：

```bash
curl http://localhost:9001/healthz
curl http://localhost:9001/readyz

curl http://localhost:9101/healthz
curl http://localhost:9101/readyz

curl http://localhost:9201/healthz
curl http://localhost:9201/readyz
```

::: warning
`user` 和 `idgen` 的健康检查走 HTTP 端口，不是 gRPC 端口。例如 user 是 `9101`，不是 `9102`。
:::

## 最小验证流程

### 1. 验证 gateway 页面

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9001/
```

预期输出：

```text
200
```

### 2. 验证服务发现

gateway 依赖 etcd 发现 user 和 idgen：

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

如果 gateway 调用 user 超时，优先检查：

```bash
curl http://localhost:2379/health
```

### 3. 验证前端权限合约

```bash
test -f web/dist/permission-contract.json && echo ok
```

角色新增/编辑时，后端会使用该合约校验菜单允许授予的 API ID。

## 常用开发命令

| 目标 | 命令 |
|------|------|
| 安装工具链 | `make install` |
| 生成 proto / Go / Wire | `make gen` |
| 运行全部服务 | `make run` |
| 运行单个服务 | `make run SERVICE=user` |
| 构建全部服务 | `make build` |
| 构建单个服务 | `make build SERVICE=gateway` |
| 启动开发中间件 | `make dev-up` |
| 停止开发中间件 | `make dev-down` |
| 生成迁移 | `make migrate.new SERVICE=user NAME=add_field` |
| 校验迁移 | `make migrate.validate SERVICE=user` |
| 后端测试 | `go test ./...` |
| e2e 测试 | `make e2e E2E_TIMEOUT=20m` |
| 前端类型检查 | `cd web && pnpm run type-check` |
| 前端构建 | `cd web && pnpm run build` |

## 单服务 Docker 运行

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

## 常见问题

| 现象 | 重点排查 |
|------|----------|
| gateway 启动失败 | `web/dist/index.html` 是否存在；`configs/gateway/config.toml` 是否正确 |
| user/idgen 注册失败 | etcd 是否可用；`EGO_NAME` 是否为 `egoadmin-user` / `egoadmin-idgen` |
| 数据库迁移失败 | `app.dbMigration.url` 是否指向正确服务数据库；迁移目录是否匹配服务 |
| 权限保存失败 | `web/dist/permission-contract.json` 是否已生成；`routeMenu.ts` 绑定的 API 是否存在 |
| 前端 ID 精度异常 | protobuf `uint64` ID 在前端必须作为字符串处理 |

