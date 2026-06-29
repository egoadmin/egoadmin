# API 路由与转发

Gateway 是 EgoAdmin 的外部 HTTP 入口，通过 protoc-gen-go-http 生成的 HTTP 兼容层，同一套 Controller 代码同时处理 gRPC 和 HTTP 请求。

## 概述

Gateway 服务不手写 HTTP 路由。所有 API 端点均从 proto 文件中的 `google.api.http` 注解自动生成，HTTP 请求由 protoc-gen-go-http 生成的 handler 直接处理。Controller 层实现生成的 gRPC 服务接口，内部通过 `internal/client` 调用下游 user 或 idgen 服务。

```text
浏览器  ->  HTTP POST /api/*  ->  gateway HTTP (9001)
                                      |
                                      | protoc-gen-go-http HTTP handler
                                      v
                                  gateway gRPC (9002)
                                      |
                                      | internal/client
                                      v
                              user gRPC (9102) / idgen gRPC (9202)
```

## 核心用法

### Proto 中定义 HTTP 路由

每个 RPC 方法通过 `google.api.http` 注解绑定 HTTP 路径。所有管理类 API 使用 `POST` 方法、`body: "*"` 将整个请求体映射为 gRPC 消息。

```protobuf
service UserService {
  rpc AddUser(AddUserRequest) returns (AddUserResponse) {
    option (google.api.http) = {
      post: "/user.v1.UserService/AddUser"
      body: "*"
    };
  }

  rpc GetUserList(GetUserListRequest) returns (GetUserListResponse) {
    option (google.api.http) = {
      post: "/user.v1.UserService/GetUserList"
      body: "*"
    };
  }
}
```

URL 模式遵循固定格式：`POST /<package.version.Service>/<Method>`。

### 生成 HTTP 路由代码

运行 `make gen` 后，protoc-gen-go-http 生成 `RegisterXxxServiceHTTPServer` 函数，这些函数将 HTTP 路由注册到 Gin 引擎。

```bash
make gen
```

生成代码位于 `api/gen/go/` 目录，禁止手动编辑。

### 注册 HTTP 路由

Gateway 的 HTTP 服务器在 `NewHttpServer` 中初始化，通过调用生成的注册函数将所有路由挂载到 Gin。

```go
// internal/app/gateway/server/http_server.go
func registerHttp(r *egin.Component, opts controller.Options) {
    userv1.RegisterLogServiceHTTPServer(r, opts.LogGRPC)
    userv1.RegisterUserServiceHTTPServer(r, opts.UserGRPC)
    userv1.RegisterRoleServiceHTTPServer(r, opts.RoleGRPC)
    userv1.RegisterDeptServiceHTTPServer(r, opts.DeptGRPC)
    userv1.RegisterCenterServiceHTTPServer(r, opts.CenterGRPC)
}
```

每个 `RegisterXxxHTTPServer` 函数内部调用 `gin.POST(path, handler)`，将 `google.api.http` 注解中声明的路径映射到对应的 gRPC handler。

### Controller 实现 gRPC 接口

Controller 实现生成的 gRPC server 接口，方法内部通过 `internal/client` 将请求转发到下游服务。

```go
// internal/app/gateway/controller/user_grpc.go
type UserGRPC struct {
    client userclient.UserService
    api    *application.APIUseCase
}

func NewUserGRPCController(client *userclient.Client, api *application.APIUseCase) *UserGRPC {
    return &UserGRPC{client: client.User, api: api}
}

func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (*userv1.AddUserResponse, error) {
    return s.client.AddUser(ctx, in)
}
```

Gateway 的 Controller 是一个薄代理层，不包含业务逻辑。业务逻辑在 user 服务的 Service 和 Application 层实现。

### HTTP 到 gRPC 的完整链路

```text
1. 浏览器发送 POST /api/user.v1.UserService/AddUser
2. Gin 路由匹配，进入 protoc-gen-go-http 生成的 handler
3. handler 将 JSON 请求体反序列化为 AddUserRequest protobuf 消息
4. 调用 UserGRPC.AddUser(ctx, req)
5. UserGRPC 通过 userclient 转发 gRPC 到 user 服务
6. user 服务处理请求，返回 AddUserResponse
7. handler 将 protobuf 响应序列化为 JSON 返回给浏览器
```

::: tip HTTP 路径前缀
Gateway 的 HTTP 服务配置了 `ginRelativePath = "/api/*action"` 和 `stripPrefix = "/api"`。客户端请求路径为 `/api/user.v1.UserService/AddUser`，路由匹配时会自动去掉 `/api` 前缀。
:::

### gRPC 服务注册

HTTP 和 gRPC 共享同一套 Controller 实例。gRPC 服务器在 `NewGrpcServer` 中注册完全相同的服务。

```go
// internal/app/gateway/server/grpc_server.go
func NewGrpcServer(opts controller.Options, _ config.EgoReady) *GrpcServer {
    s := egrpc.Load("server.grpc").Build(
        egrpc.WithUnaryInterceptor(
            grpc.Middleware(
                recovery.Recovery(),
                remoteAuthServer(opts),
                perm.Server(permCheckFunc(opts)),
                validate.Validator(validate.WithV10(opts.Validator)),
                ecode.Ecode(),
            ),
        ),
    )

    userv1.RegisterLogServiceServer(s.Server, opts.LogGRPC)
    userv1.RegisterUserServiceServer(s.Server, opts.UserGRPC)
    userv1.RegisterRoleServiceServer(s.Server, opts.RoleGRPC)
    userv1.RegisterDeptServiceServer(s.Server, opts.DeptGRPC)
    userv1.RegisterCenterServiceServer(s.Server, opts.CenterGRPC)

    return &GrpcServer{Component: s}
}
```

gRPC 服务端口默认 9002，供服务间调用使用。外部客户端不直接访问 gRPC 端口。

### OpenAPI 文档端点

Gateway 自动暴露 `/openapi.yaml` 端点，提供 API 文档供前端和工具使用。

```go
func registerOpenAPIDoc(r *egin.Component) {
    r.GET(openAPIYAMLPath, func(ctx *gin.Context) {
        ctx.Header("Cache-Control", "public, max-age=3600")
        ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", egoadmin.OpenAPIYAML)
    })
}
```

OpenAPI 规范同样由 proto 注解生成，与后端实现保持一致。

## 配置示例

### HTTP 服务器配置

```toml
# configs/gateway/config.toml

[server.http]
host = "0.0.0.0"
port = 9001
mode = "release"
enableAccessInterceptorReq = false
enableAccessInterceptorRes = false
enableMetricInterceptor = false
enableTraceInterceptor = true
enableURLPathTrans = true
ginRelativePath = "/api/*action"
grpcEndpoint = "127.0.0.1:9002"
stripPrefix = "/api"
```

| 配置项 | 说明 |
|--------|------|
| `host` | HTTP 监听地址 |
| `port` | HTTP 端口，外部客户端访问 |
| `mode` | Gin 模式，`release` 关闭调试日志 |
| `ginRelativePath` | HTTP 路由匹配的路径前缀模式 |
| `grpcEndpoint` | gRPC 服务端点 |
| `stripPrefix` | 匹配时去掉的 URL 前缀 |
| `enableTraceInterceptor` | 启用链路追踪拦截器 |

### gRPC 服务器配置

```toml
[server.grpc]
host = "0.0.0.0"
port = 9002
```

### 服务间 gRPC 客户端配置

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

Gateway 通过 etcd 服务发现连接下游服务。`addr` 使用 `etcd:///` 前缀，客户端自动从 etcd 查询服务实例列表。

## 实战示例

### 添加新的 API 路由

以「获取用户详情」为例，完整的路由注册流程：

**第一步：定义 Proto**

```protobuf
// api/proto/user/v1/user.proto
service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse) {
    option (google.api.http) = {
      post: "/user.v1.UserService/GetUser"
      body: "*"
    };
  }
}

message GetUserRequest {
  uint64 id = 1 [
    (tagger.tags) = "validate:\"required\" label:\"User ID\"",
    (google.api.field_behavior) = REQUIRED
  ];
}

message GetUserResponse {
  uint64 id = 1;
  string username = 2;
  string nickname = 3;
}
```

**第二步：生成代码**

```bash
make gen
```

**第三步：Gateway Controller 转发**

```go
// internal/app/gateway/controller/user_grpc.go
func (s *UserGRPC) GetUser(ctx context.Context, in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
    return s.client.GetUser(ctx, in)
}
```

**第四步：注册路由（自动生成）**

`RegisterUserServiceHTTPServer` 在 `make gen` 时已自动生成新的路由注册代码，无需手动操作。

**第五步：客户端调用**

```bash
# HTTP 调用
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUser \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"id": 1}'
```

### 路由调试

查看已注册的路由列表：

```bash
# 启动 gateway 后，查看 Gin 路由表
curl http://localhost:9001/debug/routes  # 需要在 debug 模式下
```

::: warning 调试模式
将 `mode` 设为 `debug` 可以看到详细的路由信息和请求日志。生产环境必须使用 `release` 模式。
:::

## 工作原理

### protoc-gen-go-http 工作机制

EgoAdmin 使用自定义的 `protoc-gen-go-http` 插件生成 HTTP 兼容层，而非 gRPC-Gateway。同一套 Controller 代码同时处理 gRPC 和 HTTP 请求，不存在反向代理或协议转换层。它的工作流程：

1. 解析 proto 中的 `google.api.http` 注解
2. 生成对应的 HTTP handler 函数（`RegisterXxxServiceHTTPServer`）
3. 在运行时将 HTTP 请求的 JSON body 反序列化为 protobuf 消息
4. 直接调用 Controller 方法并将 protobuf 响应序列化为 JSON

```text
HTTP POST /api/user.v1.UserService/AddUser
Content-Type: application/json

{"username": "admin", "nickname": "管理员"}
        |
        | protoc-gen-go-http handler（直接调用，非代理）
        v
Controller.AddUser(ctx, AddUserRequest{
    Username: "admin",
    Nickname: "管理员",
})
```

### 中间件链

HTTP 请求经过以下中间件处理后才到达路由 handler：

```text
请求 -> Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler
```

| 中间件 | 职责 |
|--------|------|
| `Recovery` | 捕获 panic，返回 500 错误 |
| `AuthSession` | 提取 Bearer token，调用 user 服务验证 JWT |
| `Perm` | 检查 API 权限（Casbin RBAC） |
| `Validator` | 参数校验（基于 `validate` struct tag） |
| `Ecode` | 统一错误码映射和响应格式化 |

### 服务注册入口

所有组件通过 Wire 编译时注入连接：

```go
// internal/app/gateway/server/server.go
var ProviderSet = wire.NewSet(
    wire.Struct(new(controller.Options), "*"),
    NewGrpcServer,
    NewHttpServer,
    NewGovernServer,
)
```

Controller 的 `Options` 包含所有依赖：

```go
type Options struct {
    Conf       *config.Config
    Validator  *validate.Validate
    UserClient *userclient.Client
    LogGRPC    *LogGRPC
    UserGRPC   *UserGRPC
    RoleGRPC   *RoleGRPC
    DeptGRPC   *DeptGRPC
    CenterGRPC *CenterGRPC
}
```

## 常见问题

### 路由注册后 404

**现象**：新增 proto 方法后，HTTP 请求返回 404。

**排查步骤**：

1. 确认已运行 `make gen` 生成最新代码
2. 检查 proto 中 `google.api.http` 注解的路径格式是否正确
3. 确认 `RegisterXxxHTTPServer` 在 `registerHttp` 中已调用
4. 检查 HTTP 请求路径是否包含 `/api` 前缀

### 请求体解析失败

**现象**：返回 `invalid argument` 错误。

**排查步骤**：

1. 确认 `Content-Type: application/json` 请求头
2. 检查 JSON 字段名是否与 proto 定义一致（camelCase）
3. 验证 `validate` 标签约束是否满足

### 跨域请求被拒绝

**现象**：前端浏览器发送请求报 CORS 错误。

**解决方案**：确认 HTTP 服务器配置中已启用跨域支持，或在接入层（如 Nginx）配置 CORS 头。

### 服务发现连接失败

**现象**：Gateway 启动后调用下游服务报 `connection refused` 或 `not found`。

**排查步骤**：

1. 确认 etcd 集群正在运行：`etcdctl get --prefix /egoadmin --keys-only`
2. 确认下游服务已注册到 etcd
3. 检查 `client.grpc.user.addr` 配置中 `etcd:///` 前缀和名称是否正确
4. 确认网络连通性和防火墙设置

## 参考链接

- [Google API HTTP 注解规范](https://cloud.google.com/endpoints/docs/grpc-service-config/reference/rpc/google.api#http)
- 项目内相关源码：
  - `internal/app/gateway/server/http_server.go` — HTTP 服务器初始化
  - `internal/app/gateway/server/grpc_server.go` — gRPC 服务器初始化
  - `internal/app/gateway/controller/` — Controller 层（gRPC/HTTP 请求处理）
  - `configs/gateway/config.toml` — Gateway 配置文件
