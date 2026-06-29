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

### 认证流程代码示例

后端通过 `authsession` 中间件从 Bearer token 解析用户上下文：

```go
// gateway authsession 中间件伪逻辑
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractBearerToken(r)
        if token == "" {
            // public API 不需要 token
            next.ServeHTTP(w, r)
            return
        }
        auth, err := userClient.InternalAuth.ValidateAccessToken(r.Context(), token)
        if err != nil {
            writeError(w, http.StatusUnauthorized, err)
            return
        }
        ctx := authsession.WithAuthContext(r.Context(), auth)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

在应用层获取当前用户：

```go
func (uc *UserUseCase) GetUserList(ctx context.Context, req *GetUserListQuery) (*UserListResult, error) {
    auth := authsession.FromContext(ctx)
    if auth == nil {
        return nil, ErrAuthRequired
    }

    // DataScope 数据权限过滤
    deptIDs, err := uc.dataScope.AllowedDeptIDs(ctx, auth.DeptID, auth.UserID)
    if err != nil {
        return nil, err
    }

    return uc.repo.FindByDeptIDs(ctx, deptIDs, req.Page, req.Limit)
}
```

### RBAC 权限配置示例

Casbin 策略通过 proto 方法标识控制 API 访问：

```text
# 策略格式: p, role_id, service, method
p, 1, USER.V1.ROLESERVICE, ADDROLE
p, 1, USER.V1.ROLESERVICE, DELETEROLE
p, 1, USER.V1.USERSERVICE, GETUSERLIST
p, 2, USER.V1.USERSERVICE, GETUSERLIST
```

::: tip
角色的权限策略绑定在 `web/src/config/routeMenu.ts` 中定义，构建时自动生成 `permission-contract.json`。后端校验时会比对授予的 API ID 是否在合约范围内。
:::

## 认证与登录安全

EgoAdmin 内置登录加密挑战机制，防止密码明文传输：

```text
前端请求 GetLoginCrypto
  -> 后端生成 RSA 公钥 + challenge（3 分钟过期）
  -> 前端用 RSA 公钥加密密码
  -> 前端用 challenge 派生 AES 密钥加密密码
  -> Login 时后端验证 challenge 和密码
```

配置项（`configs/user/config.toml`）：

```toml
[component.logincrypto]
challengeTTL = "3m0s"        # challenge 有效期
timestampSkew = "2m0s"       # 允许的客户端时间偏差
rsaKeyBits = 4096            # RSA 密钥长度
enableMetrics = true         # 是否上报指标

[app.user]
useCaptcha = false           # 是否启用登录验证码
jwtExpire = 604800           # JWT 有效期（秒）
refreshTokenExpire = 2592000 # 刷新 token 有效期（秒）
multiLoginEnabled = true     # 是否允许多端登录
maxLoginClient = 2           # 最大同时登录客户端数
```

## 国际化 (i18n)

后端和前端均支持中英文切换：

```text
后端: internal/platform/i18n/locales/    # 消息翻译文件
前端: web/src/i18n/                       # Vue I18n 翻译文件
```

后端错误消息国际化：

```go
// 在 controller 层翻译错误消息
func mapRoleError(ctx context.Context, err error) error {
    if errors.Is(err, roledomain.ErrNameExists) {
        return platformi18n.Errorf(ctx, "role.name.exists")
    }
    if errors.Is(err, roledomain.ErrInUse) {
        var inUseErr *roledomain.InUseError
        if errors.As(err, &inUseErr) {
            return platformi18n.Errorf(ctx, "role.in.use", inUseErr.Count)
        }
    }
    return err
}
```

前端 i18n 使用：

```vue
<template>
  <el-button>{{ $t('common.save') }}</el-button>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t, locale } = useI18n()
</script>
```

::: tip
后端通过 `Accept-Language` 请求头自动选择语言。gRPC 元数据会在服务间调用时自动传递（见 `userclient.outgoingContext`）。
:::

## 数据库迁移

迁移由 Atlas 管理，每个服务独立迁移目录：

```text
atlas/migrations/gateway/   # gateway 专属
atlas/migrations/user/      # user 专属
atlas/migrations/idgen/     # idgen 专属
```

标准迁移流程：

```bash
# 1. 修改 GORM model
# vim internal/app/user/adapter/persistence/mysql/user_model.go

# 2. 更新 schema 列表
# vim internal/app/user/schema/migration_models.go

# 3. 生成迁移 SQL
make migrate.new SERVICE=user NAME=add_phone_field

# 4. 校验迁移文件
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user

# 5. 启动服务时自动执行迁移
make run SERVICE=user
# 日志中应出现: atlas migration applied
```

运行时迁移配置：

```toml
[app.dbMigration]
enabled = true                    # 是否启用自动迁移
driver = "atlas"                  # 迁移工具
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"                     # atlas 可执行文件路径
```

::: warning
生产环境建议将 `app.dbMigration.enabled` 设为 `false`，由 CI/CD 流程手动执行 `atlas migrate apply`，避免多个实例同时执行迁移导致冲突。
:::

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

## 功能开关与配置

部分功能通过配置项控制开关，无需改代码：

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `app.user.useCaptcha` | bool | `false` | 登录验证码开关 |
| `app.user.multiLoginEnabled` | bool | `true` | 多端登录开关 |
| `app.user.heartbeatOfflineEnabled` | bool | `true` | 心跳离线检测开关 |
| `app.service.autoMigrate` | bool | `false` | 服务启动时自动 GORM AutoMigrate（仅开发用） |
| `app.dbMigration.enabled` | bool | `true` | Atlas 迁移开关 |
| `app.service.skipPermissionContractCheck` | bool | `false` | 跳过权限合约校验（开发初期用） |
| `server.http.enableCors` | bool | `false` | 跨域开关 |
| `server.http.enableMetricInterceptor` | bool | `false` | HTTP 请求指标采集 |
| `server.http.enableTraceInterceptor` | bool | `false` | HTTP 链路追踪 |
| `component.logincrypto.enableMetrics` | bool | `true` | 登录加密指标 |

通过环境变量临时覆盖：

```bash
# 关闭验证码（例如前端开发不需要验证码流程）
export EGOADMIN_APP_USER_USECAPTCHA=false

# 关闭 Atlas 迁移（例如使用外部迁移工具）
export EGOADMIN_APP_DBMIGRATION_ENABLED=false

# 开启 HTTP 请求指标
export EGOADMIN_SERVER_HTTP_ENABLEMETRICINTERCEPTOR=true
```

## 功能对比表：内置 vs 需配置

| 功能 | 内置 | 需配置 | 配置方式 |
|------|------|--------|----------|
| JWT 认证 | ✓ | | 开箱即用 |
| RBAC 权限 | ✓ | | Casbin 策略自动同步 |
| 登录加密挑战 | ✓ | | RSA + AES 双重加密 |
| 多端登录控制 | | ✓ | `app.user.multiLoginEnabled` |
| 登录验证码 | | ✓ | `app.user.useCaptcha` |
| 心跳离线检测 | | ✓ | `app.user.heartbeatOfflineEnabled` |
| 数据权限 DataScope | ✓ | | 角色维度配置 |
| 文件上传 (TUS) | ✓ | | MinIO 存储后端 |
| CDN 签名分发 | | ✓ | `component.cdn.signSecret` |
| 分布式事务 DTM | | ✓ | 需部署 DTM 服务 |
| 链路追踪 | | ✓ | `trace.*` 配置块 |
| 多语言 i18n | ✓ | | 中英文内置 |
| 审计日志 | ✓ | | 自动记录关键操作 |
| ID 生成 (雪花) | ✓ | | idgen 服务自动提供 |
| 号段 ID 分配 | ✓ | | idgen 自动切换 |
| 稳定 ID 编解码 | | ✓ | `component.idgen.codec.secret` |
| 异步任务队列 | | ✓ | `client.asyncq.*` |
| Atlas 自动迁移 | | ✓ | `app.dbMigration.*` |

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

::: tip
每次修改后至少运行对应行的最小验证命令。CI 环境会执行完整的 `go test -race ./...` + `pnpm run build` + `make e2e` 流水线。
:::

