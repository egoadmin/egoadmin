# 微服务架构

## 服务拓扑

EgoAdmin 由三个独立微服务组成。每个服务是独立进程，拥有自己的配置、数据库和 gRPC 契约。

```text
┌─────────────────────────────────────────────────┐
│                    浏览器                        │
└─────────────────────┬───────────────────────────┘
                      │ HTTP POST /api/*
                      ▼
┌─────────────────────────────────────────────────────┐
│                    gateway (9001)                   │
│                                                     │
│  HTTP 兼容层（protoc-gen-go-http）                  │
│  鉴权中间件（authsession Bearer）                   │
│  路由 + 请求聚合                                    │
│  go:embed 内嵌前端静态资源                          │
│  TUS 断点续传上传入口                               │
│  CDN 签名分发                                      │
│                                                     │
│  数据库: egoadmin_gateway                           │
└──────────┬──────────────────────────┬──────────────┘
           │ gRPC                     │ gRPC
           ▼                          ▼
┌──────────────────────┐  ┌──────────────────────────┐
│   user (9101)        │  │   idgen (9201)            │
│                      │  │                          │
│  用户 CRUD           │  │  雪花算法 ID 生成         │
│  角色/部门/组织管理  │  │  号段模式 ID 分配         │
│  JWT 认证与会话管理  │  │  机器租约管理             │
│  Casbin RBAC 权限    │  │  命名空间隔离             │
│  登录加密挑战        │  │  服务发现自注册           │
│  审计日志            │  │                          │
│  心跳离线检测        │  │                          │
│  数据权限 DataScope  │  │                          │
│                      │  │                          │
│  数据库: egoadmin_user│  │ 数据库: egoadmin_idgen    │
└──────────────────────┘  └──────────────────────────┘
```

## 服务职责

| 服务 | 核心职责 | 不能做什么 |
|------|----------|------------|
| `gateway` | 对外 HTTP 入口、前端内嵌、上传、API 聚合、服务间调用发起方 | 不直接访问 user/idgen 的数据库、domain、adapter |
| `user` | 用户、角色、部门、认证、权限、审计日志、数据权限 | 不承担外部前端静态资源分发 |
| `idgen` | ID 生成、号段、机器租约、命名空间 | 不承载业务权限和用户域状态 |

服务边界原则：

- 服务只能直接访问自己的数据库。
- 跨服务读写必须通过 `internal/client/*` gRPC wrapper。
- 业务 API 从 proto 定义，不添加独立 Gin 业务路由。
- 完整用户可见链路需要通过 gateway e2e 覆盖。

## 端口与健康检查

| 服务 | HTTP | gRPC | Governor | 健康检查 |
|------|------|------|----------|----------|
| gateway | 9001 | 9002 | 9003 | `/healthz`, `/readyz` |
| user | 9101 | 9102 | 9103 | `/healthz`, `/readyz` |
| idgen | 9201 | 9202 | 9203 | `/healthz`, `/readyz` |

健康检查 HTTP 服务器不需要鉴权。对于仅用于健康检查的 HTTP 服务，不注册业务中间件。

```bash
curl http://localhost:9001/readyz
curl http://localhost:9101/readyz
curl http://localhost:9201/readyz
```

## 服务发现

服务通过 etcd 注册和发现。

服务注册配置：

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"
```

gRPC 客户端通过 `etcd:///` 地址发现服务：

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

运行服务时，Makefile 会设置默认服务名：

```bash
make run SERVICE=user
# 内部等效于 EGO_NAME=egoadmin-user GO_SERVICE=user go run ./cmd/user
```

## 服务间 gRPC 客户端

跨服务调用封装在 `internal/client`，调用方不直接构造 generated client。

```text
gateway/application
  -> internal/client/userclient
  -> api/gen/go/user/v1.UserServiceClient
  -> etcd:///egoadmin-user
```

推荐接口形态：

```go
type UserClient interface {
  GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error)
}

type UserServiceClient struct {
  client userpb.UserServiceClient
}

func (c *UserServiceClient) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
  return c.client.GetUser(ctx, req)
}
```

::: tip
业务代码依赖窄接口，测试时可以替换为 fake client。不要在 application 里散落 `egrpc.Load(...)` 或 `NewXxxServiceClient(...)`。
:::

## 数据库边界

| 服务 | 数据库 | 迁移目录 |
|------|--------|----------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

规则：

- Atlas 不负责创建数据库，Compose 初始化脚本或 DBA 需要先创建 database。
- 不能将 user 的迁移目录应用到 gateway 数据库。
- 新表和字段只维护服务自己的 GORM model 和 `schema.MigrationModels()`。
- 迁移 SQL 和 `atlas.sum` 一起提交。

## 后端目录职责

```text
internal/app/user/
├── server/                    # 运行时装配、gRPC/HTTP/governor、迁移、健康检查
├── controller/                # gRPC server 实现，协议转换
├── application/               # 用例、事务、权限、跨服务、DTM 编排
├── domain/                    # 纯业务模型和 Repository 接口
├── adapter/                   # MySQL/缓存/外部组件适配
└── schema/                    # GORM 迁移模型清单
```

依赖方向规则：

```text
server -> controller -> application -> domain
                              |
                              v
                         adapter/persistence/mysql
```

- `server` 负责 Wire 装配和启动生命周期，不包含业务逻辑。
- `controller` 是 gRPC 生成代码的实现层，负责协议转换（proto -> domain），不直接操作数据库。
- `application` 编排用例、管理事务、调用跨服务客户端、协调权限校验。
- `domain` 是纯业务模型层，不导入 protobuf、GORM、EGO、Redis 或任何外部框架。
- `adapter` 实现 domain 定义的 Repository 接口，是唯一允许导入数据库驱动的层。

### Wire 依赖注入

EgoAdmin 使用 Google Wire 实现编译时依赖注入。每个层暴露一个 `ProviderSet`：

```go
// internal/app/user/application/provider.go
var ProviderSet = wire.NewSet(
    NewUserUseCase,
    NewRoleUseCase,
    NewDeptUseCase,
    wire.Struct(new(UserOptions), "*"),
    wire.Struct(new(RoleOptions), "*"),
    wire.Struct(new(DeptOptions), "*"),
)
```

应用层的 Options 结构体收集所有依赖：

```go
type RoleOptions struct {
    RoleRepository roledomain.Repository    // domain 接口，adapter 实现
    Mysql          mysql.MysqlInterface     // 事务管理
    RoleLocks      RoleLocks                // 并发锁
    Assignments    RoleAssignments          // 角色使用量查询
    Permissions    RolePermissionBinding    // Casbin 策略同步
}
```

最终在 `server/wire.go` 中组装：

```go
func NewApp() (*App, error) {
    panic(wire.Build(
        newEgo,
        newConfig,
        application.ProviderSet,       // 应用层
        mysql.CoreProviderSet,          // 数据库
        controller.ProviderSet,         // gRPC 控制器
        userclient.ProviderSet,         // 跨服务客户端
        discovery.ProviderSet,          // etcd 注册发现
        shutdown.ProviderSet,           // 优雅关闭
        wire.Bind(new(roledomain.Repository), new(*mysql.RoleRepository)),
        ProviderSet,
        newApp,
    ))
}
```

::: tip
`wire.Bind` 将 domain 层定义的 Repository 接口绑定到 adapter 层的具体实现。这是依赖倒置原则的编译时保证：domain 不知道 adapter 的存在，但 Wire 确保运行时注入正确的实现。
:::

### 各层代码示例

**domain 层 -- 纯业务模型和接口：**

```go
// internal/app/user/domain/role/repository.go
package role

type Repository interface {
    Create(ctx context.Context, aggregate *Role) error
    Update(ctx context.Context, id uint64, aggregate *Role) error
    Delete(ctx context.Context, id uint64) error
    FindByID(ctx context.Context, id uint64) (*Role, error)
    FindByName(ctx context.Context, name string) (*Role, error)
}
```

```go
// internal/app/user/domain/role/error.go
var (
    ErrNotFound   = errors.New("role: not found")
    ErrNameExists = errors.New("role: name exists")
    ErrInUse      = errors.New("role: in use")
)

type InUseError struct {
    Count int64
}

func (e InUseError) Error() string {
    return fmt.Sprintf("role: in use by %d users", e.Count)
}

func (e InUseError) Unwrap() error {
    return ErrInUse
}
```

**adapter 层 -- MySQL Repository 实现：**

```go
// internal/app/user/adapter/persistence/mysql/role_repository.go
type RoleRepository struct {
    db platformmysql.MysqlInterface
}

var _ roledomain.Repository = (*RoleRepository)(nil)  // 编译时接口检查

func (r *RoleRepository) Create(ctx context.Context, aggregate *roledomain.Role) error {
    model := roleModelFromDomain(aggregate)
    if err := r.db.WithTx(ctx).Create(model).Error; err != nil {
        return err
    }
    aggregate.ID = model.ID
    return nil
}
```

**application 层 -- 用例编排：**

```go
// internal/app/user/application/role_usecase.go
type RoleUseCase struct {
    role        roledomain.Repository
    mysql       mysql.MysqlInterface
    locks       RoleLocks
    assignments RoleAssignments
    permissions RolePermissionBinding
}

func (uc *RoleUseCase) SaveRole(ctx context.Context, cmd SaveRoleCommand) (uint64, error) {
    var roleID uint64
    err := uc.locks.WithRoleCreateLock(ctx, func(lockCtx context.Context) error {
        existing, _ := uc.role.FindByName(lockCtx, cmd.Name)
        if existing != nil {
            return roledomain.ErrNameExists
        }
        role := roledomain.NewRole(cmd.Name, cmd.Desc)
        if err := uc.role.Create(lockCtx, role); err != nil {
            return err
        }
        roleID = role.ID
        return uc.permissions.ReplaceRolePermissions(lockCtx, role.ID, cmd.Policies)
    })
    return roleID, err
}
```

**controller 层 -- 协议转换：**

```go
// internal/app/user/controller/role_grpc.go
func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error) {
    out := &userv1.AddRoleResponse{}

    defer func() {
        if err == nil {
            s.logger.Save(ctx, "系统管理-角色管理", "新增", "新增角色", in)
        }
    }()

    cmd := application.SaveRoleCommand{
        Name:       in.GetRole().GetName(),
        Desc:       in.GetRole().GetDesc(),
        Policies:   convertPolicies(in.GetRole().GetPolicies()),
    }
    id, err := s.roleUseCase.SaveRole(ctx, cmd)
    if err != nil {
        return out, mapRoleError(ctx, err)
    }
    out.Id = id
    return out, nil
}
```

## 请求链路

## 请求链路

典型管理后台请求：

```text
浏览器
  -> POST /api/user.v1.UserService/GetUserList
  -> gateway HTTP server
  -> authsession 中间件
  -> Casbin API 权限校验
  -> protoc-gen-go-http handler 直接处理
  -> user gRPC controller
  -> application usecase
  -> domain / repository
  -> MySQL / Redis
  -> response
```

## 启动流程

每个服务启动时执行：

```text
1. 加载配置：configs/<service>/config.toml + 环境变量覆盖
2. 初始化平台组件：MySQL、Redis、MinIO、etcd、trace
3. 执行 Atlas 迁移（如启用）
4. Wire 装配：server -> controller -> application -> adapter
5. 注册 gRPC 服务
6. 启动 HTTP 健康检查/业务入口
7. 注册到 etcd
8. readiness 转为 ready
```

### 配置加载机制

配置加载遵循以下优先级（后者覆盖前者）：

```text
内置默认配置
  -> configs/<service>/config.toml
  -> .env 文件
  -> EGOADMIN_* 环境变量
```

```bash
# 查看内置默认配置
go run ./cmd/user config print-default

# 输出完整 TOML，包含所有配置项和默认值
# 只需在 config.toml 中写与默认值不同的项
```

环境变量覆盖规则：TOML 路径转大写下划线，加 `EGOADMIN_` 前缀。

```text
TOML:                          ENV:
[server.http] port = 9001     EGOADMIN_SERVER_HTTP_PORT=9101
[client.mysql] dsn = "..."    EGOADMIN_CLIENT_MYSQL_DSN="..."
[etcd] addrs = ["..."]        EGOADMIN_ETCD_ADDRS="a:2379,b:2379"
[app.user] jwtExpire = 604800  EGOADMIN_APP_USER_JWTEXPIRE=86400
```

::: warning
环境变量覆盖在启动时一次性执行。Duration 类型使用 Go 格式（如 `"3s"`），数组类型使用逗号分隔。同名冲突（不同 TOML 路径映射到相同环境变量后缀）会在启动时报错。
:::

## 关闭流程

```text
SIGTERM / Ctrl+C
  -> readiness 返回 not ready
  -> 停止接收新请求
  -> gRPC graceful stop
  -> 关闭 cron / async workers
  -> 释放 idgen 机器租约
  -> 关闭 Redis / MinIO / DB
  -> 退出进程
```

非 server 资源通过 `internal/platform/shutdown.Manager` 管理关闭顺序。

## 错误处理流转

错误从 domain 层产生，逐层转换，最终返回给客户端：

```text
domain: ErrNameExists (纯 Go error)
  -> application: 包装业务上下文
  -> controller: mapRoleError() 转为 gRPC status
  -> gateway: normalizeEgoError() 保持 gRPC 错误码
  -> HTTP: 状态码 + JSON 错误体
```

**domain 层 -- 领域错误定义：**

```go
var (
    ErrNotFound   = errors.New("role: not found")
    ErrNameExists = errors.New("role: name exists")
    ErrInUse      = errors.New("role: in use")
)

type InUseError struct {
    Count int64
}

func (e InUseError) Unwrap() error { return ErrInUse }
```

**controller 层 -- 错误映射为 gRPC status：**

```go
func mapRoleError(ctx context.Context, err error) error {
    if errors.Is(err, roledomain.ErrNotFound) {
        return status.Error(codes.NotFound, i18n.T(ctx, "role.not.found"))
    }
    if errors.Is(err, roledomain.ErrNameExists) {
        return status.Error(codes.AlreadyExists, i18n.T(ctx, "role.name.exists"))
    }
    if errors.Is(err, roledomain.ErrInUse) {
        var inUseErr *roledomain.InUseError
        if errors.As(err, &inUseErr) {
            return status.Errorf(codes.FailedPrecondition,
                i18n.T(ctx, "role.in.use"), inUseErr.Count)
        }
    }
    return err
}
```

**跨服务客户端 -- 错误标准化：**

```go
// internal/client/userclient/client.go
func normalizeEgoError(err error) error {
    if err == nil {
        return nil
    }
    egoErr := eerrors.FromError(err)
    if egoErr.GetReason() == "" {
        return err
    }
    return egoErr
}
```

::: tip
domain 层使用 `errors.Is` / `errors.As` 标识性错误，controller 层负责国际化翻译和 gRPC 状态码映射。不要在 domain 层直接构造 gRPC status 错误。
:::

## 跨服务通信模式

gateway 调用 user/idgen 通过 `internal/client/*` 封装，业务代码只依赖接口。

### Metadata 自动传递

gateway 的 HTTP 请求头（认证、语言、链路追踪）会自动透传到下游 gRPC 服务：

```go
// internal/client/userclient/client.go
var forwardedMetadataKeys = map[string]struct{}{
    "authorization":     {},
    "accept-language":   {},
    "x-forwarded-for":   {},
    "x-request-id":      {},
    "x-b3-traceid":      {},
    "traceparent":       {},
}

func outgoingContext(ctx context.Context) context.Context {
    incoming, _ := metadata.FromIncomingContext(ctx)
    outgoing, _ := metadata.FromOutgoingContext(ctx)
    merged := outgoing.Copy()
    for key, values := range incoming {
        if _, ok := forwardedMetadataKeys[strings.ToLower(key)]; ok {
            merged[key] = values
        }
    }
    return metadata.NewOutgoingContext(ctx, merged)
}
```

这意味着：
- 用户 Bearer token 从 gateway HTTP 传递到 user gRPC。
- `Accept-Language` 使得 user 服务的错误消息自动国际化。
- OpenTelemetry trace ID 贯穿整个调用链。

### 客户端使用方式

应用层注入窄接口，不直接接触 gRPC 连接：

```go
// application 层只依赖 interface
type UserGetter interface {
    GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error)
}

// 客户端实现
type Client struct {
    conn *egrpc.Component
    User UserService
    Role RoleService
    Dept DeptService
    // ...
}

func NewClient(_ discovery.Ready) *Client {
    conn := egrpc.Load("client.grpc.user").Build()
    return &Client{
        conn: conn,
        User: &userClient{client: userv1.NewUserServiceClient(conn.ClientConn)},
        // ...
    }
}
```

::: danger
不要在 `application` 层散落 `egrpc.Load(...)` 或 `NewXxxServiceClient(...)`。所有跨服务调用必须通过 `internal/client/*` 封装，便于测试时替换为 fake client。
:::

### 调用链示例

```text
浏览器 POST /api/user.v1.RoleService/AddRole
  -> gateway HTTP server (gin, protoc-gen-go-http)
  -> authsession 中间件 (解析 Bearer token)
  -> Casbin 权限校验 (检查 USER.V1.ROLESERVICE/ADDROLE)
  -> gateway RoleGRPC.AddRole() controller
  -> gateway application 调用 userclient.User.GetUser() (如需校验)
  -> etcd:///egoadmin-user (解析后端实例)
  -> user gRPC server
  -> user controller -> user application -> user domain -> MySQL
  -> 返回响应
```

## 新服务检查清单

新增服务时需要完成：

1. `cmd/<service>/main.go` 和 `cmd/<service>/Dockerfile`
2. `configs/<service>/config.toml`
3. `internal/app/<service>/{server,controller,application,domain,adapter,schema}`
4. `api/proto` 或 `api/proto-internal` 契约
5. `atlas/migrations/<service>/atlas.sum`
6. `server/http_server.go` 暴露 `/healthz` 和 `/readyz`
7. `server/grpc_server.go` 注册 generated gRPC service
8. `internal/client/<service>client` 供其他服务调用
9. Docker Compose 中间件和应用配置
10. `make service.check SERVICE=<service>` 通过

