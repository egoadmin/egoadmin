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
| `*.pb.gw.go` | protoc-gen-go-http HTTP 兼容 |
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

## 进阶：Proto 文件完整注解说明

下面是一段带完整注解的 proto 字段定义，展示了项目中使用的所有 annotation 类型：

```protobuf
message Role {
  uint64 id = 1 [
    (tagger.tags) = "copier:\"ID\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "角色ID"
    }
  ];

  string name = 2 [
    (tagger.tags) = "validate:\"required\" label:\"角色名称\" copier:\"Name\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "角色名称"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  google.protobuf.Timestamp created_at = 3 [
    (tagger.tags) = "copier:\"CreatedAtToRPC\""
  ];
}
```

各注解用途：

| 注解 | 生成产物 | 用途 |
|------|----------|------|
| `tagger.tags` → `validate` | 校验中间件 | 字段验证规则 |
| `tagger.tags` → `label` | 错误信息 | i18n 错误提示文案 |
| `tagger.tags` → `copier` | `copier.Copy` 字段映射 | domain → proto response 转换 |
| `google.api.http` | HTTP 路由 | gRPC HTTP 兼容路径 |
| `google.api.field_behavior` | OpenAPI spec | 标记 REQUIRED/OPTIONAL |
| `openapiv2_operation` | OpenAPI spec | 接口描述、认证要求、标签 |
| `openapiv2_field` | OpenAPI spec | 字段标题和描述 |

## 进阶：protoc-gen-go-http 解释

::: warning
本项目不使用 gRPC-Gateway。HTTP 兼容层由 `protoc-gen-go-http` 提供。
:::

`protoc-gen-go-http` 是自定义 protoc 插件，读取 `google.api.http` 注解后生成独立的 HTTP 路由注册代码。与 gRPC-Gateway 的区别：

| 特性 | protoc-gen-go-http | gRPC-Gateway |
|------|-------------------|-------------|
| 二进制协议 | protobuf + JSON | protobuf + JSON |
| 路由方式 | 直接注册到 gRPC server | 独立 HTTP server + gRPC 转发 |
| 性能 | 无额外跳转 | 多一跳反向代理 |
| 依赖 | 轻量 | 依赖完整 gateway 库 |

生成产物 `*.pb.gw.go` 包含 `RegisterXxxHTTPServer` 接口和路由绑定，和 gRPC server 共用同一端口。

## 进阶：Controller 实现模式

Controller 是 proto 请求到 application 层的转换桥。完整实现示例：

```go
// internal/app/user/controller/role_grpc.go
package controller

import (
  "context"

  userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
  "github.com/egoadmin/egoadmin/internal/app/user/application"
  platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
  "github.com/jinzhu/copier"
)

type RoleGRPC struct {
  roleUseCase *application.RoleUseCase
}

func (c *RoleGRPC) GetRoleList(ctx context.Context, in *userv1.GetRoleListRequest) (*userv1.GetRoleListResponse, error) {
  list, total, err := c.roleUseCase.List(ctx, application.ListRoleQuery{
    Page:   int(in.GetPage()),
    Limit:  int(in.GetLimit()),
    Name:   in.GetName(),
    Status: in.GetStatus(),
  })
  if err != nil {
    return nil, err
  }

  var out userv1.GetRoleListResponse
  out.Total = uint64(total)
  if err = copier.Copy(&out.Roles, &list); err != nil {
    return nil, platformi18n.ErrorFailed(ctx, "CopierFailed", nil)
  }
  return &out, nil
}

func (c *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error) {
  r, err := c.roleUseCase.Create(ctx, application.CreateRoleCommand{
    Name:   in.GetName(),
    Code:   in.GetCode(),
    Status: in.GetStatus(),
    Remark: in.GetRemark(),
  })
  if err != nil {
    return nil, err
  }
  return &userv1.AddRoleResponse{Id: r.ID}, nil
}
```

Controller 职责清单：

- 使用 `in.GetXxx()` 读取请求字段（nil-safe）。
- Proto DTO → application command/query 结构体。
- 调用 usecase 方法。
- Domain result → proto response（使用 `copier.Copy`）。
- 使用 `platformi18n.ErrorFailed` 返回 i18n 错误。
- 通过 `auditlog` 记录操作日志。

::: tip
Controller 不包含业务逻辑。业务规则（事务、权限边界、重复检查）放在 application/service 层。
:::

## 进阶：Service 层模式

Service 层位于 controller 和 application 之间，处理横切关注点：

```go
// internal/app/user/service/role_service.go
package service

import (
  "context"

  platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
)

type RoleService struct {
  Options
}

func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) (err error) {
  // 1. 数据权限检查
  scope, err := s.DataScope(ctx)
  if err != nil {
    return err
  }
  if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
    return err
  }

  // 2. 设置数据归属
  if !scope.IsAdmin {
    role.OwnerUserID = scope.UserID
    role.OwnerDeptID = scope.DeptID
  }

  // 3. 调用 application usecase
  result, err := s.RoleUseCase.CreateRole(ctx, roleCommandFromStore(role))
  if err != nil {
    return mapRoleDomainError(ctx, err)
  }
  role.ID = result.ID
  return nil
}
```

Service 层职责：

- 数据权限（DataScope）检查和过滤。
- 数据归属字段自动填充。
- Domain 错误到 i18n 错误的映射。
- 跨聚合编排（如删除角色后清理缓存）。

## 进阶：Store/Repository 模式

Store 层封装 GORM 操作，domain 层通过 Repository 接口访问：

```go
// internal/app/user/adapter/persistence/mysql/role_repository.go
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

模型与领域对象转换：

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
Repository 实现不应被 domain 层 import。通过 Wire 注入 `role.Repository` 接口。
:::

## 进阶：Copier 标签用法

`jinzhu/copier` 用于 domain → proto response 的字段自动映射。通过 proto `tagger.tags` 中的 `copier` 注解控制映射关系：

```protobuf
// 基本字段映射
uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];

// 时间字段使用辅助方法
google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];

// 敏感字段使用脱敏方法
string password = 6 [(tagger.tags) = "copier:\"HiddenPasswordToRPC\""];

// 复合字段使用自定义方法
repeated RolePermissionPolicy policies = 12 [
  (tagger.tags) = "copier:\"PoliciesToRPC\""
];
```

生成的 Go struct tag 会包含 `copier:"ID"` 等，调用 `copier.Copy(&out, &domain)` 时自动按 tag 映射。

常用 copier 映射模式：

| Proto 字段类型 | copier 值 | 说明 |
|---------------|-----------|------|
| `uint64` | `ID` | 直接映射 |
| `Timestamp` | `CreatedAtToRPC` | `time.Time` → `*timestamppb.Timestamp` |
| `string`（敏感） | `HiddenPasswordToRPC` | 返回脱敏值如 `***` |
| `repeated` | `PoliciesToRPC` | 调用自定义转换方法 |

## 进阶：OpenAPI 注解最佳实践

每个 RPC 方法必须添加 OpenAPI 注解：

```protobuf
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
```

最佳实践：

- `tags` 第一个值为中文服务名，第二个为英文服务名。
- `description` 包含认证要求和权限标识。
- `security` 始终使用 `BearerAuth`。
- 必填字段同时标记 `validate:"required"` 和 `field_behavior = REQUIRED`。
- `body: "*"` 统一使用 POST，不做 RESTful 映射。

## 进阶：权限合约生成

权限合约 `permission-contract.json` 从前端 routeMenu 配置和 api-manifest 自动提取：

```bash
cd web
pnpm run contract:gen
```

生成产物包含所有 API 路径和对应的菜单/按钮 ID，后端用于运行时权限校验。

构建流程：

```text
routeMenu (config/*.ts)
  + api-manifest.ts
    → permission-contract.json
      → 后端权限校验
```

::: tip
每次新增或修改 routeMenu 后都需要重新构建，否则权限合约不会更新。
:::

## 进阶：前端 API 模块生成

后端 proto 变更后，前端 API 模块更新流程：

1. 执行 `make gen` 生成 OpenAPI spec。
2. 执行 `pnpm run build`，构建过程自动更新 `api-manifest.ts`。
3. 手动更新或新增 `api/modules/<service>.ts` 中的函数和类型。

API 模块模板：

```ts
// web/src/api/modules/role.ts
import api from '../index'

export interface AddRoleRequest {
  name: string
  code: string
  status: number
  remark?: string
}

export interface RoleItem {
  id: string
  name: string
  code: string
  status: number
  createdAt: string
}

export function addRole(data: AddRoleRequest) {
  return api.post('/user.v1.RoleService/AddRole', data)
}

export function getRoleList(data: GetRoleListRequest) {
  return api.post('/user.v1.RoleService/GetRoleList', data)
}
```

## 进阶：验证中间件流程

请求到达 Controller 之前，验证中间件自动执行：

```text
HTTP/gRPC 请求
  → 解码 protobuf/JSON body
  → 提取 tagger.tags 中的 validate 规则
  → 逐字段验证 (required, min, max, pattern...)
  → 验证失败 → 返回 INVALID_ARGUMENT + label 作为错误信息
  → 验证通过 → 调用 Controller
```

验证规则对照：

| Proto validate | 含义 | 前端对应 |
|---------------|------|---------|
| `required` | 非空 | `trigger: 'blur'` |
| `min=N` | 最小长度/值 | `{ min: N }` |
| `max=N` | 最大长度/值 | `{ max: N }` |
| `email` | 邮箱格式 | `{ type: 'email' }` |
| `gt=0` | 大于 0 | 自定义 rule |

## 进阶：错误处理链

从 domain 层到 HTTP 响应的完整错误传播链：

```text
domain error (ErrNameRequired)
  → service: mapRoleDomainError(ctx, err)
    → platformi18n.ErrorFailed(ctx, "RoleNameRequired", nil)
      → go-i18n 查找 accept-language 对应的翻译
      → 构造 gRPC status.Error(codes.FailedPrecondition, translated_msg)
        → gRPC → HTTP 转码
          → HTTP 400 + JSON body { code: 3, message: "角色名称不能为空" }
```

核心代码：

```go
// internal/platform/i18n/message.go
func ErrorFailed(ctx context.Context, messageID string, templateData map[string]any) error {
  msg := localizeMessage(ctx, messageID, templateData)
  return status.Error(codes.FailedPrecondition, msg)
}

func Message(ctx context.Context, messageID string) string {
  return localizeMessage(ctx, messageID, nil)
}
```

语言检测顺序：

1. gRPC metadata `accept-language` header。
2. gRPC metadata `grpcgateway-accept-language` header（gateway 转发）。
3. 默认 `zh-CN`。

i18n 消息文件位于 `internal/platform/i18n/locales/`：

- `active.zh-CN.toml` -- 中文消息。
- `active.en.toml` -- 英文消息。

::: tip
新增错误消息时，必须同时在 `active.zh-CN.toml` 和 `active.en.toml` 中添加对应条目。
:::

