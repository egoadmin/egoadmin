# API 开发工作流

本章说明如何从零新增一个管理后台 API。完整链路包括：

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

## 1. 定义 Proto

业务 API 从 `api/proto` 开始。下面以新增角色 API 为例：

```protobuf
syntax = "proto3";
package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "tagger/tagger.proto";
import "protoc-gen-openapiv2/options/annotations.proto";

service RoleService {
  // AddRole 创建角色
  rpc AddRole(AddRoleRequest) returns (AddRoleResponse) {
    option (google.api.http) = {
      post: "/user.v1.RoleService/AddRole"
      body: "*"
    };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      tags: "角色服务,服务名: user.v1.RoleService"
      tags: "RoleService"
      description: "认证: 需要 Authorization: Bearer <token>，并需要接口权限 USER.V1.ROLESERVICE/ADDROLE。"
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
    (tagger.tags) = "validate:\"required\" label:\"角色名称\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "角色名称"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  string code = 2 [
    (tagger.tags) = "validate:\"required\" label:\"角色编码\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "角色编码"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  int32 status = 3 [
    (tagger.tags) = "validate:\"required\" label:\"状态\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "状态"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  string remark = 4 [
    (tagger.tags) = "label:\"备注\""
  ];
}

message AddRoleResponse {
  uint64 id = 1;
}
```

Proto 规则：

- 每个 RPC 都定义独立 request 和 response。
- HTTP 兼容统一使用 `post` 和 `body: "*"`。
- 必填请求字段同时写 `validate:"required"` 和 `google.api.field_behavior = REQUIRED`。
- OpenAPI 认证描述要和实际 API 分类一致。

## 2. 生成代码

```bash
make gen
```

该命令生成：

| 产物 | 用途 |
|------|------|
| `*.pb.go` | protobuf message |
| `*_grpc.pb.go` | gRPC server/client 接口 |
| `*.pb.gw.go` | grpc-gateway HTTP 兼容 |
| `wire_gen.go` | Wire 注入生成代码 |
| OpenAPI 输出 | API 文档和 manifest 输入 |

::: warning
不要手改 generated 文件。修改源 proto 或 Wire provider 后重新执行 `make gen`。
:::

## 3. 定义领域模型

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

## 4. 实现 MySQL Adapter

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
package mysql

import "time"

type RoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name      string    `gorm:"column:name;type:varchar(50);not null;comment:角色名称"`
  Code      string    `gorm:"column:code;type:varchar(50);not null;uniqueIndex;comment:角色编码"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
  Remark    string    `gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RoleModel) TableName() string {
  return "role"
}
```

```go
// internal/app/user/adapter/persistence/mysql/role_repository.go
package mysql

import (
  "context"
  "errors"

  "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
  "gorm.io/gorm"
)

type RoleRepository struct {
  db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) *RoleRepository {
  return &RoleRepository{db: db}
}

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

## 5. 注册迁移模型

```go
// internal/app/user/schema/schema.go
func MigrationModels() []any {
  return []any{
    &mysql.RoleModel{},
    &mysql.UserModel{},
    &mysql.DeptModel{},
  }
}
```

生成迁移：

```bash
make migrate.new SERVICE=user NAME=add_role_table
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

## 6. 实现 Application

```go
// internal/app/user/application/role_usecase.go
package application

import (
  "context"

  "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
)

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

Application 层适合处理：

- 本地事务。
- duplicate check。
- 删除保护。
- 权限边界验证。
- 数据权限过滤。
- 跨服务调用。
- DTM 编排。

## 7. 实现 Controller

```go
// internal/app/user/controller/role_grpc.go
package controller

import (
  "context"

  pb "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
  "github.com/egoadmin/egoadmin/internal/app/user/application"
)

type RoleController struct {
  pb.UnimplementedRoleServiceServer
  roleUC *application.RoleUseCase
}

func NewRoleController(roleUC *application.RoleUseCase) *RoleController {
  return &RoleController{roleUC: roleUC}
}

func (c *RoleController) AddRole(ctx context.Context, in *pb.AddRoleRequest) (*pb.AddRoleResponse, error) {
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

Controller 只做：

- 使用 `in.GetXxx()` 读取请求字段。
- proto DTO -> application command/query。
- 调用 usecase。
- application result -> proto response。
- 按已有模式记录操作日志。

## 8. 注册 gRPC 服务

```go
// internal/app/user/server/grpc_server.go
func NewGrpcServer(
  roleCtrl *controller.RoleController,
  userCtrl *controller.UserController,
) *grpc.Server {
  s := grpc.NewServer()
  userv1.RegisterRoleServiceServer(s, roleCtrl)
  userv1.RegisterUserServiceServer(s, userCtrl)
  return s
}
```

## 9. 前端 API 模块

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
```

## 10. routeMenu 绑定

```ts
import { APIs } from '@/api/api-manifest'

{
  id: 30301,
  parentId: 30300,
  type: MenuType.Page,
  name: 'route.role',
  path: '/user/role',
  title: '角色管理',
  locale: 'menu.user.role',
  icon: 'ep:user',
  apis: [APIs.user.v1.RoleService.GetRoleList],
  children: [
    {
      id: 30302,
      type: MenuType.Button,
      title: '新增角色',
      apis: [APIs.user.v1.RoleService.AddRole],
    },
  ],
}
```

构建权限合约：

```bash
cd web
pnpm run build
```

## 11. 测试

最小验证：

```bash
make gen
make service.check SERVICE=user
go test -race ./internal/app/user/...
cd web && pnpm run type-check && pnpm run build
```

如果该接口是 gateway 可见管理流程，补充：

```bash
make e2e E2E_TIMEOUT=20m
```

## 完整命令串

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

