# API 设计规范

本页定义 EgoAdmin 项目中 gRPC API 的设计标准，覆盖从 Proto 契约到代码生成、错误处理、OpenAPI 文档的完整规范。

## 概述

EgoAdmin 采用 **Proto-First**（契约优先）开发模式。所有业务 API 必须从 Proto 定义开始，通过 `buf` 生成 Go 桩代码，再由开发者实现 Controller 层逻辑。HTTP 兼容层由 protoc-gen-go-http 插件生成，与 gRPC 共享同一套 Controller，作为前端和外部系统的接入方式，而非 RESTful API 的独立设计。

gRPC 是 EgoAdmin 的主协议。服务间通信（gateway <-> user、gateway <-> idgen）全部走 gRPC；对外暴露的 HTTP 接口通过 protoc-gen-go-http 生成的 HTTP 兼容 handler 实现，所有 HTTP 请求统一使用 `POST` 方法加 `body: "*"` 映射，URL 路径遵循 `/<package.version.Service>/<Method>` 格式。

本文以 user 服务的 **角色（Role）** 模块为主线，结合 `api/proto/user/v1/role.proto`、`api/proto-internal/idgen/v1/idgen.proto` 等真实文件，说明每条规范的正确写法和反模式。

## 核心用法

### 1. Proto-First 开发原则

**所有业务 API 从 Proto 定义开始，不得直接在 Gin/EGO server 上添加业务路由。**

```go
// 反模式: 直接添加 Gin 路由处理业务 API
// 绝对不要这样做
func registerRoutes(r *gin.RouterGroup) {
    r.POST("/api/roles", func(c *gin.Context) { /* ... */ })
}

// 正确做法: 业务 API 定义在 proto 文件中，
// 通过 protoc-gen-go-http 自动生成 HTTP 路由
```

EgoAdmin 的 HTTP 接入层（gateway 服务中的 protoc-gen-go-http handler）负责将 `POST /api/<package.version.Service>/<Method>` 自动映射到对应 gRPC 方法。开发者**不需要手动注册 HTTP 路由**，只需在 proto 中定义 RPC 方法并添加 HTTP 注解。

### 2. Proto 文件结构

Proto 文件存放位置：

| 类型 | 路径 | 用途 |
|------|------|------|
| 公共 API | `api/proto/<service>/v1/<resource>.proto` | 对外暴露的业务接口 |
| 内部 API | `api/proto-internal/<service>/v1/<resource>.proto` | 服务间内部调用 |

::: tip
公共 API 由 gateway 代理转发，会生成 OpenAPI 文档并参与权限管理。内部 API 仅供服务间直接 gRPC 调用，不生成 HTTP 映射和 OpenAPI 文档。
:::

每个 proto 文件的结构遵循固定模式：

```protobuf
// api/proto/user/v1/role.proto
syntax = "proto3";

package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/protobuf/timestamp.proto";
import "protoc-gen-openapiv2/options/annotations.proto";
import "tagger/tagger.proto";

// 服务级 OpenAPI 文档声明
option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: {
    title: "角色服务"                           // 中文服务名
    description: "服务名: user.v1.RoleService"  // 用于文档说明
    version: "1.0"
    license: { name: "MIT" }
  }
  schemes: HTTP
  consumes: "application/json"
  produces: "application/json"
  // Bearer 认证定义，所有受保护接口共享
  security_definitions: {
    security: {
      key: "BearerAuth"
      value: {
        type: TYPE_API_KEY
        in: IN_HEADER
        name: "Authorization"
        description: "JWT 认证头。Swagger UI 中请输入完整值: Bearer <token>"
      }
    }
  }
};

// Service 定义
service RoleService {
  // 服务标签（OpenAPI 分组用）
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_tag) = {
    description: "角色服务,服务名: user.v1.RoleService"
  };

  // 每个 RPC 方法都需要 HTTP 映射和 OpenAPI 注解
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
        security_requirement: { key: "BearerAuth" value: {} }
      }
    };
  }
}
```

文件头部必须包含的 import 列表：

| Import | 用途 |
|--------|------|
| `google/api/annotations.proto` | HTTP 映射注解 |
| `google/api/field_behavior.proto` | `REQUIRED` / `OPTIONAL` 字段行为 |
| `google/protobuf/timestamp.proto` | 时间戳类型 |
| `protoc-gen-openapiv2/options/annotations.proto` | OpenAPI 文档注解 |
| `tagger/tagger.proto` | 校验标签和 copier 标签 |

### 3. 请求/响应消息设计

**每个 RPC 方法使用独立的 Request 和 Response 消息，不复用。**

```protobuf
// 正确: AddRole 专用请求
message AddRoleRequest {
  Role role = 1 [
    (tagger.tags) = "validate:\"required\" label:\"角色信息\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "角色信息"
    },
    (google.api.field_behavior) = REQUIRED
  ];
}

message AddRoleResponse {
  uint64 id = 1;
}

// 错误: 多个 RPC 共用一个 CreateRequest
// message CreateRequest { ... }  // AddRole 和 AddUser 不应共用
```

**字段注解三件套**：验证标签 + OpenAPI 字段标题 + field_behavior：

```protobuf
message GetRoleListRequest {
  // 验证标签: required + 范围约束
  // OpenAPI 标题: 用于生成文档
  // field_behavior: REQUIRED 标记
  int32 page = 1 [
    (tagger.tags) = "validate:\"required,gte=1\" label:\"页码\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "页码"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  int32 limit = 2 [
    (tagger.tags) = "validate:\"required,gte=1,lte=50\" label:\"单页显示数量\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
      title: "单页显示数量"
    },
    (google.api.field_behavior) = REQUIRED
  ];

  // 可选字段: validate 标签以 omitempty 开头
  string sort = 3 [
    (tagger.tags) = "validate:\"omitempty,gte=1\" label:\"排序字段\""
  ];

  string order = 4 [
    (tagger.tags) = "validate:\"omitempty,oneof=desc asc\" label:\"排序方式\""
  ];
}
```

**时间字段统一使用 `google.protobuf.Timestamp`，配合 copier 标签映射 Go 结构体字段名：**

```protobuf
// Role 共享实体消息（被多个 Request/Response 引用）
message Role {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];
  google.protobuf.Timestamp updated_at = 3 [(tagger.tags) = "copier:\"UpdatedAtToRPC\""];
  google.protobuf.Timestamp deleted_at = 4 [(tagger.tags) = "copier:\"DeletedAtToRPC\""];
  string name = 5 [
    (tagger.tags) = "validate:\"required\" label:\"角色名称\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "角色名称" },
    (google.api.field_behavior) = REQUIRED
  ];
  int32 data_perm = 7 [
    (tagger.tags) = "validate:\"required,oneof=1 2 3 4\" label:\"数据权限\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "数据权限" },
    (google.api.field_behavior) = REQUIRED
  ];
}
```

::: warning
请求/响应消息必须独立定义，不跨 RPC 复用。共享实体（如 `Role`、`User`）可以被多个 RPC 的 Request/Response 消息引用，但 Request/Response 消息本身不能共用。
:::

### 4. HTTP 映射规则

EgoAdmin 所有 RPC 统一使用 `POST` 方法加 `body: "*"` 映射，URL 模式为：

```
POST /<package.version>.<ServiceName>/<MethodName>
```

示例：

```protobuf
// 用户登录
rpc Login(LoginRequest) returns (LoginResponse) {
  option (google.api.http) = {
    post: "/user.v1.UserService/Login"
    body: "*"
  };
}

// 角色列表
rpc GetRoleList(GetRoleListRequest) returns (GetRoleListResponse) {
  option (google.api.http) = {
    post: "/user.v1.RoleService/GetRoleList"
    body: "*"
  };
}
```

::: danger
禁止使用 RESTful URL 路径参数设计。不要写 `get: "/roles/{id}"` 或 `put: "/roles/{id}"` 这样的模式。EgoAdmin 的 HTTP 兼容层统一使用 POST + body 方式，这与 gRPC 语义一致，也简化了前端调用。
:::

### 5. 错误处理规范

每个服务在 `api/proto/<service>/v1/errors.proto` 中定义错误枚举，配合 `@plugins=protoc-gen-go-errors` 插件自动生成 Go 错误码：

```protobuf
// api/proto/user/v1/errors.proto
syntax = "proto3";
package user.v1;

// @plugins=protoc-gen-go-errors
// ErrorUser 错误
enum ErrorUser {
  // @code=INTERNAL 内部服务错误
  ERROR_USER_UNKNOWN_UNSPECIFIED = 0;
  // @code=UNKNOWN 用户导入重复
  ERROR_USER_IMPORT = 1;
  // @code=UNKNOWN 组织不允许删除
  ERROR_USER_DEPT_NOT_DEL = 2;
}
```

Controller 层将领域错误映射为 i18n 错误返回：

```go
// internal/app/user/controller/role_grpc.go
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

错误处理链路为：**Domain 层返回领域错误 -> Controller 的 mapXxxError 函数匹配 -> 转换为 i18n 错误 -> gRPC status 返回客户端**。

### 6. 校验与 Copier 标签

**校验标签（validate）** 由 tagger 插件在生成代码中嵌入，Validator 中间件在 Controller 入口自动执行：

```protobuf
// 常用校验规则
string name = 1 [(tagger.tags) = "validate:\"required,gte=2,lte=50\""];
string order = 2 [(tagger.tags) = "validate:\"omitempty,oneof=desc asc\""];
uint64 id = 3 [(tagger.tags) = "validate:\"required,gte=1\""];
string password = 4 [
  (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
    extensions: {
      key: "x-go-tag-validate"
      value: { string_value: "omitempty,gte=6,lte=32" }
    }
  }
];
```

**Copier 标签** 控制 Proto 字段到 Go 结构体的映射名：

```protobuf
// copier 标签映射到 Go 结构体中的字段名
uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];             // -> Go: ID
google.protobuf.Timestamp created_at = 2
    [(tagger.tags) = "copier:\"CreatedAtToRPC\""];           // -> Go: CreatedAtToRPC
```

Controller 层使用 `copier.Copy` 将 store/domain 模型转为 Proto 响应：

```go
func (s *RoleGRPC) GetRole(ctx context.Context, in *userv1.GetRoleRequest) (
    out *userv1.GetRoleResponse, err error) {
    out = &userv1.GetRoleResponse{Role: &userv1.Role{}}

    role, err := s.role.GetRole(ctx, in.GetId())
    if err != nil {
        return
    }

    // copier 根据 proto 上的 copier 标签自动映射字段名
    if err = copier.Copy(&out.Role, &role); err != nil {
        return
    }
    return
}
```

### 7. 代码生成

一键生成所有代码：

```bash
make gen  # 等同于: make gen.proto gen.proto.internal gen.go gen.wire
```

按步骤生成：

```bash
# 公共 proto: buf lint + buf generate，输出到 api/gen/go 和 api/gen/openapi
make gen.proto

# 内部 proto: buf lint + buf generate，输出到 api/gen/go
make gen.proto.internal

# go generate: 注解代码、mock 等
make gen.go

# Wire 依赖注入: 可选 SERVICE=<service> 只生成单服务
make gen.wire
make gen.wire SERVICE=user
```

生成产物：

| 目录 | 内容 |
|------|------|
| `api/gen/go/<service>/v1/` | Go protobuf 生成代码（gRPC server/client 接口、消息结构体） |
| `api/gen/openapi/` | OpenAPI YAML 文件（供 gateway 嵌入） |
| `internal/app/<service>/server/wire_gen.go` | Wire 依赖注入生成代码 |

::: danger
禁止手动编辑任何生成文件。`api/gen/` 下的所有文件和 `wire_gen.go` 都会在下次 `make gen` 时被覆盖。如需修改，请修改对应的 `.proto` 文件或 Wire injector 文件。
:::

### 8. OpenAPI 文档

Gateway 在启动时嵌入 `openapi.yaml` 并在运行时提供 Swagger UI。每个 RPC 的 OpenAPI 注解控制文档中的描述、分组和安全要求。

**公开接口（无需认证）**：

```protobuf
rpc Login(LoginRequest) returns (LoginResponse) {
  option (google.api.http) = {
    post: "/user.v1.UserService/Login"
    body: "*"
  };
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
    tags: "用户服务,服务名: user.v1.UserService"
    tags: "UserService"
    description: "认证: 公开接口，无需 Authorization。"
    // 不添加 security 块 = 无需认证
  };
};
```

**受保护接口（需要 Bearer Token）**：

```protobuf
rpc GetMenus(GetMenusRequest) returns (GetMenusResponse) {
  option (google.api.http) = {
    post: "/user.v1.UserService/GetMenus"
    body: "*"
  };
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
    tags: "用户服务,服务名: user.v1.UserService"
    tags: "UserService"
    description: "认证: 需要 Authorization: Bearer <token>，无需接口权限。"
    security: {
      security_requirement: { key: "BearerAuth" value: {} }
    }
  };
};
```

**受保护接口 + 接口权限控制**：

```protobuf
option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
  description: "认证: 需要 Authorization: Bearer <token>，并需要接口权限 USER.V1.ROLESERVICE/ADDROLE。"
  security: {
    security_requirement: { key: "BearerAuth" value: {} }
  }
};
```

::: warning
所有受保护接口必须同时添加 `security` 块和接口权限说明。缺少安全声明会导致 Swagger UI 中该接口不需要认证即可调试，与实际运行时行为不一致。
:::

## 配置示例

### Buf 代码生成配置

```yaml
# buf.gen.yaml (公共 proto 生成)
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: api/gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: api/gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc-ecosystem/gateway
    out: api/gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc-ecosystem/openapiv2
    out: api/gen/openapi
  # tagger 插件: 生成 validate/copier 标签
  - local: protoc-gen-tag
    out: api/gen/go
    opt: paths=source_relative
```

```yaml
# buf.gen.internal.yaml (内部 proto 生成)
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: api/gen/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: api/gen/go
    opt: paths=source_relative
```

### Proto 包目录结构

```text
api/
  proto/                      # 公共 API 定义
    buf.yaml                  # buf lint 和 breaking 配置
    user/
      v1/
        user.proto            # 用户服务
        role.proto            # 角色服务
        dept.proto            # 部门服务
        center.proto          # 用户中心服务
        log.proto             # 日志服务
        errors.proto          # 错误码定义
  proto-internal/             # 内部 API 定义
    buf.yaml
    idgen/
      v1/
        idgen.proto           # ID 生成服务（内部）
    user/
      v1/
        auth_internal.proto   # 内部认证（内部）
  gen/                        # 生成产物（禁止手动编辑）
    go/
    openapi/
```

## 实战示例

### 示例：新增一个「公告管理」API

以下完整演示从 Proto 到 Controller 的新增流程，以 user 服务的公告（Notice）资源为例。

**步骤 1：定义 Proto**

```protobuf
// api/proto/user/v1/notice.proto
syntax = "proto3";
package user.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/protobuf/timestamp.proto";
import "protoc-gen-openapiv2/options/annotations.proto";
import "tagger/tagger.proto";

option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: { title: "公告服务" description: "服务名: user.v1.NoticeService" version: "1.0" }
  schemes: HTTP
  consumes: "application/json"
  produces: "application/json"
  security_definitions: {
    security: {
      key: "BearerAuth"
      value: { type: TYPE_API_KEY in: IN_HEADER name: "Authorization"
               description: "JWT 认证头。Swagger UI 中请输入完整值: Bearer <token>" }
    }
  }
};

service NoticeService {
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_tag) = {
    description: "公告服务,服务名: user.v1.NoticeService"
  };

  // AddNotice 新增公告
  rpc AddNotice(AddNoticeRequest) returns (AddNoticeResponse) {
    option (google.api.http) = { post: "/user.v1.NoticeService/AddNotice" body: "*" };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      tags: "公告服务,服务名: user.v1.NoticeService"
      tags: "NoticeService"
      description: "认证: 需要 Authorization: Bearer <token>，并需要接口权限 USER.V1.NOTICESERVICE/ADDNOTICE。"
      security: { security_requirement: { key: "BearerAuth" value: {} } }
    };
  };

  // GetNoticeList 获取公告列表
  rpc GetNoticeList(GetNoticeListRequest) returns (GetNoticeListResponse) {
    option (google.api.http) = { post: "/user.v1.NoticeService/GetNoticeList" body: "*" };
    option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
      tags: "公告服务,服务名: user.v1.NoticeService"
      tags: "NoticeService"
      description: "认证: 需要 Authorization: Bearer <token>，并需要接口权限 USER.V1.NOTICESERVICE/GETNOTICELIST。"
      security: { security_requirement: { key: "BearerAuth" value: {} } }
    };
  };
}

message AddNoticeRequest {
  Notice notice = 1 [
    (tagger.tags) = "validate:\"required\" label:\"公告信息\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "公告信息" },
    (google.api.field_behavior) = REQUIRED
  ];
}

message AddNoticeResponse {
  uint64 id = 1;
}

message GetNoticeListRequest {
  int32 page = 1 [
    (tagger.tags) = "validate:\"required,gte=1\" label:\"页码\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "页码" },
    (google.api.field_behavior) = REQUIRED
  ];
  int32 limit = 2 [
    (tagger.tags) = "validate:\"required,gte=1,lte=50\" label:\"单页显示数量\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "单页显示数量" },
    (google.api.field_behavior) = REQUIRED
  ];
}

message GetNoticeListResponse {
  int32 total = 1;
  repeated Notice notices = 2;
}

message Notice {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];
  string title = 3 [
    (tagger.tags) = "validate:\"required,gte=1,lte=100\" label:\"标题\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "标题" },
    (google.api.field_behavior) = REQUIRED
  ];
  string content = 4 [
    (tagger.tags) = "validate:\"required\" label:\"内容\"",
    (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = { title: "内容" },
    (google.api.field_behavior) = REQUIRED
  ];
}
```

**步骤 2：生成代码**

```bash
make gen.proto       # 生成 Go 桩代码和 OpenAPI 文档
make gen.go          # 生成 go generate 产物
make gen.wire        # 重新生成 Wire 注入
```

**步骤 3：实现 Controller**

生成后，`api/gen/go/user/v1/` 下会出现 `NoticeService` 的 gRPC server 接口。Controller 层需要实现该接口：

```go
// internal/app/user/controller/notice_grpc.go
package controller

import (
    "context"

    userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
    "github.com/egoadmin/egoadmin/internal/app/user/service"
    "github.com/egoadmin/egoadmin/internal/user/internal/store"
    "github.com/egoadmin/egoadmin/internal/user/internal/auditlog"
    "github.com/jinzhu/copier"
)

type NoticeGRPC struct {
    notice *service.NoticeService
    logger auditlog.Loger
}

func NewNoticeGRPCController(
    notice *service.NoticeService,
    logger auditlog.Loger,
) *NoticeGRPC {
    return &NoticeGRPC{notice: notice, logger: logger}
}

func (s *NoticeGRPC) AddNotice(ctx context.Context,
    in *userv1.AddNoticeRequest) (out *userv1.AddNoticeResponse, err error) {
    out = &userv1.AddNoticeResponse{}

    defer func() {
        if err == nil {
            s.logger.Save(ctx, "系统管理-公告管理", "新增", "新增公告", in)
        }
    }()

    notice := &store.NoticeModel{
        Title:   in.GetNotice().GetTitle(),
        Content: in.GetNotice().GetContent(),
    }

    if err = s.notice.AddNotice(ctx, notice); err != nil {
        return
    }

    out.Id = notice.ID
    return
}

func (s *NoticeGRPC) GetNoticeList(ctx context.Context,
    in *userv1.GetNoticeListRequest) (out *userv1.GetNoticeListResponse, err error) {
    out = &userv1.GetNoticeListResponse{}

    notices, total, err := s.notice.GetNoticeList(ctx, int(in.GetPage()), int(in.GetLimit()))
    if err != nil {
        return
    }

    out.Total = int32(total)
    out.Notices = make([]*userv1.Notice, 0, len(notices))
    if err = copier.Copy(&out.Notices, &notices); err != nil {
        return
    }
    return
}
```

**步骤 4：注册到 Wire 和 Server**

在 `controller.go` 的 `ProviderSet` 中添加 `NewNoticeGRPCController`，在 server 层注册 `NoticeService`，并更新 Wire injector 文件后运行 `make gen.wire`。

**步骤 5：配置权限并更新前端**

对于 gateway 可见的 API，需要在权限系统中注册接口权限条目 `USER.V1.NOTICESERVICE/ADDNOTICE`，并在前端路由和页面中对接。

::: tip
完整的新增 API 流程请参考 [API 开发工作流](/guide/api-development) 页面，该页覆盖了从 Proto 到前端的端到端步骤。
:::

### 内部 API 示例：idgen 服务

内部 API 不需要 HTTP 映射和 OpenAPI 注解，结构更简洁：

```protobuf
// api/proto-internal/idgen/v1/idgen.proto
syntax = "proto3";
package idgen.v1;

import "google/protobuf/timestamp.proto";

// 注意: 无 openapiv2_swagger 选项，无 HTTP 映射
service SegmentService {
  // EnsureSegment 创建号段定义
  rpc EnsureSegment(EnsureSegmentRequest) returns (EnsureSegmentResponse);
  // AllocateSegment 分配不重叠的半开区间 [start, end)
  rpc AllocateSegment(AllocateSegmentRequest) returns (AllocateSegmentResponse);
  // Health 检查分配器是否可用
  rpc Health(SegmentServiceHealthRequest) returns (SegmentServiceHealthResponse);
}

message AllocateSegmentRequest {
  string namespace = 1;   // 命名空间隔离不同部署
  string name = 2;        // 业务 ID 流名称
  int64 requested_step = 3; // 期望的段大小
}

message AllocateSegmentResponse {
  int64 start = 1;   // 分配的起始 ID
  int64 end = 2;     // 分配的结束 ID（不含）
  int64 step = 3;
  int64 min_step = 4;
  int64 max_step = 5;
  int32 status = 6;
}
```

内部 API 与公共 API 的关键区别：

| 维度 | 公共 API (`api/proto/`) | 内部 API (`api/proto-internal/`) |
|------|------------------------|-------------------------------|
| HTTP 映射 | 有，protoc-gen-go-http 兼容 | 无，直接 gRPC 调用 |
| OpenAPI 文档 | 生成 | 不生成 |
| 权限管理 | 参与 Casbin 权限 | 不参与 |
| 生成命令 | `make gen.proto` | `make gen.proto.internal` |
| buf 配置 | `buf.gen.yaml` | `buf.gen.internal.yaml` |

## 工作原理

### protoc-gen-go-http 映射机制

当 Proto 定义了 HTTP 注解后，`protoc-gen-go-http` 插件生成 HTTP 兼容注册代码。Gateway 服务启动时注册这些 handler，将 HTTP POST 请求反序列化为 Proto 消息，直接调用同一 Controller 的 gRPC 方法，再将响应序列化为 JSON 返回。由于 HTTP handler 和 gRPC server 共享同一个 Controller，无需反向代理，无额外延迟。

请求链路：

```text
浏览器/前端
  -> HTTP POST /api/user.v1.RoleService/AddRole
  -> Gateway protoc-gen-go-http handler
  -> 反序列化 JSON -> AddRoleRequest Proto
  -> 直接调用同一 Controller 的 AddRole 方法（无代理）
  -> 收到 AddRoleResponse Proto
  -> 序列化为 JSON 返回
```

### 校验中间件链路

Proto 字段上的 `tagger.tags` 校验标签在代码生成时嵌入 Go 结构体的 struct tag。Controller 层注入的 `*validate.Validate` 中间件在方法入口自动读取 struct tag 并执行校验，校验失败直接返回 gRPC InvalidArgument 错误，不会进入 Service/Domain 层。

## 常见问题

**Q: 可以在 EgoAdmin 中使用 GET 请求获取数据吗？**

不可以。EgoAdmin 统一使用 POST + `body: "*"` 方式，不使用 GET 或路径参数。这与 gRPC 语义一致（所有调用都是方法调用），也简化了鉴权和日志记录。前端所有请求均通过 POST 方法发送 JSON body。

**Q: 新增字段后旧客户端会报错吗？**

Proto3 的默认值设计保证了前向兼容性。新增字段时使用新编号，旧客户端忽略未知字段，新字段在旧客户端以零值呈现。**不要删除或重命名已有字段编号**，否则会破坏反序列化兼容性。

**Q: 为什么每个 RPC 都要单独定义 Request/Response 消息？**

独立消息保证每个方法的接口契约清晰，不会因为共用消息而导致字段膨胀或方法间隐式耦合。当一个方法需要增加参数时，不会影响其他方法的签名。

**Q: OpenAPI 的 security 块和 description 中的权限说明有什么关系？**

`security` 块控制 Swagger UI 是否显示认证输入框。`description` 中的权限说明是给人看的文档信息。两者必须同步：受保护接口要同时添加 `security` 块（让 Swagger UI 正确展示认证）和 description 中的权限描述（让开发者知道具体需要什么权限）。

**Q: Proto 文件名有什么命名规则？**

- 文件名使用小写蛇形：`role.proto`、`auth_internal.proto`
- 公共 API 文件放在 `api/proto/<service>/v1/` 下
- 每个资源一个文件，以资源名单数命名
- 内部 API 文件放在 `api/proto-internal/<service>/v1/` 下，可用 `_internal` 后缀区分

**Q: 如何处理 Proto 字段到 Go 结构体字段名不一致的情况？**

使用 `copier` 标签。例如 Proto 字段 `created_at`（蛇形）在 Go 结构体中叫 `CreatedAtToRPC`（PascalCase），在 Proto 中标注 `[(tagger.tags) = "copier:\"CreatedAtToRPC\""]` 即可。Controller 层的 `copier.Copy` 会根据标签自动完成映射。

**Q: 可以在 Proto 中定义 Streaming RPC 吗？**

当前 EgoAdmin 不使用 Streaming RPC。所有业务 API 都是 Unary RPC（一问一答模式）。如果未来需要文件上传或实时推送，应评估是否走 TUS 协议或 WebSocket 独立通道，而非在现有 Proto 服务中添加 stream 方法。

## 参考链接

- `api/proto/user/v1/user.proto` -- 用户服务完整定义
- `api/proto/user/v1/role.proto` -- 角色服务完整定义
- `api/proto/user/v1/errors.proto` -- 错误码定义
- `api/proto-internal/idgen/v1/idgen.proto` -- 内部 API 示例
- `api/proto-internal/user/v1/auth_internal.proto` -- 内部认证示例
- `internal/app/user/controller/role_grpc.go` -- Controller 实现范例
- `internal/app/user/controller/controller.go` -- Wire ProviderSet 注册
- `scripts/make/proto.mk` -- Proto 生成 Makefile 目标
- `buf.gen.yaml` -- 公共 Proto 生成配置
- `buf.gen.internal.yaml` -- 内部 Proto 生成配置
- `buf.work.yaml` -- Buf workspace 配置
