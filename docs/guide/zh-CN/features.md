# 核心功能

本章是 EgoAdmin 的能力地图。它不替代后续深度章节，而是帮助你快速判断某类需求应该进入哪些目录、修改哪些层、运行哪些验证命令。

## 能力总览

| 能力 | 主要目录 | 关键命令 | 深度章节 |
|------|----------|----------|----------|
| 微服务运行时 | `cmd/*`, `internal/app/*/server`, `configs/*` | `make run SERVICE=user` | [微服务架构](/guide/architecture) |
| API 开发 | `api/proto`, `controller`, `application`, `adapter` | `make gen` | [API 开发工作流](/guide/api-development) |
| 权限系统 | `authsession`, `routeMenu.ts`, `api-manifest.ts` | `cd web && pnpm run build` | [权限系统](/guide/permission-system) |
| 数据库迁移 | `internal/app/*/schema`, `atlas/migrations/*` | `make migrate.new SERVICE=user NAME=xxx` | [数据库与迁移](/guide/database-migration) |
| 前端页面 | `web/src/views`, `web/src/api`, `web/src/router` | `cd web && pnpm run type-check` | [前端开发](/guide/frontend-development) |
| 分布式事务 | `application`, `proto-internal`, DTM branch API | `make e2e E2E_TIMEOUT=20m` | [分布式事务](/guide/distributed-transactions) |
| 组件系统 | `internal/component`, `internal/platform` | `go test ./internal/component/...` | [组件系统](/guide/components) |
| 配置体系 | `configs/*/config.toml`, `deploy/configs/*` | `make service.config SERVICE=user` | [运行时配置](/guide/configuration) |
| 测试部署 | `test/e2e`, `deploy`, `scripts/make` | `make e2e`, `make deploy-up` | [测试与部署](/guide/testing-deployment) |

## 微服务边界

EgoAdmin 默认包含三个服务：

| 服务 | 当前职责 |
|------|----------|
| `gateway` | 对外 HTTP 兼容入口、内嵌前端、上传、API/权限聚合、调用 user/idgen |
| `user` | 用户、角色、部门、认证会话、权限、审计日志、数据权限 |
| `idgen` | 雪花 ID、号段生成、机器租约、命名空间、稳定 ID 编解码 |

服务边界规则：

- 服务只能直接访问自己的数据库。
- 跨服务读写必须走 `internal/client/*` gRPC wrapper。
- gateway 不直接访问 user/idgen 的 store、domain 或 adapter。
- user/idgen 不依赖 gateway 的业务实现。
- 跨服务写一致性优先判断是否能避免，确实需要时使用 DTM。

## 后端分层

每个服务使用一致的目录结构：

```text
internal/app/<service>/
├── server/       # EGO 运行时、gRPC/HTTP/governor 注册、健康检查、迁移、关闭
├── controller/   # gRPC controller，负责协议转换
├── application/  # 用例编排、事务、权限、跨服务调用、DTM
├── domain/       # 聚合、值对象、领域错误、Repository 接口
├── adapter/      # MySQL/Redis/MinIO 等基础设施适配
└── schema/       # GORM 迁移模型列表
```

依赖方向：

```text
server -> controller -> application -> domain
                              |
                              v
                         adapter/persistence/mysql
```

::: tip
`domain` 层保持纯净，不导入 protobuf、GORM、EGO/Gin、Redis、MinIO、Casbin adapter 或其他服务 app 包。
:::

## Proto 优先 API

业务 API 从 `.proto` 开始，不手写独立 Gin 业务路由。

```protobuf
rpc GetRoleList(GetRoleListRequest) returns (GetRoleListResponse) {
  option (google.api.http) = {
    post: "/user.v1.RoleService/GetRoleList"
    body: "*"
  };
}
```

前端请求同样使用 gRPC HTTP 兼容路径：

```ts
api.post('/user.v1.RoleService/GetRoleList', {
  page: 1,
  limit: 20,
  name: '',
})
```

关键约束：

- HTTP 兼容接口统一使用 `POST`。
- 路径格式为 `/<package>.<Service>/<Method>`。
- request body 使用 protobuf JSON 字段名，例如 `deptId`、`roleIds`。
- OpenAPI 文档、校验标签和权限描述写在 proto 中。

## 权限闭环

权限不是单点控制，而是一条闭环：

```text
authsession Bearer token
  -> API 分类（public / login-only / protected）
  -> Casbin gRPC service/method 权限
  -> DataScope 数据权限
  -> API manifest
  -> routeMenu.ts
  -> permission-contract.json
  -> 角色授予边界校验
```

权限 ID 使用 gRPC 方法身份：

```text
USER.V1.ROLESERVICE/ADDROLE
USER.V1.USERSERVICE/GETUSERLIST
```

不要使用 REST 路径、前端路由路径、菜单 ID 或小写 service 名称作为后端权限标识。

## 数据库与迁移

迁移由 Atlas 管理，每个服务一个迁移目录：

```text
atlas/migrations/gateway
atlas/migrations/user
atlas/migrations/idgen
```

新增字段或表的标准流程：

```bash
# 1. 修改服务自己的 GORM model
# 2. 更新 internal/app/<service>/schema/MigrationModels()
# 3. 生成迁移
make migrate.new SERVICE=user NAME=add_profile_field

# 4. 校验迁移
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

运行时配置示例：

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

## 前端工程

前端位于 `web/`，与后端同仓：

```text
web/src/
├── api/          # API 调用与 api-manifest
├── config/       # routeMenu 权限菜单
├── router/       # 路由与守卫
├── store/        # Pinia 状态
├── views/        # 页面
├── components/   # 通用组件
├── i18n/         # 多语言
└── styles/       # 样式与主题
```

新增页面通常需要同时修改：

| 层 | 文件 |
|----|------|
| API 模块 | `web/src/api/modules/<domain>.ts` |
| 页面 | `web/src/views/<domain>/<page>/index.vue` |
| 路由 | `web/src/router/modules/<domain>.ts` |
| 权限菜单 | `web/src/config/routeMenu.ts` |
| 国际化 | `web/src/i18n/*` |
| 权限合约 | `web/dist/permission-contract.json`（构建生成） |

前端 ID 规则：

```ts
// 后端 uint64 / fixed64 ID 必须作为字符串处理
const userId = ref<string>('')
const roleIds = ref<string[]>([])
```

## DTM 分布式事务

单服务写入使用本地事务，跨服务写入才考虑 DTM。

```go
gid := dtmgrpc.MustGenGid(dtmServer)

err := dtmgrpc.NewSagaGrpc(dtmServer, gid).
  Add(
    userTarget+"/user.v1.UserDtmService/Action",
    userTarget+"/user.v1.UserDtmService/Compensate",
    req,
  ).
  Submit()
```

DTM 规则：

- 编排代码位于 `application`。
- branch API 从 proto 定义。
- branch handler 只修改本服务数据库。
- RM 服务数据库需要 barrier 表。
- 需要覆盖成功、补偿、等幂、空补偿、悬挂、DTM 不可用、branch 服务不可用等测试场景。

## 运行时配置

每个服务有独立配置：

```text
configs/gateway/config.toml
configs/user/config.toml
configs/idgen/config.toml
```

打印内置默认配置：

```bash
make service.config SERVICE=gateway
make service.config SERVICE=user
make service.config SERVICE=idgen
```

核心配置块：

| 配置块 | 用途 |
|--------|------|
| `[app.service]` | 服务名、平台名、业务开关 |
| `[app.web]` | gateway 注入前端的运行时配置 |
| `[app.dbMigration]` | Atlas 迁移 |
| `[server.http]` | HTTP 服务器 |
| `[server.grpc]` | gRPC 服务器 |
| `[client.mysql]` | MySQL |
| `[client.redis]` | Redis |
| `[client.grpc.*]` | 跨服务 gRPC 客户端 |
| `[etcd]` / `[registry]` | 服务发现和注册 |
| `[trace]` | OpenTelemetry |

## 测试与质量门禁

| 修改类型 | 最小验证 |
|----------|----------|
| proto / Wire / provider | `make gen` |
| 服务结构 | `make service.check SERVICE=user` |
| 后端逻辑 | `go test -race ./...` |
| 前端页面 | `cd web && pnpm run type-check` |
| 权限 / routeMenu | `cd web && pnpm run build` |
| 迁移 | `make migrate.validate SERVICE=user` |
| gateway 用户可见流程 | `make e2e E2E_TIMEOUT=20m` |

