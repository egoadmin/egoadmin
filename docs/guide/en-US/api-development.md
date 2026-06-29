# API Development Workflow

A complete management API change usually touches proto, generated code, domain, persistence, application, controller, service registration, permissions, frontend and tests.

The full pipeline:

```text
proto
  -> make gen
  -> domain
  -> adapter/persistence/mysql
  -> schema migration
  -> application
  -> controller
  -> server registration
  -> permission catalog
  -> frontend api/router/routeMenu/page
  -> tests
```

## 1. Define Proto

Business APIs start from `api/proto`. Here is a complete proto definition:

```protobuf
syntax = "proto3";
package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "tagger/tagger.proto";
import "protoc-gen-openapiv2/options/annotations.proto";

service RoleService {
  // AddRole creates a new role
  rpc AddRole(AddRoleRequest) returns (AddRoleResponse) {
    option (google.api.http) = {
      post: "/user.v1.RoleService/AddRole"
      body: "*"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      tags: "Role Service, Service: user.v1.RoleService"
      tags: "RoleService"
      description: "Auth: Requires Authorization: Bearer <token> and permission USER.V1.ROLESERVICE/ADDROLE."
      security: {
        security_requirement: {
          key: "BearerAuth"
          value: {}
        }
      }
    };
  }
}

message AddRoleRequest {
  string name = 1 [
    (tagger.tags) = "validate:\"required\" label:\"Role Name\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "Role Name"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  string code = 2 [
    (tagger.tags) = "validate:\"required\" label:\"Role Code\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "Role Code"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  int32 status = 3 [
    (tagger.tags) = "validate:\"required\" label:\"Status\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "Status"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  string remark = 4 [
    (tagger.tags) = "label:\"Remark\""
  ];
}

message AddRoleResponse {
  uint64 id = 1;
}
```

Rules:

- Use a separate request and response message for every RPC.
- Use HTTP `post` with `body: "*"`.
- Required input fields need both `validate:"required"` tags and `field_behavior = REQUIRED`.
- OpenAPI auth docs must match backend API classification.

## 2. Generate Code

```bash
make gen
```

Generated artifacts:

| Artifact | Purpose |
|----------|---------|
| `*.pb.go` | Protobuf message structs |
| `*_grpc.pb.go` | gRPC server/client interfaces |
| `*.pb.gw.go` | protoc-gen-go-http HTTP compatibility |
| `wire_gen.go` | Wire dependency injection |
| OpenAPI output | API docs and manifest input |

::: warning
Do not hand-edit generated files. After modifying source proto or Wire providers, re-run `make gen`.
:::

## 3. Domain

```go
// internal/app/user/domain/role/role.go
package role

import "time"

const (
  StatusEnabled  int32 = 1
  StatusDisabled int32 = 2
)

type Role struct {
  ID        uint64
  Name      string
  Code      string
  Status    int32
  Remark    string
  CreatedAt time.Time
  UpdatedAt time.Time
}

func NewRole(name, code string, status int32, remark string) (*Role, error) {
  if name == "" {
    return nil, ErrNameRequired
  }
  if code == "" {
    return nil, ErrCodeRequired
  }
  if status == 0 {
    status = StatusEnabled
  }
  return &Role{Name: name, Code: code, Status: status, Remark: remark}, nil
}
```

```go
// internal/app/user/domain/role/repository.go
package role

import "context"

type Query struct {
  Page   int32
  Limit  int32
  Name   string
  Code   string
  Status int32
}

type Repository interface {
  Create(ctx context.Context, r *Role) error
  Update(ctx context.Context, r *Role) error
  Delete(ctx context.Context, id uint64) error
  GetByID(ctx context.Context, id uint64) (*Role, error)
  List(ctx context.Context, q Query) ([]*Role, int64, error)
}
```

## 4. MySQL Adapter

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
type RoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name      string    `gorm:"column:name;type:varchar(50);not null;comment:Role Name"`
  Code      string    `gorm:"column:code;type:varchar(50);not null;uniqueIndex;comment:Role Code"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:Status"`
  Remark    string    `gorm:"column:remark;type:varchar(255);not null;default:'';comment:Remark"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RoleModel) TableName() string { return "role" }
```

Repository implementation with list support:

```go
func (r *RoleRepository) Create(ctx context.Context, ro *role.Role) error {
  model := toRoleModel(ro)
  if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
    return err
  }
  ro.ID = model.ID
  return nil
}

func (r *RoleRepository) GetByID(ctx context.Context, id uint64) (*role.Role, error) {
  var m RoleModel
  err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
  if errors.Is(err, gorm.ErrRecordNotFound) {
    return nil, nil
  }
  if err != nil {
    return nil, err
  }
  return toRoleDomain(&m), nil
}

func (r *RoleRepository) List(ctx context.Context, q role.Query) ([]*role.Role, int64, error) {
  var models []RoleModel
  var total int64

  db := r.db.WithContext(ctx).Model(&RoleModel{})
  if q.Name != "" {
    db = db.Where("name LIKE ?", "%"+q.Name+"%")
  }
  if q.Status > 0 {
    db = db.Where("status = ?", q.Status)
  }

  if err := db.Count(&total).Error; err != nil {
    return nil, 0, err
  }
  offset := (int(q.Page) - 1) * int(q.Limit)
  if err := db.Offset(offset).Limit(int(q.Limit)).Order("id DESC").Find(&models).Error; err != nil {
    return nil, 0, err
  }

  roles := make([]*role.Role, len(models))
  for i, m := range models {
    roles[i] = toRoleDomain(&m)
  }
  return roles, total, nil
}
```

Model-domain conversion functions:

```go
func toRoleModel(d *role.Role) *RoleModel {
  return &RoleModel{
    ID:     d.ID,
    Name:   d.Name,
    Code:   d.Code,
    Status: d.Status,
    Remark: d.Remark,
  }
}

func toRoleDomain(m *RoleModel) *role.Role {
  return &role.Role{
    ID:        m.ID,
    Name:      m.Name,
    Code:      m.Code,
    Status:    m.Status,
    Remark:    m.Remark,
    CreatedAt: m.CreatedAt,
    UpdatedAt: m.UpdatedAt,
  }
}
```

::: warning
Repository implementations must not be imported by the domain layer. Use Wire to inject the `role.Repository` interface.
:::

## 5. Migration

Register the model:

```go
func MigrationModels() []any {
  return []any{
    &mysql.RoleModel{},
    &mysql.UserModel{},
    &mysql.DeptModel{},
  }
}
```

Generate migration:

```bash
make migrate.new SERVICE=user NAME=add_role_table
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

## 6. Application

The application layer handles business logic:

```go
type CreateRoleCommand struct {
  Name   string
  Code   string
  Status int32
  Remark string
}

type RoleUseCase struct {
  roles role.Repository
}

func NewRoleUseCase(roles role.Repository) *RoleUseCase {
  return &RoleUseCase{roles: roles}
}

func (uc *RoleUseCase) Create(ctx context.Context, cmd CreateRoleCommand) (*role.Role, error) {
  r, err := role.NewRole(cmd.Name, cmd.Code, cmd.Status, cmd.Remark)
  if err != nil {
    return nil, err
  }
  if err := uc.roles.Create(ctx, r); err != nil {
    return nil, err
  }
  return r, nil
}
```

Application layer responsibilities:

- Local transactions.
- Duplicate checks.
- Delete protection.
- Permission boundary validation.
- Data permission filtering.
- Cross-service calls.
- DTM orchestration.

## 7. Controller

Controller translates proto requests to application commands:

```go
func (c *RoleGRPC) AddRole(ctx context.Context, in *pb.AddRoleRequest) (*pb.AddRoleResponse, error) {
  r, err := c.roleUC.Create(ctx, application.CreateRoleCommand{
    Name:   in.GetName(),
    Code:   in.GetCode(),
    Status: in.GetStatus(),
    Remark: in.GetRemark(),
  })
  if err != nil {
    return nil, err
  }
  return &pb.AddRoleResponse{Id: r.ID}, nil
}
```

Full controller example with list and copier:

```go
func (c *RoleGRPC) GetRoleList(ctx context.Context, in *pb.GetRoleListRequest) (*pb.GetRoleListResponse, error) {
  list, total, err := c.roleUseCase.List(ctx, application.ListRoleQuery{
    Page:   int(in.GetPage()),
    Limit:  int(in.GetLimit()),
    Name:   in.GetName(),
    Status: in.GetStatus(),
  })
  if err != nil {
    return nil, err
  }

  var out pb.GetRoleListResponse
  out.Total = uint64(total)
  if err = copier.Copy(&out.Roles, &list); err != nil {
    return nil, platformi18n.ErrorFailed(ctx, "CopierFailed", nil)
  }
  return &out, nil
}
```

Controller rules:

- Use `in.GetXxx()` to read request fields (nil-safe).
- Proto DTO -> application command/query struct.
- Call usecase method.
- Domain result -> proto response (via `copier.Copy`).
- Use `platformi18n.ErrorFailed` for i18n errors.
- Log operations via `auditlog`.

::: tip
Controllers should not contain business logic. Business rules (transactions, permission boundaries, duplicate checks) belong in the application/service layer.
:::

## 8. Service Layer

The service layer sits between controller and application, handling cross-cutting concerns:

```go
type RoleService struct {
  Options
}

func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) (err error) {
  // 1. Data permission check
  scope, err := s.DataScope(ctx)
  if err != nil {
    return err
  }
  if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
    return err
  }

  // 2. Set data ownership
  if !scope.IsAdmin {
    role.OwnerUserID = scope.UserID
    role.OwnerDeptID = scope.DeptID
  }

  // 3. Call application usecase
  result, err := s.RoleUseCase.CreateRole(ctx, roleCommandFromStore(role))
  if err != nil {
    return mapRoleDomainError(ctx, err)
  }
  role.ID = result.ID
  return nil
}
```

Service layer responsibilities:

- Data permission (DataScope) check and filtering.
- Automatic data ownership field population.
- Domain error to i18n error mapping.
- Cross-aggregate orchestration (e.g., cache cleanup after role deletion).

## 9. Copier Tag Usage

`jinzhu/copier` handles domain -> proto response field mapping. Proto `tagger.tags` annotations control the mapping:

```protobuf
// Basic field mapping
uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];

// Time fields use helper methods
google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];

// Sensitive fields use masking methods
string password = 6 [(tagger.tags) = "copier:\"HiddenPasswordToRPC\""];

// Complex fields use custom methods
repeated RolePermissionPolicy policies = 12 [
  (tagger.tags) = "copier:\"PoliciesToRPC\""
];
```

Common copier mapping patterns:

| Proto Field Type | Copier Value | Description |
|-----------------|--------------|-------------|
| `uint64` | `ID` | Direct mapping |
| `Timestamp` | `CreatedAtToRPC` | `time.Time` -> `*timestamppb.Timestamp` |
| `string` (sensitive) | `HiddenPasswordToRPC` | Returns masked value like `***` |
| `repeated` | `PoliciesToRPC` | Calls custom conversion method |

## 10. Frontend

```ts
export function addRole(data: AddRoleRequest) {
  return api.post('/user.v1.RoleService/AddRole', data)
}
```

Bind routeMenu APIs through generated constants:

```ts
apis: [APIs.user.v1.RoleService.AddRole]
```

Frontend API module template:

```ts
// web/src/api/modules/role.ts
import api from '../index'

export interface AddRoleRequest {
  name: string
  code: string
  status: number
  remark?: string
}

export function addRole(data: AddRoleRequest) {
  return api.post('/user.v1.RoleService/AddRole', data)
}

export function getRoleList(data: GetRoleListRequest) {
  return api.post('/user.v1.RoleService/GetRoleList', data)
}
```

After backend proto changes:

1. Run `make gen` to generate OpenAPI spec.
2. Run `pnpm run build` which auto-updates `api-manifest.ts`.
3. Manually update or add functions in `api/modules/<service>.ts`.

## 11. Permission Contract

The permission contract `permission-contract.json` is extracted from frontend routeMenu config and api-manifest:

```bash
cd web
pnpm run contract:gen
```

The output contains all API paths and their corresponding menu/button IDs, used by the backend for runtime permission checks.

Build flow:

```text
routeMenu (config/*.ts)
  + api-manifest.ts
    -> permission-contract.json
      -> backend permission enforcement
```

::: tip
After modifying routeMenu, always rebuild to update the permission contract.
:::

## 12. Validation Middleware

Before requests reach the controller, validation middleware runs automatically:

```text
HTTP/gRPC request
  -> decode protobuf/JSON body
  -> extract validate rules from tagger.tags
  -> validate each field (required, min, max, pattern...)
  -> validation fails -> return INVALID_ARGUMENT + label as error message
  -> validation passes -> call Controller
```

Validation rule mapping:

| Proto validate | Meaning | Frontend equivalent |
|---------------|---------|-------------------|
| `required` | Non-empty | `trigger: 'blur'` |
| `min=N` | Min length/value | `{ min: N }` |
| `max=N` | Max length/value | `{ max: N }` |
| `email` | Email format | `{ type: 'email' }` |
| `gt=0` | Greater than 0 | Custom rule |

## 13. Error Handling Chain

The complete error propagation from domain layer to HTTP response:

```text
domain error (ErrNameRequired)
  -> service: mapRoleDomainError(ctx, err)
    -> platformi18n.ErrorFailed(ctx, "RoleNameRequired", nil)
      -> go-i18n looks up translation for accept-language
      -> constructs gRPC status.Error(codes.FailedPrecondition, translated_msg)
        -> gRPC -> HTTP transcoding
          -> HTTP 400 + JSON body { code: 3, message: "Role name is required" }
```

Core i18n error function:

```go
// internal/platform/i18n/message.go
func ErrorFailed(ctx context.Context, messageID string, templateData map[string]any) error {
  msg := localizeMessage(ctx, messageID, templateData)
  return status.Error(codes.FailedPrecondition, msg)
}
```

Language detection order:

1. gRPC metadata `accept-language` header.
2. gRPC metadata `grpcgateway-accept-language` header (gateway forwarded).
3. Default `zh-CN`.

i18n message files live in `internal/platform/i18n/locales/`:

- `active.zh-CN.toml` -- Chinese messages.
- `active.en.toml` -- English messages.

::: tip
When adding new error messages, add entries to both `active.zh-CN.toml` and `active.en.toml`.
:::

## 14. Validate

```bash
make gen
make service.check SERVICE=user
go test -race ./internal/app/user/...
cd web && pnpm run build
make e2e E2E_TIMEOUT=20m
```

## Complete Command Sequence

```bash
make gen
make migrate.new SERVICE=user NAME=add_role_table
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
make service.check SERVICE=user
go test -race ./internal/app/user/...
cd web && pnpm run build
make e2e E2E_TIMEOUT=20m
```
