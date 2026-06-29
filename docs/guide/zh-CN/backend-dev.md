# 后端开发指南

面向需要在 EgoAdmin 中新增或修改后端 API 的开发者。本页覆盖从 Proto 定义到持久层的完整链路，帮助你快速上手 DDD 分层架构下的日常开发。

## 概览

EgoAdmin 后端采用领域驱动设计（DDD）分层架构，服务入口在 `cmd/<service>`，业务代码位于 `internal/app/<service>`。每个服务内部按照 server、controller、application、domain、adapter 五层组织，依赖方向严格单向：server -> controller -> application -> domain，adapter 实现 domain 层定义的接口。

核心开发流程为：**定义 Proto -> make gen 生成代码 -> 实现 Controller -> 编写 Application/Domain 逻辑 -> 实现持久层 -> 更新权限与前端 -> 测试验证**。理解这条链路后，新增 API 只需按步骤填充各层代码即可。

本文以 user 服务的**角色（Role）**模块为贯穿示例，展示真实代码中的每一层写法。

## 核心用法

### 1. 定义 Proto 契约

所有 API 从 `api/proto/<service>/v1/*.proto` 开始。Proto 定义了 RPC 方法、请求/响应消息、校验标签和 copier 映射标签。

```protobuf
// api/proto/user/v1/role.proto
syntax = "proto3";
package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "tagger/tagger.proto";
import "protoc-gen-openapiv2/options/annotations.proto";

// RoleService 角色服务
service RoleService {
  // AddRole 新增角色
  rpc AddRole(AddRoleRequest) returns (AddRoleResponse) {
    option (google.api.http) = {
      post: "/user.v1.RoleService/AddRole",
      body: "*"
    };
  };

  // GetRoleList 获取角色列表
  rpc GetRoleList(GetRoleListRequest) returns (GetRoleListResponse) {
    option (google.api.http) = {
      post: "/user.v1.RoleService/GetRoleList",
      body: "*"
    };
  };
}

message AddRoleRequest {
  Role role = 1 [
    (tagger.tags) = "validate:\"required\" label:\"角色信息\"",
    (google.api.field_behavior) = REQUIRED
  ];
}

message AddRoleResponse {
  uint64 id = 1;
}

message GetRoleListRequest {
  int32 page = 1 [
    (tagger.tags) = "validate:\"required,gte=1\" label:\"页码\"",
    (google.api.field_behavior) = REQUIRED
  ];
  int32 limit = 2 [
    (tagger.tags) = "validate:\"required,gte=1,lte=50\" label:\"单页显示数量\"",
    (google.api.field_behavior) = REQUIRED
  ];
  string sort = 3 [(tagger.tags) = "validate:\"omitempty,gte=1\" label:\"排序字段\""];
  string order = 4 [(tagger.tags) = "validate:\"omitempty,oneof=desc asc\" label:\"排序方式\""];
  string name = 5 [(tagger.tags) = "validate:\"omitempty,gte=1\" label:\"角色名称\""];
}

message GetRoleListResponse {
  int32 total = 1;
  repeated Role roles = 2;
}

message Role {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  string name = 5 [
    (tagger.tags) = "validate:\"required\" label:\"角色名称\"",
    (google.api.field_behavior) = REQUIRED
  ];
  int32 typ = 6 [
    (tagger.tags) = "validate:\"required,oneof=1\" label:\"角色类型\"",
    (google.api.field_behavior) = REQUIRED
  ];
  int32 data_perm = 7 [
    (tagger.tags) = "validate:\"required,oneof=1 2 3 4\" label:\"数据权限\"",
    (google.api.field_behavior) = REQUIRED
  ];
  string desc = 8 [(tagger.tags) = "validate:\"omitempty\" label:\"角色备注\""];
  string view_menus = 9 [(tagger.tags) = "validate:\"omitempty\" label:\"页面菜单权限\""];
  string uses = 10 [(tagger.tags) = "validate:\"omitempty\" label:\"功能权限\""];
}
```

**关键点：**
- `(tagger.tags)` 中的 `validate` 标签会在代码生成时注入到 Go 结构体，由中间件自动校验。
- `copier:"ID"` 标签用于 `jinzhu/copier` 在 protobuf 消息与 store 模型之间做字段映射。
- HTTP 路由统一使用 POST + gRPC 路径风格（如 `/user.v1.RoleService/AddRole`）。

### 2. 运行代码生成

```bash
# 生成 proto、Go 代码和 Wire 代码
make gen
```

生成产物包括：
- `api/gen/go/user/v1/*.pb.go` — protobuf 消息和 gRPC 服务接口
- `api/gen/go/user/v1/*.pb.validate.go` — 校验逻辑
- `api/gen/go/user/v1/*.pb.gw.go` — protoc-gen-go-http HTTP 兼容层

::: warning
不要手动编辑生成文件。每次修改 proto 后都必须重新运行 `make gen`。
:::

### 3. 实现 Controller

Controller 层负责实现 `make gen` 生成的 gRPC 服务接口。它的职责是：接收 protobuf 请求、转换为 store/service 输入、调用业务逻辑、将结果映射回 protobuf 响应。

```go
// internal/app/user/controller/role_grpc.go
package controller

import (
    "context"
    "errors"
    "math"

    userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
    roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
    store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
    "github.com/egoadmin/egoadmin/internal/app/user/service"
    platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "github.com/jinzhu/copier"
)

// RoleGRPC 角色 grpc 控制器
type RoleGRPC struct {
    role *service.RoleService
}

func NewRoleGRPCController(role *service.RoleService) *RoleGRPC {
    return &RoleGRPC{role: role}
}

// AddRole 新增角色
func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (
    out *userv1.AddRoleResponse, err error,
) {
    // 1) 初始化 out，保证返回值非 nil
    out = &userv1.AddRoleResponse{}

    // 2) 将 protobuf 请求转换为 store 模型
    role := &store.RoleModel{
        Name:      in.GetRole().GetName(),
        Typ:       in.GetRole().GetTyp(),
        DataPerm:  in.GetRole().GetDataPerm(),
        Uses:      in.GetRole().GetUses(),
        ViewMenus: in.GetRole().GetViewMenus(),
        Desc:      in.GetRole().GetDesc(),
    }

    // 3) 调用 service 层
    err = s.role.AddRole(ctx, role)
    if err = mapRoleError(ctx, err); err != nil {
        return
    }

    // 4) 将结果写回响应
    out.Id = role.ID
    return
}

// GetRoleList 获取角色列表
func (s *RoleGRPC) GetRoleList(ctx context.Context, in *userv1.GetRoleListRequest) (
    out *userv1.GetRoleListResponse, err error,
) {
    out = &userv1.GetRoleListResponse{}

    // 从 protobuf 请求构建分页选项
    pgopt := xorm.PaginateOption{
        Page:  int(in.GetPage()),
        Limit: int(in.GetLimit()),
        Sort:  in.GetSort(),
        Order: in.GetOrder(),
    }

    // 调用 service 获取列表
    roles, total, err := s.role.GetRoleList(ctx, in.GetName(), pgopt)
    if err != nil {
        return
    }
    if total > math.MaxInt32 {
        err = platformi18n.ErrorFailed(ctx, "RoleCountExceeded", nil)
        return
    }

    out.Total = int32(total)
    out.Roles = make([]*userv1.Role, 0, len(roles))

    // 使用 copier 将 store 模型批量映射到 protobuf 消息
    if err = copier.Copy(&out.Roles, &roles); err != nil {
        return
    }
    return
}

// mapRoleError 将领域错误转为国际化错误
func mapRoleError(ctx context.Context, err error) error {
    var inUse roledomain.InUseError
    switch {
    case err == nil:
        return nil
    case errors.Is(err, roledomain.ErrNameExists):
        return platformi18n.ErrorFailed(ctx, "RoleNameExists", nil)
    case errors.As(err, &inUse):
        return platformi18n.ErrorFailed(ctx, "RoleInUseCount",
            map[string]any{"Count": inUse.Count})
    case errors.Is(err, roledomain.ErrInUse):
        return platformi18n.ErrorFailed(ctx, "RoleInUse", nil)
    default:
        return err
    }
}
```

**Controller 规则：**
- 每个方法入口第一件事是 `out = &XxxResponse{}` 初始化返回值。
- 使用 `in.GetXxx()` 访问嵌套消息字段（proto3 默认值语义安全）。
- 校验由中间件自动完成（基于 proto 中的 `validate` 标签），controller 中不需要手动校验。
- 使用 `copier.Copy` 做 protobuf <-> store 模型的批量映射。
- 不要在 controller 中写 GORM 查询。

### 4. 编写 Service / Application 层

Service 层（或 DDD 中的 Application 层）负责事务、锁、权限校验、跨服务编排和缓存失效。

```go
// internal/app/user/service/role_service.go
package service

import (
    "context"
    "errors"

    "github.com/egoadmin/egoadmin/internal/app/user/application"
    roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
    store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
    platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
    "github.com/egoadmin/elib/pkg/util/xorm"
)

type RoleService struct {
    Options
}

func NewRoleService(options Options) *RoleService {
    return &RoleService{Options: options}
}

// AddRole 新增角色
func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) error {
    // 数据权限校验
    scope, err := s.DataScope(ctx)
    if err != nil {
        return err
    }
    if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
        return err
    }
    if !scope.IsAdmin {
        role.OwnerUserID = scope.UserID
        role.OwnerDeptID = scope.DeptID
    }

    // 调用 application use case（领域编排）
    result, err := s.RoleUseCase.CreateRole(ctx, application.SaveRoleCommand{
        Name:        role.Name,
        Type:        role.Typ,
        DataPerm:    role.DataPerm,
        OwnerUserID: role.OwnerUserID,
        OwnerDeptID: role.OwnerDeptID,
        Uses:        role.Uses,
        ViewMenus:   role.ViewMenus,
        Desc:        role.Desc,
    })
    if err != nil {
        return mapRoleDomainError(ctx, err)
    }
    role.ID = result.ID
    return nil
}

// GetRoleList 获取角色列表
func (s *RoleService) GetRoleList(ctx context.Context, name string, pgopt xorm.PaginateOption) (
    roles []*store.RoleModel, total int64, err error,
) {
    scope, err := s.DataScope(ctx)
    if err != nil {
        return nil, 0, err
    }
    // 数据权限 scope 作为 GORM scope 注入查询
    roles, total, err = s.Role.GetList(ctx, name, pgopt, scope.RoleScope())
    return
}
```

**Service 规则：**
- 事务由 service 层控制（跨多个 store 操作时使用 `mysql.Transaction`）。
- 重复检查、删除守卫、状态流转等业务规则在此层或 domain 层实现。
- 返回 store 模型或 service DTO，不返回 protobuf 消息。
- 跨服务调用通过 `internal/client/*` 包进行。

### 5. 定义 Domain 层

Domain 层定义聚合根、值对象、领域错误和 Repository 接口。不依赖任何基础设施包。

```go
// internal/app/user/domain/role/role.go
package role

type Role struct {
    ID          uint64
    Name        string
    Type        int32
    BuiltIn     int32
    DataPerm    int32
    OwnerUserID uint64
    OwnerDeptID uint64
    Uses        string
    ViewMenus   string
    Desc        string
    Policies    []PermissionPolicy
}

type PermissionPolicy struct {
    Service string
    Method  string
}

// NormalizePolicies 去重并清理权限策略
func NormalizePolicies(policies []PermissionPolicy) []PermissionPolicy {
    normalized := make([]PermissionPolicy, 0, len(policies))
    seen := make(map[string]struct{}, len(policies))
    for _, p := range policies {
        service := strings.ToUpper(strings.TrimSpace(p.Service))
        method := strings.ToUpper(strings.TrimSpace(p.Method))
        if service == "" || method == "" {
            continue
        }
        key := service + "/" + method
        if _, ok := seen[key]; ok {
            continue
        }
        seen[key] = struct{}{}
        normalized = append(normalized, PermissionPolicy{
            Service: service, Method: method,
        })
    }
    return normalized
}
```

```go
// internal/app/user/domain/role/repository.go
package role

import "context"

// Repository 持久化角色聚合
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
package role

import (
    "errors"
    "fmt"
)

var (
    ErrNotFound   = errors.New("role: not found")
    ErrNameExists = errors.New("role: name exists")
    ErrInUse      = errors.New("role: in use")
)

// InUseError 携带引用计数的"正在使用"错误
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

### 6. 实现持久层（Store / Adapter）

持久层有两种并存路径：

- **迁移期路径**（`internal/app/<service>/internal/store`）：包含 GORM 模型、Repository 接口实现、Scopes 和事务辅助函数。
- **DDD 目标路径**（`internal/app/<service>/adapter/persistence/mysql`）：实现 domain 层定义的 Repository 接口，包含领域模型 <-> 持久化模型的转换。

#### 迁移期 Store 示例

```go
// internal/app/user/internal/store/interface_role.go
package store

import (
    "context"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "gorm.io/gorm"
)

type RoleInterface interface {
    Add(ctx context.Context, role *RoleModel) error
    Delete(ctx context.Context, id uint64) error
    Update(ctx context.Context, id uint64, role *RoleModel) error
    Get(ctx context.Context, id uint64, scopes ...func(*gorm.DB) *gorm.DB) (*RoleModel, error)
    GetList(ctx context.Context, name string, opt xorm.PaginateOption, scopes ...func(*gorm.DB) *gorm.DB) ([]*RoleModel, int64, error)
    GetAll(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]*RoleModel, error)
    CountByOption(ctx context.Context, scope func(*gorm.DB) *gorm.DB) (int64, error)
}
```

```go
// internal/app/user/internal/store/role.go
package store

import (
    "context"
    "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
    "github.com/gotomicro/ego-component/egorm"
    "gorm.io/gorm"
)

// RoleModel 角色 GORM 模型
type RoleModel struct {
    xorm.Model
    Name        string `gorm:"index;type:varchar(255);not null;default:'';comment:角色名称"`
    Typ         int32  `gorm:"int(10);not null;default:1;comment:类型"`
    BuiltIn     int32  `gorm:"int(10);not null;default:2;comment:1内置,2普通"`
    DataPerm    int32  `gorm:"int(10);not null;default:1;comment:数据权限"`
    OwnerUserID uint64 `gorm:"index;type:bigint(20) unsigned;not null;default:0"`
    OwnerDeptID uint64 `gorm:"index;type:bigint(20) unsigned;not null;default:0"`
    Uses        string `gorm:"type:text;not null;comment:功能权限"`
    ViewMenus   string `gorm:"type:text;not null;comment:页面菜单"`
    Desc        string `gorm:"type:varchar(255);not null;default:'';comment:描述"`
    Policies    []RolePermissionPolicy `gorm:"foreignKey:RoleModelID;references:ID"`
}

func (RoleModel) TableName() string { return "role" }

func (m *RoleModel) SetID(id uint64) {
    if m.ID == 0 { m.ID = id }
}

func (m *RoleModel) BeforeCreate(tx *gorm.DB) error {
    return mysql.SetID(m)
}

// Role 角色 Repository 实现
type Role struct {
    cc *egorm.Component
}

func NewRole(db *egorm.Component) RoleInterface {
    return &Role{cc: db}
}

// Add 新增角色，同时保存权限策略
func (m *Role) Add(ctx context.Context, role *RoleModel) error {
    role.Policies = normalizePermissionPolicies(role.Policies)
    db := mysql.DBWithContext(ctx, m.cc)
    return db.Create(&role).Error
}

// Delete 删除角色及其权限策略
func (m *Role) Delete(ctx context.Context, id uint64) error {
    return mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
        if err := tx.Where(map[string]any{"role_model_id": id}).Delete(&RolePermissionPolicy{}).Error; err != nil {
            return err
        }
        return tx.Unscoped().Delete(&RoleModel{Model: xorm.Model{ID: id}}).Error
    })
}

// GetList 分页查询角色列表
func (m *Role) GetList(ctx context.Context, name string, opt xorm.PaginateOption,
    scopes ...func(*gorm.DB) *gorm.DB,
) ([]*RoleModel, int64, error) {
    db := mysql.DBWithContext(ctx, m.cc)
    scopes = append(scopes, roleScopeNameLike(name))
    if opt.Sort == "" {
        opt.Sort = "created_at"
        opt.Order = "desc"
    }

    var total int64
    if err := db.Scopes(scopes...).Model(&RoleModel{}).Count(&total).Error; err != nil {
        return nil, 0, err
    }
    scopes = append(scopes, xorm.WithScopePaginate(opt)...)

    var roles []*RoleModel
    err := db.Scopes(scopes...).Find(&roles).Error
    return roles, total, err
}
```

#### DDD Adapter 示例

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
package mysql

import (
    roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "gorm.io/gorm"
)

// roleAggregateModel 角色聚合持久化模型
type roleAggregateModel struct {
    xorm.Model
    Name        string
    Typ         int32
    BuiltIn     int32
    DataPerm    int32
    OwnerUserID uint64
    OwnerDeptID uint64
    Uses        string
    ViewMenus   string
    Desc        string
    Policies    []rolePermissionPolicyModel `gorm:"foreignKey:RoleModelID;references:ID"`
}

func (roleAggregateModel) TableName() string { return "role" }

func (m *roleAggregateModel) SetID(id uint64) {
    if m.ID == 0 { m.ID = id }
}

func (m *roleAggregateModel) BeforeCreate(tx *gorm.DB) error {
    return platformmysql.SetID(m)
}

// roleModelFromDomain 领域模型 -> 持久化模型
func roleModelFromDomain(r *roledomain.Role) *roleAggregateModel {
    if r == nil { return nil }
    return &roleAggregateModel{
        Model:       xorm.Model{ID: r.ID},
        Name:        r.Name,
        Typ:         r.Type,
        BuiltIn:     r.BuiltIn,
        DataPerm:    r.DataPerm,
        OwnerUserID: r.OwnerUserID,
        OwnerDeptID: r.OwnerDeptID,
        Uses:        r.Uses,
        ViewMenus:   r.ViewMenus,
        Desc:        r.Desc,
        Policies:    rolePolicyModelsFromDomain(r.ID, r.Policies),
    }
}

// toDomain 持久化模型 -> 领域模型
func (m *roleAggregateModel) toDomain() *roledomain.Role {
    if m == nil { return nil }
    return &roledomain.Role{
        ID:          m.ID,
        Name:        m.Name,
        Type:        m.Typ,
        BuiltIn:     m.BuiltIn,
        DataPerm:    m.DataPerm,
        OwnerUserID: m.OwnerUserID,
        OwnerDeptID: m.OwnerDeptID,
        Uses:        m.Uses,
        ViewMenus:   m.ViewMenus,
        Desc:        m.Desc,
        Policies:    rolePoliciesToDomain(m.Policies),
    }
}
```

## 配置示例

### 服务配置（configs/user/config.toml）

```toml
[app.service]
name = "egoadmin-user"              # 服务注册名
platformName = "核心管理平台"         # 平台显示名
autoMigrate = false                  # Atlas 迁移后关闭自动迁移

[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"

[app.user]
adminPassword = "123456"             # 初始管理员密码
jwtExpire = 604800                   # JWT 过期秒数（7天）
refreshTokenExpire = 2592000         # 刷新令牌过期秒数（30天）
jwtSignKey = "local-egoadmin-jwt-sign-key"
useCaptcha = false                   # 登录验证码开关
multiLoginEnabled = true             # 多端登录
maxLoginClient = 2                   # 最大同时登录设备数

[server.http]
host = "0.0.0.0"
port = 9101
mode = "release"

[server.grpc]
name = "egoadmin-user"
host = "127.0.0.1"
port = 9102

[server.governor]
host = "0.0.0.0"
port = 9103

[client.mysql]
debug = true
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

[client.redis]
addr = "127.0.0.1:6380"
password = "egoadmin"
mode = "stub"

[registry]
scheme = "etcd"
prefix = "egoadmin"
serviceTTL = "10s"
```

### 环境变量覆盖

所有配置项均可通过 `EGOADMIN_*` 环境变量覆盖：

| 环境变量 | 对应配置项 | 示例 |
|----------|-----------|------|
| `EGOADMIN_APP_USER_JWTSIGNKEY` | `app.user.jwtSignKey` | `prod-secret-key` |
| `EGOADMIN_APP_USER_ADMINPASSWORD` | `app.user.adminPassword` | `strong-password` |
| `EGOADMIN_CLIENT_MYSQL_DSN` | `client.mysql.dsn` | `user:pass@tcp(db:3306)/egoadmin_user` |
| `EGOADMIN_SERVER_GRPC_PORT` | `server.grpc.port` | `9102` |
| `EGOADMIN_CLIENT_REDIS_ADDR` | `client.redis.addr` | `redis:6379` |
| `EGOADMIN_REGISTRY_SCHEME` | `registry.scheme` | `etcd` |

::: tip
容器部署中务必使用环境变量覆盖 DSN、密码和密钥等敏感配置，不要将生产密钥提交到代码仓库。
:::

## 完整实战示例：新增"菜单管理"API

以下演示如何从零新增一个完整 CRUD API。

### Step 1: Proto 定义

```protobuf
// api/proto/user/v1/menu.proto
syntax = "proto3";
package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "tagger/tagger.proto";

service MenuService {
  rpc AddMenu(AddMenuRequest) returns (AddMenuResponse) {
    option (google.api.http) = { post: "/user.v1.MenuService/AddMenu", body: "*" };
  };
  rpc GetMenuList(GetMenuListRequest) returns (GetMenuListResponse) {
    option (google.api.http) = { post: "/user.v1.MenuService/GetMenuList", body: "*" };
  };
}

message AddMenuRequest {
  string name = 1 [
    (tagger.tags) = "validate:\"required\" label:\"菜单名称\"",
    (google.api.field_behavior) = REQUIRED
  ];
  uint64 parent_id = 2;
  int32 sort = 3;
  string path = 4;
}

message AddMenuResponse {
  uint64 id = 1;
}

message GetMenuListRequest {}

message GetMenuListResponse {
  repeated Menu menus = 1;
}

message Menu {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  string name = 2;
  uint64 parent_id = 3;
  int32 sort = 4;
  string path = 5;
}
```

运行 `make gen`。

### Step 2: Store 层

```go
// internal/app/user/internal/store/menu.go
package store

import (
    "context"
    "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
    "github.com/gotomicro/ego-component/egorm"
    "gorm.io/gorm"
)

type MenuModel struct {
    xorm.Model
    Name     string `gorm:"type:varchar(255);not null;comment:菜单名称"`
    ParentID uint64 `gorm:"index;type:bigint(20) unsigned;not null;default:0;comment:父菜单ID"`
    Sort     int32  `gorm:"int(10);not null;default:0;comment:排序"`
    Path     string `gorm:"type:varchar(255);not null;default:'';comment:路由路径"`
}

func (MenuModel) TableName() string { return "menu" }
func (m *MenuModel) SetID(id uint64) { if m.ID == 0 { m.ID = id } }
func (m *MenuModel) BeforeCreate(tx *gorm.DB) error { return mysql.SetID(m) }

type Menu struct {
    cc *egorm.Component
}

func NewMenu(db *egorm.Component) MenuInterface {
    return &Menu{cc: db}
}

func (m *Menu) Add(ctx context.Context, menu *MenuModel) error {
    db := mysql.DBWithContext(ctx, m.cc)
    return db.Create(menu).Error
}

func (m *Menu) GetAll(ctx context.Context) ([]*MenuModel, error) {
    db := mysql.DBWithContext(ctx, m.cc)
    var menus []*MenuModel
    err := db.Order("sort ASC, id ASC").Find(&menus).Error
    return menus, err
}
```

### Step 3: Controller

```go
// internal/app/user/controller/menu_grpc.go
package controller

import (
    "context"
    userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
    store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
    "github.com/egoadmin/egoadmin/internal/app/user/service"
    "github.com/jinzhu/copier"
)

type MenuGRPC struct {
    menu *service.MenuService
}

func NewMenuGRPCController(menu *service.MenuService) *MenuGRPC {
    return &MenuGRPC{menu: menu}
}

func (s *MenuGRPC) AddMenu(ctx context.Context, in *userv1.AddMenuRequest) (
    out *userv1.AddMenuResponse, err error,
) {
    out = &userv1.AddMenuResponse{}
    menu := &store.MenuModel{
        Name:     in.GetName(),
        ParentID: in.GetParentId(),
        Sort:     in.GetSort(),
        Path:     in.GetPath(),
    }
    if err = s.menu.AddMenu(ctx, menu); err != nil {
        return
    }
    out.Id = menu.ID
    return
}

func (s *MenuGRPC) GetMenuList(ctx context.Context, in *userv1.GetMenuListRequest) (
    out *userv1.GetMenuListResponse, err error,
) {
    out = &userv1.GetMenuListResponse{Menus: make([]*userv1.Menu, 0)}
    menus, err := s.menu.GetAll(ctx)
    if err != nil {
        return
    }
    if err = copier.Copy(&out.Menus, &menus); err != nil {
        return
    }
    return
}
```

### Step 4: 注册到 Wire 和 Server

在 `controller/controller.go` 中添加到 `ProviderSet`：

```go
var ProviderSet = wire.NewSet(
    // ... 已有控制器
    NewMenuGRPCController,
)
```

在 `server/grpc_server.go` 中注册 gRPC 服务，以及在 `server/http_server.go` 中注册 HTTP 路由。

### Step 5: 权限与前端

如果该 API 面向 gateway（即前端通过 gateway 访问），需要：
1. 在 gateway 的 `controller` 中添加对应的代理方法。
2. 在前端的权限配置中添加新的 API 条目。
3. 更新前端路由和菜单。

### Step 6: 测试

```bash
# 运行单元测试
go test ./internal/app/user/...

# 运行所有服务测试
make test

# 运行端到端测试
make e2e E2E_TIMEOUT=20m
```

## 工作原理

### Wire 依赖注入

EgoAdmin 使用 google/wire 实现编译时依赖注入。每个服务的 `server/wire.go` 定义了完整的依赖图：

```go
//go:build wireinject
// +build wireinject

package server

import (
    usermysql "github.com/egoadmin/egoadmin/internal/app/user/adapter/persistence/mysql"
    "github.com/egoadmin/egoadmin/internal/app/user/application"
    "github.com/egoadmin/egoadmin/internal/app/user/controller"
    "github.com/egoadmin/egoadmin/internal/app/user/service"
    "github.com/google/wire"
)

func NewApp() (*App, error) {
    panic(wire.Build(
        // 基础设施
        newEgo,
        newEgoReady,
        newConfig,
        newHealth,
        newCasbin,
        newValidate,
        wire.Struct(new(Options), "*"),

        // 各层 ProviderSet
        application.ProviderSet,
        usermysql.ProviderSet,
        controller.ProviderSet,
        service.ProviderSet,

        // 接口绑定
        wire.Bind(new(application.RoleAssignments), new(*usermysql.UserRepository)),

        // 最终组装
        newApp,
    ))
}
```

运行 `make gen` 时 Wire 生成 `wire_gen.go`，将所有依赖在编译期解析完成。运行时无反射开销。

### 分层职责总览

```text
cmd/<service>/main.go          入口，启动 server
internal/app/<service>/
  server/                       EGO 应用、日志、配置、注册、健康检查、gRPC/HTTP、迁移
  controller/                   实现 gRPC 生成接口
  application/                  事务、锁、权限校验、跨服务编排、DTM
  domain/<aggregate>/           聚合根、实体、值对象、领域错误、Repository 接口
  adapter/persistence/mysql/    GORM 模型、Repository 实现、Scopes、迁移模型列表
  service/                      业务服务层（兼容期，包裹 store + use case）
  internal/store/               GORM 模型、Repository 接口/实现、Scopes
internal/platform/              共享基础设施（数据库、缓存、配置、健康检查等）
internal/component/             可复用 EGO 风格组件（authsession、idgen 等）
internal/client/                服务间 gRPC 调用封装
```

**依赖方向：**
```text
server -> controller -> application -> domain
adapter -> domain + platform
store  -> platform
```

::: warning
跨服务数据访问必须通过 `internal/client/*` 包，禁止直接 import 其他服务的 store 或 domain 包。
:::

## 常见问题

### Q: make gen 报错 "protoc not found"

**原因：** 未安装 protoc 或未在 PATH 中。

**解决：**
```bash
# macOS
brew install protobuf

# Linux
apt-get install -y protobuf-compiler

# 确认版本
protoc --version
```

### Q: copier.Copy 映射后字段值为零值

**原因：** proto 生成的 Go 字段和 store 模型字段名不一致，copier 按字段名匹配。

**解决：** 在 proto message 字段上添加 `copier` 标签：
```protobuf
uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
```
或在 Go 模型字段上添加 `copier` struct tag。

### Q: GORM 查询缺少数据权限过滤

**原因：** service 层未将 `DataScope` 作为 GORM scope 传递给 store 查询方法。

**解决：** 确保 service 层获取 `DataScope` 后，将其 `RoleScope()` / `UserScope()` 等方法返回的 `func(*gorm.DB) *gorm.DB` 传入 store 方法：
```go
scope, err := s.DataScope(ctx)
roles, total, err := s.Role.GetList(ctx, name, pgopt, scope.RoleScope())
```

### Q: Wire 生成代码报循环依赖

**原因：** ProviderSet 之间存在循环引用，通常是 A 层 import 了 B 层，B 层又 import 了 A 层。

**解决：** 检查依赖方向是否正确。domain 层不应 import 任何基础设施包。跨服务调用使用 `internal/client/*`。如果需要解耦，定义 interface 并通过 `wire.Bind` 绑定。

### Q: controller 返回 "permission denied"

**原因：** API 注册到 gateway 后，gateway 会校验当前用户的接口权限。如果权限列表未更新，请求会被拦截。

**解决：**
1. 确认 proto 中的 service 名和方法名大小写正确。
2. 在管理后台的角色权限设置中勾选新接口。
3. 确认 gateway 的 API 字典已同步最新 proto 定义（`make gen` 后重启 gateway）。

### Q: 迁移期 store 和 DDD adapter 应该用哪个？

**原因：** 项目中同时存在两种持久层路径。

**解决：**
- 新建的模块优先使用 `adapter/persistence/mysql`（DDD 路径），在 domain 层定义 Repository 接口。
- 已有模块如果还在 `internal/store` 中，继续在该路径迭代，不要混用。
- 长期目标是所有模块迁移到 `adapter/persistence/mysql`。

## 参考链接

- Proto 定义目录：`api/proto/<service>/v1/`
- 生成代码目录：`api/gen/go/<service>/v1/`
- user 服务 controller：`internal/app/user/controller/`
- user 服务 service：`internal/app/user/service/`
- user 服务 domain：`internal/app/user/domain/`
- user 服务 store：`internal/app/user/internal/store/`
- user 服务 DDD adapter：`internal/app/user/adapter/persistence/mysql/`
- user 服务 server/Wire：`internal/app/user/server/`
- 服务配置：`configs/<service>/config.toml`
- Wire ProviderSet：各层目录下的 `provider.go`
- 数据库迁移：`atlas/migrations/<service>/`
- Makefile 目标：`make gen`、`make build`、`make test`、`make e2e`
