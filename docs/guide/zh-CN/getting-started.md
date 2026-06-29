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

每个中间件的作用：

| 中间件 | 用途 | 服务依赖 |
|--------|------|----------|
| MySQL | 关系数据存储，每个服务有独立数据库实例，schema 通过 Atlas 管理 | gateway, user, idgen |
| Redis | JWT session 存储、Casbin 策略缓存、异步任务队列、JetCache 多级缓存 | gateway, user |
| etcd | 服务注册与发现，gRPC 客户端通过 `etcd:///` 前缀自动解析后端实例 | gateway, user, idgen |
| MinIO | 文件对象存储，支持上传断点续传(TUS)和 CDN 签名分发 | gateway |
| Jaeger | OpenTelemetry 链路追踪采集，通过 OTLP gRPC 协议接入 | gateway, user, idgen |
| DTM | 分布式事务管理器，协调 Saga/TCC 跨服务写入一致性 | gateway, user |

::: tip
开发初期如果只改 user 服务的业务逻辑，可以只启动 MySQL user、Redis user 和 etcd，运行 `make run SERVICE=user` 即可。不需要启动全部中间件。
:::

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

### 4. 环境变量覆盖

EgoAdmin 支持通过环境变量覆盖任何 TOML 配置项。规则是将 TOML 路径转为大写下划线格式，加上服务前缀：

```text
TOML 路径                      环境变量
─────────────────────────────  ──────────────────────────────────
[server.http] port = 9001     EGOADMIN_SERVER_HTTP_PORT=9001
[client.mysql] dsn = "..."    EGOADMIN_CLIENT_MYSQL_DSN="..."
[app.user] jwtExpire = 604800  EGOADMIN_APP_USER_JWTEXPIRE=604800
[etcd] addrs = ["..."]        EGOADMIN_ETCD_ADDRS="127.0.0.1:2379"
```

常用覆盖示例：

```bash
# 修改 user 服务的 MySQL 连接
export EGOADMIN_CLIENT_MYSQL_DSN="root:secret@tcp(db-server:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

# 修改 JWT 过期时间（秒）
export EGOADMIN_APP_USER_JWTEXPIRE=86400

# 修改 etcd 地址
export EGOADMIN_ETCD_ADDRS="etcd-1:2379,etcd-2:2379"

# 修改 HTTP 端口
export EGOADMIN_SERVER_HTTP_PORT=8001

make run SERVICE=user
```

::: warning
环境变量覆盖在服务启动时一次性读取，不支持热更新。数组类型（如 `addrs`）使用逗号分隔。Duration 类型（如 `connectTimeout`）使用 Go duration 格式，如 `"3s"`。
:::

在 Docker 容器中运行时，推荐使用环境变量而非挂载配置文件：

```bash
docker run -p 9101:9101 \
  -e EGOADMIN_CLIENT_MYSQL_DSN="egoadmin:egoadmin@tcp(mysql:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local" \
  -e EGOADMIN_CLIENT_REDIS_ADDR="redis:6380" \
  -e EGOADMIN_ETCD_ADDRS="etcd:2379" \
  -e EGOADMIN_REGISTRY_SERVICETTL="30s" \
  ghcr.io/egoadmin/egoadmin-user:latest
```

::: tip
也可以将环境变量写入项目根目录的 `.env` 文件，服务启动时会自动加载。
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

每个服务都暴露 HTTP 健康检查，不需要鉴权：

```bash
# gateway
curl http://localhost:9001/healthz   # 存活探针，进程在就返回 200
curl http://localhost:9001/readyz    # 就绪探针，迁移完成、依赖就绪后返回 200

# user
curl http://localhost:9101/healthz
curl http://localhost:9101/readyz

# idgen
curl http://localhost:9201/healthz
curl http://localhost:9201/readyz
```

::: warning
`user` 和 `idgen` 的健康检查走 HTTP 端口（9101、9201），不是 gRPC 端口（9102、9202）。governor 端口（9103、9203）也暴露了相同的健康检查端点。
:::

判断服务是否真正就绪：

```bash
# readyz 返回 200 才表示服务可以接收请求
curl -s -o /dev/null -w "%{http_code}" http://localhost:9101/readyz
# 预期: 200
```

::: danger
如果 readyz 一直返回 503，说明依赖（数据库、etcd 等）未就绪。查看服务日志确认具体是哪个组件连接失败。
:::

### 快速验证清单

完成本地开发启动后，按顺序验证以下项目：

```bash
# 1. 中间件健康
curl -s http://localhost:2379/health          # etcd
mysqladmin -h 127.0.0.1 -P 3306 ping         # MySQL gateway
mysqladmin -h 127.0.0.1 -P 3307 ping         # MySQL user
redis-cli -p 6379 -a egoadmin ping           # Redis gateway

# 2. 服务就绪
curl -s http://localhost:9001/readyz          # gateway
curl -s http://localhost:9101/readyz          # user
curl -s http://localhost:9201/readyz          # idgen

# 3. 前端页面可访问
curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/
# 预期: 200

# 4. 登录 API 调用
curl -X POST http://localhost:9001/api/user.v1.UserService/GetLoginCrypto \
  -H 'Content-Type: application/json' \
  -d '{}'
# 预期: 返回 JSON 包含 challenge 和 publicKey

# 5. 权限合约文件存在
test -f web/dist/permission-contract.json && echo "permission contract OK"
```

::: tip
完整的登录流程是：`GetLoginCrypto` -> 获取 RSA 公钥加密密码 -> `Login` -> 返回 Bearer token。前端已封装该流程，手动验证只需确认 `GetLoginCrypto` 能正常返回。
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

### 第一次启动问题

| 现象 | 重点排查 |
|------|----------|
| gateway 启动失败 | `web/dist/index.html` 是否存在；`configs/gateway/config.toml` 是否正确 |
| user/idgen 注册失败 | etcd 是否可用；`EGO_NAME` 是否为 `egoadmin-user` / `egoadmin-idgen` |
| 数据库迁移失败 | `app.dbMigration.url` 是否指向正确服务数据库；迁移目录是否匹配服务 |
| 权限保存失败 | `web/dist/permission-contract.json` 是否已生成；`routeMenu.ts` 绑定的 API 是否存在 |
| 前端 ID 精度异常 | protobuf `uint64` ID 在前端必须作为字符串处理 |

### Gateway 启动报 `embed: pattern "index.html" not found`

gateway 使用 `go:embed` 内嵌前端静态资源。如果 `web/dist/index.html` 不存在，编译会失败。

```bash
# 方案 1：先构建前端
cd web && pnpm install && pnpm run build

# 方案 2：跳过前端构建（仅后端开发时）
export SKIP_WEB_BUILD=1
make run SERVICE=gateway
```

::: danger
`SKIP_WEB_BUILD=1` 要求 `web/dist` 目录至少存在一个 `index.html`，否则编译仍然会失败。可以先 `mkdir -p web/dist && touch web/dist/index.html` 创建占位文件。
:::

### etcd 连接超时

症状：gateway 日志中出现 `context deadline exceeded` 或 `connection refused`。

```bash
# 确认 etcd 是否启动
curl http://localhost:2379/health

# 如果 etcd 没有启动，只启动 etcd
make dev.up-one MIDDLEWARE=etcd

# 确认服务是否已注册到 etcd
etcdctl get --prefix egoadmin --keys-only
```

### MySQL 连接拒绝

症状：日志中出现 `dial tcp 127.0.0.1:3307: connect: connection refused`。

```bash
# 确认 MySQL 容器是否运行
docker ps | grep mysql-user

# 重启 MySQL 容器
make dev.reset-one MIDDLEWARE=mysql-user

# 手动验证连接
mysql -h 127.0.0.1 -P 3307 -u egoadmin -pegoadmin egoadmin_user -e "SELECT 1"
```

### Redis 连接失败

症状：`redis: connection refused` 或 `WRONGPASS` 错误。

```bash
# 验证 Redis 可达
redis-cli -p 6380 -a egoadmin ping

# 注意 user 和 gateway 使用不同的 Redis 实例
# gateway: 127.0.0.1:6379
# user:    127.0.0.1:6380
```

### 迁移失败: `relation "xxx" does not exist` 或 `table already exists`

```bash
# 查看当前迁移状态
atlas migrate status --dir file://atlas/migrations/user \
  --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"

# 如果迁移文件和数据库不一致，开发环境可以重置
make dev.reset-one MIDDLEWARE=mysql-user
```

::: warning
生产环境严禁直接重置数据库。使用 `atlas migrate apply` 按顺序执行迁移，或联系 DBA 处理。
:::

### 前端页面白屏

```bash
# 1. 确认前端已构建
test -f web/dist/index.html && echo "OK" || echo "需要先构建前端"

# 2. 确认 gateway 日志中有 HTTP 请求记录
# 3. 检查浏览器控制台是否有 API 请求报错
# 4. 确认后端服务 readyz 返回 200
curl http://localhost:9101/readyz
```

