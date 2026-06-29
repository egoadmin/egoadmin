# Microservice Architecture

## Topology

EgoAdmin consists of three independent microservices. Each service is a standalone process with its own config, database and gRPC contracts.

```text
┌─────────────────────────────────────────────────┐
│                    Browser                       │
└─────────────────────┬───────────────────────────┘
                      │ HTTP POST /api/*
                      ▼
┌─────────────────────────────────────────────────────┐
│                    gateway (9001)                   │
│                                                     │
│  HTTP compatibility layer (protoc-gen-go-http)      │
│  Auth middleware (authsession Bearer)                │
│  Routing + request aggregation                      │
│  go:embed embedded frontend static assets            │
│  TUS resumable upload entry point                   │
│  CDN signed delivery                                │
│                                                     │
│  Database: egoadmin_gateway                          │
└──────────┬──────────────────────────┬──────────────┘
           │ gRPC                     │ gRPC
           ▼                          ▼
┌──────────────────────┐  ┌──────────────────────────┐
│   user (9101)        │  │   idgen (9201)            │
│                      │  │                          │
│  User CRUD           │  │  Snowflake ID generation  │
│  Role/Dept/Org Mgmt  │  │  Segment ID allocation    │
│  JWT Auth & Sessions │  │  Machine lease management │
│  Casbin RBAC         │  │  Namespace isolation      │
│  Login Crypto        │  │  Service discovery        │
│  Audit Logs          │  │                          │
│  Heartbeat Offline   │  │                          │
│  Data Permissions    │  │                          │
│                      │  │                          │
│  Database: egoadmin_user│ │ Database: egoadmin_idgen │
└──────────────────────┘  └──────────────────────────┘
```

## Service Responsibilities

| Service | Core Responsibilities | Cannot Do |
|---------|----------------------|-----------|
| `gateway` | External HTTP entry, embedded frontend, upload, API aggregation, cross-service calls | Cannot access user/idgen database, domain, or adapter |
| `user` | Users, roles, departments, auth, permissions, audit logs, data permissions | Does not serve frontend static assets |
| `idgen` | ID generation, segments, machine leases, namespaces | Does not handle business permissions |

Boundary rules:

- A service directly accesses only its own database.
- Cross-service reads/writes go through `internal/client/*` gRPC wrappers.
- Business APIs start from proto definitions, no standalone Gin business routes.
- Full user-visible flows require gateway e2e coverage.

## Ports and Health Checks

| Service | HTTP | gRPC | Governor | Health Endpoints |
|---------|------|------|----------|------------------|
| gateway | 9001 | 9002 | 9003 | `/healthz`, `/readyz` |
| user | 9101 | 9102 | 9103 | `/healthz`, `/readyz` |
| idgen | 9201 | 9202 | 9203 | `/healthz`, `/readyz` |

Health check endpoints do not require authentication. Business middleware is not registered on the health-only HTTP server.

```bash
curl http://localhost:9001/readyz
curl http://localhost:9101/readyz
curl http://localhost:9201/readyz
```

## Service Discovery

Services register to etcd and clients use `etcd:///` targets.

Registration config:

```toml
[etcd]
addrs = ["127.0.0.1:2379"]
connectTimeout = "1s"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"
```

gRPC client discovery:

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

When running services, the Makefile sets default service names:

```bash
make run SERVICE=user
# Internally equivalent to: EGO_NAME=egoadmin-user GO_SERVICE=user go run ./cmd/user
```

## Cross-Service gRPC Clients

Cross-service calls are wrapped in `internal/client/*`. Business code never constructs generated clients directly.

```text
gateway/application
  -> internal/client/userclient
  -> api/gen/go/user/v1.UserServiceClient
  -> etcd:///egoadmin-user
```

Recommended client interface:

```go
type UserClient interface {
    GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error)
}

type Client struct {
    conn *egrpc.Component
    User UserService
    Role RoleService
    Dept DeptService
}

func NewClient(_ discovery.Ready) *Client {
    conn := egrpc.Load("client.grpc.user").Build()
    return &Client{
        conn: conn,
        User: &userClient{client: userv1.NewUserServiceClient(conn.ClientConn)},
    }
}
```

::: tip
Business code depends on narrow interfaces, enabling fake client substitution in tests. Do not scatter `egrpc.Load(...)` or `NewXxxServiceClient(...)` in application code.
:::

### Metadata Forwarding

HTTP headers (auth, language, tracing) are automatically forwarded to downstream gRPC services:

```go
var forwardedMetadataKeys = map[string]struct{}{
    "authorization":   {},
    "accept-language": {},
    "x-request-id":    {},
    "traceparent":     {},
    "x-b3-traceid":    {},
    // ...
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

This means:
- User Bearer token flows from gateway HTTP to user gRPC.
- `Accept-Language` enables automatic i18n in the user service.
- OpenTelemetry trace IDs span the entire call chain.

::: danger
Do not scatter `egrpc.Load(...)` or `NewXxxServiceClient(...)` in application code. All cross-service calls must go through `internal/client/*` wrappers for testability.
:::

## Database Boundaries

| Service | Database | Migration Directory |
|---------|----------|---------------------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

Rules:

- A service directly accesses only its own database.
- Cross-service reads/writes go through `internal/client/*` gRPC wrappers.
- Do not apply one service's migration directory to another service's database.
- Atlas does not create databases; Compose init scripts or DBA must create them first.
- New tables and fields only modify the service's own GORM models and `schema.MigrationModels()`.
- Migration SQL and `atlas.sum` are committed together.

## Backend Layer Contract

```text
internal/app/<service>/
├── server/       # runtime assembly, gRPC/HTTP/governor, migrations, health checks
├── controller/   # gRPC server implementation, protocol conversion
├── application/  # use cases, transactions, permissions, cross-service calls, DTM
├── domain/       # aggregates, value objects, domain errors, repository interfaces
├── adapter/      # MySQL/cache/object-store adapters
└── schema/       # migration model lists
```

Dependency direction rules:

```text
server -> controller -> application -> domain
                              |
                              v
                         adapter/persistence/mysql
```

- `server` handles Wire assembly and startup lifecycle, no business logic.
- `controller` implements generated gRPC code, handles protocol conversion (proto -> domain).
- `application` orchestrates use cases, manages transactions, calls cross-service clients.
- `domain` is the pure business model layer, imports no frameworks.
- `adapter` implements Repository interfaces defined in `domain`.

### Wire Dependency Injection

EgoAdmin uses Google Wire for compile-time dependency injection. Each layer exposes a `ProviderSet`:

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

The application layer Options struct collects all dependencies:

```go
type RoleOptions struct {
    RoleRepository roledomain.Repository    // domain interface, adapter implements
    Mysql          mysql.MysqlInterface     // transaction management
    RoleLocks      RoleLocks                // concurrency locks
    Assignments    RoleAssignments          // role usage queries
    Permissions    RolePermissionBinding    // Casbin policy sync
}
```

Final assembly in `server/wire.go`:

```go
func NewApp() (*App, error) {
    panic(wire.Build(
        newEgo,
        newConfig,
        application.ProviderSet,       // application layer
        mysql.CoreProviderSet,          // database
        controller.ProviderSet,         // gRPC controllers
        userclient.ProviderSet,         // cross-service clients
        discovery.ProviderSet,          // etcd registration
        shutdown.ProviderSet,           // graceful shutdown
        wire.Bind(new(roledomain.Repository), new(*mysql.RoleRepository)),
        ProviderSet,
        newApp,
    ))
}
```

::: tip
`wire.Bind` binds a domain-layer Repository interface to an adapter-layer concrete implementation. This is compile-time enforcement of the dependency inversion principle: domain knows nothing about adapter, but Wire ensures the correct implementation is injected at runtime.
:::

## Runtime Flow

```text
1. Load configs and env overrides
2. Initialize platform clients and components
3. Run Atlas migrations when enabled
4. Wire server/controller/application/adapter
5. Register generated gRPC services
6. Start HTTP and gRPC servers
7. Register to etcd
8. Mark readiness as ready
```

### Config Loading Mechanism

Config loading follows this precedence (later overrides earlier):

```text
Built-in defaults
  -> configs/<service>/config.toml
  -> .env file
  -> EGOADMIN_* environment variables
```

```bash
# View built-in defaults
go run ./cmd/user config print-default

# Only write values that differ from defaults in config.toml
```

Environment variable override rule: convert TOML path to uppercase underscores with `EGOADMIN_` prefix.

```text
TOML:                          ENV:
[server.http] port = 9001     EGOADMIN_SERVER_HTTP_PORT=9101
[client.mysql] dsn = "..."    EGOADMIN_CLIENT_MYSQL_DSN="..."
[etcd] addrs = ["..."]        EGOADMIN_ETCD_ADDRS="a:2379,b:2379"
[app.user] jwtExpire = 604800  EGOADMIN_APP_USER_JWTEXPIRE=86400
```

::: warning
Environment variable overrides are executed once at startup. Duration types use Go format (e.g. `"3s"`), arrays use comma separation. Suffix conflicts (different TOML paths mapping to the same env var suffix) cause a startup error.
:::

## Request Flow

```text
Browser
  -> POST /api/user.v1.UserService/GetUserList
  -> gateway HTTP server
  -> authsession middleware
  -> Casbin permission check
  -> protoc-gen-go-http handler (direct, no proxy)
  -> user gRPC controller
  -> application use case
  -> domain / repository
  -> MySQL / Redis
```

## Error Handling Across Layers

Errors originate in the domain layer and transform as they propagate:

```text
domain: ErrNameExists (plain Go error)
  -> application: wraps business context
  -> controller: mapRoleError() converts to gRPC status
  -> gateway: normalizeEgoError() preserves gRPC error codes
  -> HTTP: status code + JSON error body
```

**Domain layer** -- domain error definitions:

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

**Controller layer** -- error mapping to gRPC status:

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

**Cross-service client** -- error normalization:

```go
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
Domain layer uses `errors.Is` / `errors.As` sentinel errors. Controller layer is responsible for i18n translation and gRPC status code mapping. Do not construct gRPC status errors directly in the domain layer.
:::

## Shutdown Flow

```text
SIGTERM / Ctrl+C
  -> readiness returns not ready
  -> stop accepting new requests
  -> gRPC graceful stop
  -> close cron / async workers
  -> release idgen machine lease
  -> close Redis / MinIO / DB
  -> exit process
```

Non-server resources are managed by `internal/platform/shutdown.Manager` for ordered shutdown.

## New Service Checklist

When adding a new service:

1. `cmd/<service>/main.go` and `cmd/<service>/Dockerfile`
2. `configs/<service>/config.toml`
3. `internal/app/<service>/{server,controller,application,domain,adapter,schema}`
4. `api/proto` or `api/proto-internal` contracts
5. `atlas/migrations/<service>/atlas.sum`
6. `server/http_server.go` exposes `/healthz` and `/readyz`
7. `server/grpc_server.go` registers generated gRPC service
8. `internal/client/<service>client` for other services to call
9. Docker Compose middleware and application configuration
10. `make service.check SERVICE=<service>` passes
