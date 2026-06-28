# EgoAdmin

基于 [EGO](https://github.com/gotomicro/ego) 框架的微服务后台管理模板，包含 gateway、user、idgen 三个服务，内嵌 Vue 3 + Element Plus 前端。

## 快速开始

### Docker Compose 一键部署

```bash
# 克隆仓库
git clone https://github.com/egoadmin/egoadmin.git
cd egoadmin

# 启动全部服务（含中间件）
make deploy-up

# 访问 http://localhost:9001
# 默认管理员：admin / 123456
```

镜像默认从 GitHub Container Registry 拉取，无需额外配置。

### 仅运行单个服务

```bash
# gateway（内嵌前端，推荐入口）
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

### Release 附件

| 文件 | 说明 |
|------|------|
| `Source code (tar.gz)` | 源码包（GitHub 自动生成） |
| `vendor.tar.gz` | Go vendor 依赖包，离线构建可用 |
| `openapi.tar.gz` | OpenAPI 接口规范（`api/openapi/openapi.yaml`） |
| `egoadmin-tools-linux-amd64.tar.gz` | 开发工具包（protoc、buf、wire、atlas 等） |

```bash
# 下载工具包，解压后加入 PATH 即可使用
curl -L https://github.com/egoadmin/egoadmin/releases/latest/download/egoadmin-tools-linux-amd64.tar.gz | tar xz
export PATH="$PWD/egoadmin-tools-linux-amd64:$PATH"

# 下载 API 文档
curl -L https://github.com/egoadmin/egoadmin/releases/latest/download/openapi.tar.gz | tar xz
```

### 发布镜像

所有镜像发布在 `ghcr.io`，支持 `linux/amd64` 和 `linux/arm64`：

| 镜像 | 说明 |
|------|------|
| `ghcr.io/egoadmin/egoadmin-gateway` | 网关服务（内嵌前端） |
| `ghcr.io/egoadmin/egoadmin-user` | 用户服务 |
| `ghcr.io/egoadmin/egoadmin-idgen` | ID 生成服务 |
| `ghcr.io/egoadmin/mysql` | MySQL 8.4.5 |
| `ghcr.io/egoadmin/redis` | Redis 8.0 |
| `ghcr.io/egoadmin/etcd` | etcd v3.5.15 |
| `ghcr.io/egoadmin/minio` | MinIO |
| `ghcr.io/egoadmin/dtm` | DTM 分布式事务 |
| `ghcr.io/egoadmin/meilisearch` | Meilisearch 搜索 |
| `ghcr.io/egoadmin/jaeger` | Jaeger 链路追踪 |
| `ghcr.io/egoadmin/imagor` | 图片处理 |

---

## 模板初始化与重命名

推荐先安装模板工具，再直接创建派生项目：

```bash
# 安装模板工具
go install github.com/egoadmin/egoadmin/tools/egoadminctl@latest

# 从默认 EgoAdmin 模板仓库初始化新项目并自动重命名
egoadminctl init --dest ../demoadmin --name DemoAdmin --slug demoadmin --module github.com/acme/demoadmin --env-prefix DEMOADMIN

# 使用指定模板仓库
egoadminctl init --repo <git-url> --dest ../demoadmin --name DemoAdmin --slug demoadmin --module github.com/acme/demoadmin --env-prefix DEMOADMIN
```

在模板仓库内维护或调整当前项目时，也可以使用 Make 入口：

```bash
# 重命名当前项目，默认只 dry-run
make template.rename NEW_NAME=DemoAdmin NEW_SLUG=demoadmin NEW_MODULE=github.com/acme/demoadmin ENV_PREFIX=DEMOADMIN

# 确认输出后写入
make template.rename NEW_NAME=DemoAdmin NEW_SLUG=demoadmin NEW_MODULE=github.com/acme/demoadmin ENV_PREFIX=DEMOADMIN APPLY=1
```

模板身份记录在 `.egoadmin/template.json`。工具会同步修改 Go module、Buf 生成前缀、根包名、服务名、服务发现名、数据库名、本地 Docker Compose 中间件账号与 bucket、环境变量前缀、文档和测试断言。默认跳过 `.agents`、`.claude`、`api/gen`、`api/openapi`、`api/catalog`、`web/dist`、`web/node_modules` 和本地运行数据目录。初始化完成后进入目标目录执行 `go mod tidy` 和 `make gen`。

## 本地开发

常用开发入口：

```bash
make install
make dev-up
make run
```

`make run` 默认按 `SERVICES="idgen user gateway"` 依次启动 idgen、user、gateway，便于本地联调。只调试单个服务时使用：

```bash
make run SERVICE=idgen
make run SERVICE=user
make run SERVICE=gateway
```

单个中间件可独立操作：

```bash
make dev.up-one MIDDLEWARE=mysql-user
make dev.down-one MIDDLEWARE=mysql-user
make dev.reset-one MIDDLEWARE=mysql-user
```

## 前端同仓与构建

前端项目直接放在 `web/` 目录，由主仓库统一管理，不再使用 Git submodule。首次拉取后直接安装前端依赖即可。

前端使用 `vite-plus` 和 `pnpm`：

```bash
cd web
pnpm install
pnpm run build
```

`pnpm run build` 会执行类型检查、前端打包，并生成 `dist/permission-contract.json`。后端通过 `go:embed` 内嵌 `web/dist`，因此编译 gateway 或运行全量 Go 测试前必须先有前端产物。仓库 Makefile 会在相关 gateway 路径自动确保 `web/dist` 存在；CI 也会先构建同仓前端。

后端启动后会向 `index.html` 注入运行时配置：

```toml
[app.web]
apiBaseUrl = ""               # 空值表示前端 API 使用当前域名 /api
fileBaseUrl = ""              # 文件/图片访问地址前缀
offlineOnPageLeave = false    # 是否在最后一个浏览器标签页离开时主动登出，默认关闭
```

角色新增/编辑会读取内嵌的 `permission-contract.json`，校验勾选菜单允许授予的 API ID。开发调试如确实需要临时跳过，可在本地配置中设置：

```toml
[app.service]
skipPermissionContractCheck = true
```

## 数据库版本迁移

项目使用 `gorm + atlas-provider-gorm` 生成 Atlas 版本迁移文件，部署阶段默认使用 Atlas 执行迁移。GORM `AutoMigrate` 只保留为本地开发兜底能力，配置里的 `app.service.autoMigrate` 默认应保持 `false`。

### 开发机生成迁移

1. 安装依赖和生成代码：

```bash
make install
make gen
```

2. `make install` 会安装官方版 Atlas CLI。开发机生成 GORM diff 需要 `external_schema`，community 版不支持；如果手动安装，不要加 `--community`：

```bash
curl -sSf https://atlasgo.sh | ATLAS_VERSION=v1.2.2 sh
atlas version
```

3. 修改服务拥有的 GORM model 后，按服务生成新的迁移版本：

```bash
make migrate.new SERVICE=gateway NAME=<change_name>
make migrate.new SERVICE=idgen NAME=<change_name>
make migrate.new SERVICE=user NAME=<change_name>
```

该命令等价于带 `--var service=<gateway|idgen|user> --var dialect=mysql` 的 `atlas migrate diff <change_name> --env gorm --config file://atlas/atlas.hcl`。

默认 MySQL 生成结果会写入 `atlas/migrations/<service>`，并更新该目录的 `atlas.sum`。迁移文件需要随代码一起提交。

Atlas loader 支持按方言生成 schema 源，默认 `mysql`：

```bash
go run ./tools/atlasloader --service user --dialect mysql
go run ./tools/atlasloader --service user --dialect postgres
go run ./tools/atlasloader --service user --dialect sqlite
go run ./tools/atlasloader --service user --dialect sqlserver
```

非 MySQL migration 作为扩展能力，目录按方言隔离，例如：

```bash
make migrate.new SERVICE=user NAME=<change_name> DIALECT=postgres
# 写入 atlas/migrations/postgres/user
```

如果只想审计最终 schema，而不是生成迁移，可生成 Atlas HCL 快照：

```bash
make migrate.schema SERVICE=user
make migrate.schema SERVICE=user DIALECT=postgres
```

HCL 快照写入 `atlas/schema/<dialect>/<service>.hcl`，用于评审和审计。当前项目的正式迁移源仍然是 GORM model + 版本化 SQL migration，不手写维护 HCL 作为唯一 schema 源。

GORM schema 的来源按服务分开：

- `GormConfig()` 定义运行时和 Atlas loader 共同使用的 GORM schema 配置。
- `ApplyGormConfig()` 将同一份配置应用到运行时 DB。
- `internal/app/gateway/schema.MigrationModels()` 定义 gateway 服务参与迁移的 model 列表。
- `internal/app/idgen/schema.MigrationModels()` 定义 idgen 服务参与迁移的 model 列表。
- `internal/app/user/schema.MigrationModels()` 定义 user 服务参与迁移的 model 列表。
- `MigrationJoinTables()` 定义服务内自定义 join table，例如 user 服务的 `user_role`。

`casbin_rule` 属于 user 服务授权数据，使用 `github.com/casbin/gorm-adapter/v3.CasbinRule` 的官方结构定义迁移模型。应用启动顺序保证 Atlas 先执行，再初始化 Casbin，避免 Casbin 抢先建表导致 Atlas 判断数据库为 dirty。

新增或调整表结构时只维护服务自己的 `internal/app/<service>/internal/store` 包，并通过服务根目录的 `schema` 包导出迁移模型，不要在 `tools/atlasloader` 里重复写 model 或 join table 清单。

4. 校验迁移目录：

```bash
make migrate.validate SERVICE=gateway
make migrate.validate SERVICE=idgen
make migrate.validate SERVICE=user
make migrate.hash SERVICE=gateway
make migrate.hash SERVICE=idgen
make migrate.hash SERVICE=user
```

### 普通部署迁移

普通二进制部署需要安装 `atlas` 命令，并提供数据库连接：

```bash
export ATLAS_URL='mysql://user:password@host:3306/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local'
./egoadmin --config=./configs/gateway/config.toml
```

配置项：

```toml
[app.service]
autoMigrate = false

[app.dbMigration]
enabled = true
driver = "atlas"
url = "${ATLAS_URL}"
dir = "file://atlas/migrations/gateway"
bin = "atlas"
```

应用启动时会先执行：

```bash
atlas migrate apply --url "$ATLAS_URL" --dir "file://atlas/migrations/gateway"
```

迁移失败会直接阻止应用启动。

### Docker 部署迁移

Docker 镜像内包含 Atlas CLI 和 `atlas/migrations`。容器入口默认会先执行 Atlas 迁移，再启动应用：

```bash
docker run \
  -e ATLAS_URL='mysql://user:password@host:3306/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local' \
  egoadmin:latest
```

如需临时跳过容器入口迁移，可设置：

```bash
EGOADMIN_ATLAS_MIGRATE=false
```

### 已有数据库上线

已有数据库不能直接在生产库上重放 `initial` 迁移。上线前应先对现有库做 Atlas baseline，让 `atlas_schema_revisions` 记录当前版本，再在后续版本使用正常的 `atlas migrate apply` 流程。

## ego comp 组件

项目内置的 ego component 放在 `internal/component`。当前包括：

- `eredis`：基于 `github.com/redis/go-redis/v9` 的 Redis 组件。
- `jetcache`：基于 Redis 的本地+远程缓存组件。
- `asyncq`：基于 Asynq 的异步任务组件。
- `meilisearch`：Meilisearch 搜索客户端组件。
- `etusupload`：基于 TUS 协议的断点续传组件。

### Redis

运行时 Redis 已从官方 `github.com/gotomicro/ego-component/eredis` 切换为本项目的 `internal/component/eredis`。平台级 Redis wrapper 位于 `internal/platform/cache/redis`，服务自有缓存和 key 归服务私有目录，例如 `internal/app/user/internal/cache`，避免服务层直接依赖底层组件。

配置键已统一为 `[client.redis]`：

```toml
[client.redis]
addr = "127.0.0.1:6379"
mode = "stub"
password = "change-me"
debug = true
```

旧的 `[client.redis.stub]` 不再作为默认配置使用。新组件支持 `stub`、`cluster`、`sentinel`、`ring`，并提供 debug、access log、metric、trace hook 和 ecron 分布式锁。

### 可选组件

`jetcache`、`asyncq`、`meilisearch`、`etusupload` 当前只提供组件能力和配置示例，不进入主服务 Wire 启动链路。未显式注入时，它们不会阻塞 `make run`、普通部署或 Docker 部署。

`jetcache` 示例：

```toml
[client.jetcache]
name = "default"
remoteExpiry = "1h0m0s"
localSize = 256
localExpiry = "1m0s"
refreshDuration = "2m0s"
stopRefreshAfter = "1h0m0s"
notFoundExpiry = "1m0s"
enableMetrics = true
enableSyncLocal = false
codec = "msgpack"
```

`jetcache` 依赖 `[client.redis]`。只有 `enableSyncLocal=true` 时才会订阅 Redis pubsub channel 用于本地缓存同步。当前不会替换 `internal/platform/cache/local`。

`asyncq` 示例：

```toml
[client.asyncq]
redisAddr = "127.0.0.1:6379"
redisPassword = "change-me"
redisDB = 0
enableClient = true
enableServer = false
concurrency = 10
queues = { critical = 6, default = 3, low = 1 }
maxRetry = 3
retryDelayFunc = "exponential"
taskTimeout = "30s"
enableHealthCheck = false
```

`asyncq` 的健康检查会向 Redis 写入 `health_check` 任务，默认示例关闭健康检查，业务接入后按需要启用。

`meilisearch` 示例：

```toml
[client.meili]
host = "http://127.0.0.1:7700"
apiKey = ""
timeout = "5s"
enableHealth = true
ensureOnBuild = false
```

`ensureOnBuild` 默认关闭，避免搜索服务不可达时影响主服务启动。需要自动创建索引时再显式配置 indexes。

`etusupload` 示例：

```toml
[component.etusupload]
basePath = "/tus/upload"
maxSize = 1073741824
dataDir = "./data/tus"
uploadDir = "./uploads"
enableValidation = true
validateBeforeUpload = true
validateAfterUpload = true
allowAllOrigins = true
```

`etusupload` 是 TUS 断点续传能力，不替换当前 `/upload` 的 S3/MinIO multipart 上传。生产使用时建议通过上传完成 hook 将本地完成文件转存到 S3/MinIO。
