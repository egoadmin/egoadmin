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
│  HTTP 兼容层（grpc-gateway）                        │
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

## 请求链路

典型管理后台请求：

```text
浏览器
  -> POST /api/user.v1.UserService/GetUserList
  -> gateway HTTP server
  -> authsession 中间件
  -> Casbin API 权限校验
  -> grpc-gateway 转发
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

