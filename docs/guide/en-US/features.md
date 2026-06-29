# Feature Overview

This page is the capability map for EgoAdmin. Use it to locate the right modules, commands and deeper guides before changing code.

## Capability Map

| Area | Main Paths | Command | Guide |
|------|------------|---------|-------|
| Runtime | `cmd/*`, `internal/app/*/server`, `configs/*` | `make run SERVICE=user` | [Architecture](/en-US/guide/architecture) |
| API development | `api/proto`, `controller`, `application`, `adapter` | `make gen` | [API Workflow](/en-US/guide/api-development) |
| Permission | `authsession`, `routeMenu.ts`, `api-manifest.ts` | `cd web && pnpm run build` | [Permission System](/en-US/guide/permission-system) |
| Database | `schema`, `atlas/migrations/*` | `make migrate.new SERVICE=user NAME=xxx` | [Database & Migrations](/en-US/guide/database-migration) |
| Frontend | `web/src/views`, `web/src/api`, `web/src/router` | `pnpm run type-check` | [Frontend](/en-US/guide/frontend-development) |
| DTM | `application`, branch proto APIs | `make e2e E2E_TIMEOUT=20m` | [DTM](/en-US/guide/distributed-transactions) |
| Components | `internal/component`, `internal/platform` | `go test ./internal/component/...` | [Components](/en-US/guide/components) |
| Configuration | `configs/*/config.toml` | `make service.config SERVICE=user` | [Configuration](/en-US/guide/configuration) |
| Testing & deployment | `test/e2e`, `deploy`, `scripts/make` | `make e2e` | [Testing & Deployment](/en-US/guide/testing-deployment) |

## Services

| Service | Responsibility |
|---------|----------------|
| `gateway` | external HTTP compatibility entry, embedded frontend, upload, API aggregation, calls user/idgen |
| `user` | users, roles, departments, auth/session, permissions, audit logs, data scope |
| `idgen` | Snowflake IDs, segments, machine leases, namespaces, stable ID codec |

## Backend Layers

```text
internal/app/<service>/
├── server/       # runtime assembly, gRPC/HTTP/governor, migrations, health
├── controller/   # generated gRPC server implementation and protocol conversion
├── application/  # use cases, transactions, permissions, cross-service calls, DTM
├── domain/       # aggregates, value objects, domain errors, repository interfaces
├── adapter/      # MySQL/cache/object-store adapters
└── schema/       # migration model lists
```

Dependency direction:

```text
server -> controller -> application -> domain
                              |
                              v
                         adapter/persistence/mysql
```

::: tip
The `domain` layer stays pure: no protobuf, GORM, EGO/Gin, Redis, MinIO, Casbin adapters or other service app packages.
:::

### Code Examples by Layer

**Domain layer** -- pure business model and repository interface:

```go
// domain/role/repository.go
type Repository interface {
    Create(ctx context.Context, aggregate *Role) error
    Update(ctx context.Context, id uint64, aggregate *Role) error
    Delete(ctx context.Context, id uint64) error
    FindByID(ctx context.Context, id uint64) (*Role, error)
    FindByName(ctx context.Context, name string) (*Role, error)
}

// domain/role/error.go
var (
    ErrNotFound   = errors.New("role: not found")
    ErrNameExists = errors.New("role: name exists")
    ErrInUse      = errors.New("role: in use")
)
```

**Adapter layer** -- MySQL repository implementation:

```go
type RoleRepository struct {
    db platformmysql.MysqlInterface
}

var _ roledomain.Repository = (*RoleRepository)(nil) // compile-time interface check

func (r *RoleRepository) Create(ctx context.Context, aggregate *roledomain.Role) error {
    model := roleModelFromDomain(aggregate)
    if err := r.db.WithTx(ctx).Create(model).Error; err != nil {
        return err
    }
    aggregate.ID = model.ID
    return nil
}
```

**Application layer** -- use case orchestration:

```go
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

**Controller layer** -- protocol conversion:

```go
func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error) {
    out := &userv1.AddRoleResponse{}
    cmd := application.SaveRoleCommand{
        Name:     in.GetRole().GetName(),
        Desc:     in.GetRole().GetDesc(),
        Policies: convertPolicies(in.GetRole().GetPolicies()),
    }
    id, err := s.roleUseCase.SaveRole(ctx, cmd)
    if err != nil {
        return out, mapRoleError(ctx, err)
    }
    out.Id = id
    return out, nil
}
```

## Proto-First APIs

Business APIs start from `.proto` files:

```protobuf
rpc GetRoleList(GetRoleListRequest) returns (GetRoleListResponse) {
  option (google.api.http) = {
    post: "/user.v1.RoleService/GetRoleList"
    body: "*"
  };
}
```

Frontend calls use the same gRPC compatibility path:

```ts
api.post('/user.v1.RoleService/GetRoleList', {
  page: 1,
  limit: 20,
})
```

Key constraints:

- HTTP compatibility endpoints always use `POST`.
- Path format: `/<package>.<Service>/<Method>`.
- Request body uses protobuf JSON field names, e.g. `deptId`, `roleIds`.
- OpenAPI docs, validation tags and permission descriptions are defined in proto.

::: warning
Do not add standalone Gin business routes. All business APIs must originate from `.proto` definitions with `google.api.http` annotations.
:::

## Database and Migrations

Migrations are managed by Atlas, one migration directory per service:

```text
atlas/migrations/gateway/
atlas/migrations/user/
atlas/migrations/idgen/
```

Standard migration workflow:

```bash
# 1. Modify service GORM model
# vim internal/app/user/adapter/persistence/mysql/user_model.go

# 2. Update schema list
# vim internal/app/user/schema/migration_models.go

# 3. Generate migration SQL
make migrate.new SERVICE=user NAME=add_phone_field

# 4. Validate migration
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user

# 5. Migration runs automatically at service startup
make run SERVICE=user
```

Runtime migration configuration:

```toml
[app.dbMigration]
enabled = true                    # enable auto migration
driver = "atlas"                  # migration tool
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"                     # atlas executable path
```

::: warning
In production, set `app.dbMigration.enabled` to `false` and run `atlas migrate apply` from CI/CD to prevent multiple instances from running migrations simultaneously.
:::

## Frontend Engineering

The frontend is in `web/`, co-located with the backend:

```text
web/src/
├── api/          # API calls and api-manifest
├── config/       # routeMenu permission menus
├── router/       # routes and guards
├── store/        # Pinia state
├── views/        # pages
├── components/   # shared components
├── i18n/         # internationalization
└── styles/       # styles and themes
```

Adding a new page typically requires modifying:

| Layer | File |
|-------|------|
| API module | `web/src/api/modules/<domain>.ts` |
| Page | `web/src/views/<domain>/<page>/index.vue` |
| Route | `web/src/router/modules/<domain>.ts` |
| Permission menu | `web/src/config/routeMenu.ts` |
| i18n | `web/src/i18n/*` |
| Permission contract | `web/dist/permission-contract.json` (build-generated) |

Frontend ID handling:

```ts
// Backend uint64 / fixed64 IDs must be treated as strings
const userId = ref<string>('')
const roleIds = ref<string[]>([])
```

::: danger
protobuf `uint64` IDs lose precision when parsed by JavaScript's `JSON.parse`. Always handle them as strings in TypeScript code.
:::

## Permission Chain

```text
authsession Bearer token
  -> API classification (public / login-only / protected)
  -> Casbin gRPC service/method permission
  -> DataScope query filtering
  -> API manifest
  -> routeMenu.ts
  -> permission-contract.json
```

Permission ID format:

```text
USER.V1.ROLESERVICE/ADDROLE
```

### Authentication Flow Code Example

The gateway `authsession` middleware extracts the user context from the Bearer token:

```go
// In application layer, get the current user
func (uc *UserUseCase) GetUserList(ctx context.Context, req *GetUserListQuery) (*UserListResult, error) {
    auth := authsession.FromContext(ctx)
    if auth == nil {
        return nil, ErrAuthRequired
    }

    // DataScope query filtering
    deptIDs, err := uc.dataScope.AllowedDeptIDs(ctx, auth.DeptID, auth.UserID)
    if err != nil {
        return nil, err
    }

    return uc.repo.FindByDeptIDs(ctx, deptIDs, req.Page, req.Limit)
}
```

### RBAC Permission Configuration

Casbin policies control API access via proto method identifiers:

```text
# Policy format: p, role_id, service, method
p, 1, USER.V1.ROLESERVICE, ADDROLE
p, 1, USER.V1.ROLESERVICE, DELETEROLE
p, 1, USER.V1.USERSERVICE, GETUSERLIST
p, 2, USER.V1.USERSERVICE, GETUSERLIST
```

::: tip
Role permission policies are defined in `web/src/config/routeMenu.ts` and auto-generated to `permission-contract.json` during build. The backend validates that granted API IDs fall within the contract boundaries.
:::

## Authentication and Login Security

EgoAdmin includes a built-in login crypto challenge mechanism to prevent plaintext password transmission:

```text
Frontend calls GetLoginCrypto
  -> Backend generates RSA public key + challenge (3-minute TTL)
  -> Frontend encrypts password with RSA public key
  -> Frontend derives AES key from challenge to encrypt password
  -> Login validates challenge and password
```

Configuration (`configs/user/config.toml`):

```toml
[component.logincrypto]
challengeTTL = "3m0s"        # challenge validity period
timestampSkew = "2m0s"       # allowed client time drift
rsaKeyBits = 4096            # RSA key size
enableMetrics = true         # report metrics

[app.user]
useCaptcha = false           # login captcha toggle
jwtExpire = 604800           # JWT validity (seconds)
refreshTokenExpire = 2592000 # refresh token validity (seconds)
multiLoginEnabled = true     # allow multi-device login
maxLoginClient = 2           # max concurrent login clients
```

## Internationalization (i18n)

Both backend and frontend support Chinese/English switching:

```text
Backend: internal/platform/i18n/locales/    # message translation files
Frontend: web/src/i18n/                      # Vue I18n translation files
```

Backend error message i18n:

```go
func mapRoleError(ctx context.Context, err error) error {
    if errors.Is(err, roledomain.ErrNameExists) {
        return platformi18n.Errorf(ctx, "role.name.exists")
    }
    return err
}
```

Frontend i18n usage:

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
The backend automatically selects language via the `Accept-Language` header. gRPC metadata is forwarded across service calls (see `userclient.outgoingContext`).
:::

## Feature Toggles

Some features are controlled by configuration without code changes:

| Config Key | Type | Default | Description |
|------------|------|---------|-------------|
| `app.user.useCaptcha` | bool | `false` | Login captcha toggle |
| `app.user.multiLoginEnabled` | bool | `true` | Multi-device login toggle |
| `app.user.heartbeatOfflineEnabled` | bool | `true` | Heartbeat offline detection |
| `app.service.autoMigrate` | bool | `false` | GORM AutoMigrate at startup (dev only) |
| `app.dbMigration.enabled` | bool | `true` | Atlas migration toggle |
| `app.service.skipPermissionContractCheck` | bool | `false` | Skip permission contract check |
| `server.http.enableCors` | bool | `false` | CORS toggle |
| `server.http.enableMetricInterceptor` | bool | `false` | HTTP request metrics |
| `server.http.enableTraceInterceptor` | bool | `false` | HTTP tracing |
| `component.logincrypto.enableMetrics` | bool | `true` | Login crypto metrics |

Override via environment variables:

```bash
# Disable captcha (e.g. frontend development)
export EGOADMIN_APP_USER_USECAPTCHA=false

# Disable Atlas migration (e.g. external migration tool)
export EGOADMIN_APP_DBMIGRATION_ENABLED=false

# Enable HTTP request metrics
export EGOADMIN_SERVER_HTTP_ENABLEMETRICINTERCEPTOR=true
```

## Feature Comparison: Built-in vs Requires Configuration

| Feature | Built-in | Requires Config | Config Location |
|---------|----------|-----------------|-----------------|
| JWT authentication | Yes | | Works out of the box |
| RBAC permissions | Yes | | Casbin policies auto-synced |
| Login crypto challenge | Yes | | RSA + AES dual encryption |
| Multi-device login | | Yes | `app.user.multiLoginEnabled` |
| Login captcha | | Yes | `app.user.useCaptcha` |
| Heartbeat offline detection | | Yes | `app.user.heartbeatOfflineEnabled` |
| Data permissions DataScope | Yes | | Per-role configuration |
| File upload (TUS) | Yes | | MinIO storage backend |
| CDN signed delivery | | Yes | `component.cdn.signSecret` |
| DTM distributed transactions | | Yes | Requires DTM service deployment |
| Distributed tracing | | Yes | `trace.*` config block |
| i18n | Yes | | Chinese/English built-in |
| Audit logs | Yes | | Auto-records key operations |
| Snowflake ID generation | Yes | | idgen service auto-provides |
| Segment ID allocation | Yes | | idgen auto-switches |
| Stable ID codec | | Yes | `component.idgen.codec.secret` |
| Async task queue | | Yes | `client.asyncq.*` |
| Atlas auto migration | | Yes | `app.dbMigration.*` |

## Validation Matrix

| Change | Minimum Validation |
|--------|--------------------|
| proto / provider | `make gen` |
| service layout | `make service.check SERVICE=user` |
| backend logic | `go test -race ./...` |
| frontend page | `cd web && pnpm run type-check` |
| permission / routeMenu | `cd web && pnpm run build` |
| migrations | `make migrate.validate SERVICE=user` |
| user-facing gateway flow | `make e2e E2E_TIMEOUT=20m` |

::: tip
After each change, run at least the minimum validation command for that change type. CI runs the full `go test -race ./...` + `pnpm run build` + `make e2e` pipeline.
:::
