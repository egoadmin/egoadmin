# API Routing and Forwarding

The gateway is the external HTTP entry point of EgoAdmin. Through protoc-gen-go-http generated handlers, the same Controller code handles both gRPC and HTTP requests.

## Overview

The gateway does not hand-write HTTP routes. All API endpoints are auto-generated from `google.api.http` annotations in proto files. HTTP requests are handled directly by protoc-gen-go-http generated handlers. The Controller layer implements the generated gRPC service interface and internally calls downstream user or idgen services via `internal/client`.

```text
Browser  ->  HTTP POST /api/*  ->  gateway HTTP (9001)
                                      |
                                      | protoc-gen-go-http HTTP handler
                                      v
                                  gateway gRPC (9002)
                                      |
                                      | internal/client
                                      v
                              user gRPC (9102) / idgen gRPC (9202)
```

## Core Usage

### Define HTTP Routes in Proto

Each RPC method binds an HTTP path via the `google.api.http` annotation. All management APIs use the `POST` method with `body: "*"` to map the entire request body to the gRPC message.

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

The URL pattern follows a fixed format: `POST /<package.version.Service>/<Method>`.

### Generate HTTP Route Code

Running `make gen` triggers protoc-gen-go-http to generate `RegisterXxxServiceHTTPServer` functions that register HTTP routes on the Gin engine.

```bash
make gen
```

Generated code is located in the `api/gen/go/` directory. Do not edit generated files manually.

### Register HTTP Routes

The gateway HTTP server is initialized in `NewHttpServer`, mounting all routes to Gin by calling the generated registration functions.

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

Each `RegisterXxxHTTPServer` function internally calls `gin.POST(path, handler)` to map paths declared in the `google.api.http` annotation to the corresponding handler.

### Controller Implements gRPC Interface

The Controller implements the generated gRPC server interface. Methods internally forward requests to downstream services via `internal/client`.

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

The gateway Controller is a thin proxy layer that contains no business logic. Business logic lives in the user service's Service and Application layers.

### HTTP to gRPC Full Chain

```text
1. Browser sends POST /api/user.v1.UserService/AddUser
2. Gin route matching enters the protoc-gen-go-http generated handler
3. Handler deserializes JSON body into AddUserRequest protobuf message
4. Calls UserGRPC.AddUser(ctx, req)
5. UserGRPC forwards gRPC to user service via userclient
6. User service processes the request, returns AddUserResponse
7. Handler serializes protobuf response to JSON and returns to browser
```

::: tip HTTP Path Prefix
The gateway HTTP server configures `ginRelativePath = "/api/*action"` and `stripPrefix = "/api"`. Client requests use the path `/api/user.v1.UserService/AddUser`, and the `/api` prefix is automatically stripped during route matching.
:::

### gRPC Service Registration

HTTP and gRPC share the same set of Controller instances. The gRPC server registers exactly the same services in `NewGrpcServer`.

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

The gRPC service port defaults to 9002, used for inter-service calls. External clients do not access the gRPC port directly.

### OpenAPI Documentation Endpoint

The gateway automatically exposes an `/openapi.yaml` endpoint providing API documentation for frontend and tooling.

```go
func registerOpenAPIDoc(r *egin.Component) {
    r.GET(openAPIYAMLPath, func(ctx *gin.Context) {
        ctx.Header("Cache-Control", "public, max-age=3600")
        ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", egoadmin.OpenAPIYAML)
    })
}
```

The OpenAPI spec is also generated from proto annotations, keeping it consistent with the backend implementation.

## Configuration Examples

### HTTP Server Configuration

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

| Config Key | Description |
|------------|-------------|
| `host` | HTTP listen address |
| `port` | HTTP port for external client access |
| `mode` | Gin mode, `release` disables debug logging |
| `ginRelativePath` | HTTP route matching path prefix pattern |
| `grpcEndpoint` | gRPC service endpoint |
| `stripPrefix` | URL prefix stripped during matching |
| `enableTraceInterceptor` | Enable trace interceptor |

### gRPC Server Configuration

```toml
[server.grpc]
host = "0.0.0.0"
port = 9002
```

### Inter-Service gRPC Client Configuration

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

The gateway connects to downstream services via etcd service discovery. The `addr` uses the `etcd:///` prefix, and the client automatically queries etcd for service instance lists.

## Real-World Examples

### Adding a New API Route

Using "Get User Detail" as an example, the complete route registration flow:

**Step 1: Define Proto**

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

**Step 2: Generate Code**

```bash
make gen
```

**Step 3: Gateway Controller Forwarding**

```go
// internal/app/gateway/controller/user_grpc.go
func (s *UserGRPC) GetUser(ctx context.Context, in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
    return s.client.GetUser(ctx, in)
}
```

**Step 4: Register Route (Auto-Generated)**

`RegisterUserServiceHTTPServer` already generates the new route registration code during `make gen`. No manual action needed.

**Step 5: Client Call**

```bash
# HTTP call
curl -X POST http://localhost:9001/api/user.v1.UserService/GetUser \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"id": 1}'
```

### Route Debugging

View registered route list:

```bash
# After starting gateway, view Gin route table
curl http://localhost:9001/debug/routes  # Requires debug mode
```

::: warning Debug Mode
Setting `mode` to `debug` shows detailed route info and request logs. Production must use `release` mode.
:::

## How It Works

### protoc-gen-go-http Mechanism

EgoAdmin uses a custom `protoc-gen-go-http` plugin to generate the HTTP compatibility layer, rather than gRPC-Gateway. The same Controller code handles both gRPC and HTTP requests directly -- there is no reverse proxy or protocol conversion layer. The workflow:

1. Parses `google.api.http` annotations from proto files
2. Generates corresponding `RegisterXxxServiceHTTPServer` functions
3. At runtime, the generated handler deserializes HTTP request JSON body into protobuf messages
4. Calls the Controller method (same as the gRPC service interface)
5. Serializes protobuf responses back to JSON

```text
HTTP POST /api/user.v1.UserService/AddUser
Content-Type: application/json

{"username": "admin", "nickname": "Admin"}
        |
        | protoc-gen-go-http handler
        v
UserGRPC.AddUser(AddUserRequest{
    Username: "admin",
    Nickname: "Admin",
})
```

### Middleware Chain

HTTP requests pass through the following middleware chain before reaching the route handler:

```text
Request -> Recovery -> AuthSession -> Perm -> Validator -> Ecode -> Handler
```

| Middleware | Responsibility |
|------------|----------------|
| `Recovery` | Catches panics, returns 500 errors |
| `AuthSession` | Extracts Bearer token, calls user service to validate JWT |
| `Perm` | Checks API permissions (Casbin RBAC) |
| `Validator` | Parameter validation (based on `validate` struct tags) |
| `Ecode` | Unified error code mapping and response formatting |

### Service Registration Entry

All components are wired via compile-time dependency injection:

```go
// internal/app/gateway/server/server.go
var ProviderSet = wire.NewSet(
    wire.Struct(new(controller.Options), "*"),
    NewGrpcServer,
    NewHttpServer,
    NewGovernServer,
)
```

Controller `Options` contains all dependencies:

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

## Common Issues

### 404 After Route Registration

**Symptom**: New proto methods return 404 on HTTP requests.

**Troubleshooting**:

1. Confirm `make gen` has been run to generate the latest code
2. Check that the `google.api.http` annotation path format is correct in proto
3. Confirm `RegisterXxxHTTPServer` is called in `registerHttp`
4. Check that the HTTP request path includes the `/api` prefix

### Request Body Parse Failure

**Symptom**: Returns `invalid argument` error.

**Troubleshooting**:

1. Confirm `Content-Type: application/json` request header
2. Check that JSON field names match proto definitions (camelCase)
3. Verify `validate` tag constraints are satisfied

### CORS Request Rejected

**Symptom**: Frontend browser reports CORS error when sending requests.

**Solution**: Confirm that the HTTP server configuration enables CORS support, or configure CORS headers at the reverse proxy layer (e.g., Nginx).

### Service Discovery Connection Failure

**Symptom**: After gateway startup, calls to downstream services report `connection refused` or `not found`.

**Troubleshooting**:

1. Confirm etcd cluster is running: `etcdctl get --prefix /egoadmin --keys-only`
2. Confirm downstream services have registered to etcd
3. Check that the `client.grpc.user.addr` config has the correct `etcd:///` prefix and name
4. Confirm network connectivity and firewall settings

## Reference Links

- [Google API HTTP Annotation Spec](https://cloud.google.com/endpoints/docs/grpc-service-config/reference/rpc/google.api#http)
- Relevant project source code:
  - `internal/app/gateway/server/http_server.go` -- HTTP server initialization
  - `internal/app/gateway/server/grpc_server.go` -- gRPC server initialization
  - `internal/app/gateway/controller/` -- Controller layer (gRPC/HTTP request handling)
  - `configs/gateway/config.toml` -- Gateway configuration file
