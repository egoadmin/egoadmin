# API Development Workflow

A complete management API change usually touches proto, generated code, domain, persistence, application, controller, service registration, permissions, frontend and tests.

## 1. Define Proto

```protobuf
service RoleService {
  rpc AddRole(AddRoleRequest) returns (AddRoleResponse) {
    option (google.api.http) = {
      post: "/user.v1.RoleService/AddRole"
      body: "*"
    };
  }
}

message AddRoleRequest {
  string name = 1 [
    (tagger.tags) = "validate:\"required\" label:\"Role Name\"",
    (google.api.field_behavior) = REQUIRED
  ];
  string code = 2 [
    (tagger.tags) = "validate:\"required\" label:\"Role Code\"",
    (google.api.field_behavior) = REQUIRED
  ];
}

message AddRoleResponse {
  uint64 id = 1;
}
```

Rules:

- Use a separate request and response message for every RPC.
- Use HTTP `post` with `body: "*"`.
- Required input fields need both validation tags and `field_behavior = REQUIRED`.
- OpenAPI auth docs must match backend API classification.

## 2. Generate Code

```bash
make gen
```

Generated code under `api/gen/go` is output. Do not hand edit generated files.

## 3. Domain

```go
package role

type Role struct {
  ID     uint64
  Name   string
  Code   string
  Status int32
  Remark string
}

type Repository interface {
  Create(ctx context.Context, r *Role) error
  GetByID(ctx context.Context, id uint64) (*Role, error)
}
```

## 4. MySQL Adapter

```go
type RoleModel struct {
  ID     uint64 `gorm:"primaryKey;autoIncrement;column:id"`
  Name   string `gorm:"column:name;type:varchar(50);not null"`
  Code   string `gorm:"column:code;type:varchar(50);not null;uniqueIndex"`
  Status int32  `gorm:"column:status;type:tinyint;not null;default:1"`
}

func (RoleModel) TableName() string { return "role" }
```

```go
func (r *RoleRepository) Create(ctx context.Context, ro *role.Role) error {
  model := toRoleModel(ro)
  if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
    return err
  }
  ro.ID = model.ID
  return nil
}
```

## 5. Migration

Register the model:

```go
func MigrationModels() []any {
  return []any{
    &mysql.RoleModel{},
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

```go
type CreateRoleCommand struct {
  Name string
  Code string
}

type RoleUseCase struct {
  roles role.Repository
}

func (uc *RoleUseCase) Create(ctx context.Context, cmd CreateRoleCommand) (*role.Role, error) {
  r := &role.Role{Name: cmd.Name, Code: cmd.Code, Status: 1}
  if err := uc.roles.Create(ctx, r); err != nil {
    return nil, err
  }
  return r, nil
}
```

## 7. Controller

```go
func (c *RoleController) AddRole(ctx context.Context, in *pb.AddRoleRequest) (*pb.AddRoleResponse, error) {
  r, err := c.roleUC.Create(ctx, application.CreateRoleCommand{
    Name: in.GetName(),
    Code: in.GetCode(),
  })
  if err != nil {
    return nil, err
  }
  return &pb.AddRoleResponse{Id: r.ID}, nil
}
```

## 8. Frontend

```ts
export function addRole(data: AddRoleRequest) {
  return api.post('/user.v1.RoleService/AddRole', data)
}
```

Bind routeMenu APIs through generated constants:

```ts
apis: [APIs.user.v1.RoleService.AddRole]
```

## 9. Validate

```bash
make gen
make service.check SERVICE=user
go test -race ./internal/app/user/...
cd web && pnpm run build
make e2e E2E_TIMEOUT=20m
```
