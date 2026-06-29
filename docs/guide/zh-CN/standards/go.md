# Go 语言编码规范

本文档定义 EgoAdmin 项目中 Go 语言的编码规范，涵盖命名、错误处理、函数签名、依赖注入、事务、并发、数据模型、Lint 配置及常见反模式。所有示例均来自项目真实代码。

## 概述

EgoAdmin 后端采用 DDD 分层架构，代码分布在 `cmd/<service>`、`internal/app/<service>`、`internal/platform` 和 `internal/component` 之间。为保持一致性和可维护性，所有服务的 Go 代码遵循统一规范。

本规范从命名约定开始，逐层展开到错误处理、函数设计、依赖注入、事务模式、并发控制、数据模型和 Lint 工具链。每一节都配有从项目源码中提取的真实示例，可直接作为参考。

::: tip 阅读建议
如果你是新加入项目的开发者，建议从"命名约定"和"错误处理"两节开始；如果你在写 Service 层代码，重点阅读"事务模式"和"并发与锁"。
:::

## 核心用法

### 1. 命名约定

#### 包名

包名必须简短、小写、与服务职责匹配。避免使用下划线、大写字母或过于通用的名称（如 `utils`、`common`）。

```go
// 正确：简短、语义清晰
package store        // 持久层
package controller   // gRPC 控制器
package service      // 服务层
package application  // 应用层（用例）
package user         // 用户领域

// 错误：冗长或通用
package user_store_helpers   // 不要用下划线
package common               // 语义不明确
```

项目中的实际包名示例：

| 层级 | 路径 | 包名 |
|------|------|------|
| Controller | `internal/app/user/controller` | `controller` |
| Service | `internal/app/user/service` | `service` |
| Application | `internal/app/user/application` | `application` |
| Domain | `internal/app/user/domain/user` | `user` |
| Store | `internal/app/user/internal/store` | `store` |

#### 导出与非导出标识符

导出使用 PascalCase，非导出使用 camelCase。

```go
// 导出：供其他包使用的类型、函数、方法
type UserGRPC struct { ... }
func NewUserGRPCController(...) *UserGRPC { ... }
func (s *UserGRPC) AddUser(ctx context.Context, ...) error { ... }

// 非导出：包内部使用
type Options struct { ... }
func mapUserDomainError(ctx context.Context, err error) error { ... }
func userRolesFromIDs(ids []uint64) []RoleModel { ... }
```

#### 常量命名

项目采用混合风格：极少数全局性标志使用 ALL_CAPS，其余大多数使用 PascalCase 前缀 + 描述性名称。

```go
// iota 常量：PascalCase 前缀 + 描述
const (
    UserModelBuiltIn     int32 = iota + 1  // 内置用户
    UserModelNonBuiltIn                     // 普通用户
)

const (
    UserModelStatusValid   = iota + 1  // 用户有效
    UserModelStatusInvalid              // 用户无效
)

// 特殊值常量：直接 PascalCase
const UserModelUsernameRoot  = "root"
const UserModelUsernameAdmin = "admin"
```

#### 接口命名

接口优先使用描述性名词或 `-er` 后缀。项目中 store 层的接口以 `Interface` 后缀为主：

```go
// store 层接口：XxxInterface
type UserInterface interface {
    Add(ctx context.Context, user *UserModel) error
    Get(ctx context.Context, id uint64) (*UserModel, error)
    Delete(ctx context.Context, ids []uint64) error
    // ...
}

type RoleInterface interface { ... }
type DeptInterface interface { ... }

// 基础设施接口：XxxInterface
type MysqlInterface interface {
    Transaction(ctx context.Context, callback func(context.Context) error) error
    WithTx(ctx context.Context) *gorm.DB
    Migrate(ctx context.Context, models []any, joinTables []MigrationJoinTable) error
}

// 组件接口
type authsession.Interface interface { ... }
type LoginCryptoInterface interface { ... }
```

#### 布尔字段

布尔类型字段使用 `Is`、`Has`、`Enable` 前缀，增强可读性。

```go
// 配置结构体中的布尔字段
type LoginCryptoConfig struct {
    EnableMetrics     bool   `toml:"enableMetrics"`
    ChallengeTTL      string `toml:"challengeTTL"`
}

type UserConf struct {
    MultiLoginEnabled          bool `toml:"multiLoginEnabled"`
    HeartbeatOfflineEnabled    bool `toml:"heartbeatOfflineEnabled"`
    RevokeSessionOnHeartbeatOffline bool `toml:"revokeSessionOnHeartbeatOffline"`
}

// 模型常量（以数值表示布尔语义）
const (
    UserModelBuiltIn     int32 = 1
    UserModelNonBuiltIn  int32 = 2
)
```

### 2. 错误处理

#### 基本原则

永远不要用 `_` 忽略错误。每个错误都必须显式检查和处理。

```go
// 正确：显式检查
user, err := s.User.Get(ctx, id)
if err != nil {
    return nil, err
}

// 错误：忽略错误
user, _ := s.User.Get(ctx, id)  // 禁止！
```

#### 领域错误定义

各层定义自己的哨兵错误变量，使用 `errors.New` 并带上 `包名: 描述` 格式。

```go
// domain 层：业务规则校验错误
// internal/app/user/domain/user/error.go
package user

import "errors"

var (
    ErrNotFound             = errors.New("user: not found")
    ErrInvalidDefaultPasswd = errors.New("user: invalid default password source")
    ErrBuiltinPasswordReset = errors.New("user: builtin password reset denied")
    ErrBuiltinUsername      = errors.New("user: builtin username denied")
    ErrUsernameExists       = errors.New("user: username exists")
    ErrPhoneExists          = errors.New("user: phone exists")
    ErrLoginDenied          = errors.New("user: login denied")
)

// application 层：用例级业务错误
// internal/app/user/application/error.go
package application

import "errors"

var (
    ErrSubmittedPassword = errors.New("user application: submitted password denied")
    ErrLoginUARequired   = errors.New("user application: login user agent required")
)
```

#### 错误分类：errors.Is / errors.As

使用 `errors.Is` 做哨兵错误比较，使用 `errors.As` 做类型断言。禁止直接用 `==` 比较包装后的错误。

```go
// 正确：errors.Is
case errors.Is(err, userdomain.ErrBuiltinUsername):
    return platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil)

// 错误：直接比较（被 %w 包装后会失败）
case err == userdomain.ErrBuiltinUsername:  // 禁止！
```

#### i18n 错误包装

Controller 层统一使用 `platformi18n.ErrorFailed` 将领域错误映射为国际化错误码，前端根据错误码展示对应语言的提示。

```go
// internal/app/user/controller/user_grpc.go
func mapUserDomainError(ctx context.Context, err error) error {
    switch {
    case err == nil:
        return nil
    case errors.Is(err, application.ErrSubmittedPassword):
        return platformi18n.ErrorFailed(ctx, "SubmittedPasswordUnsupported", nil)
    case errors.Is(err, userdomain.ErrBuiltinUsername):
        return platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil)
    case errors.Is(err, userdomain.ErrUsernameExists):
        return platformi18n.ErrorFailed(ctx, "UsernameExists", nil)
    case errors.Is(err, userdomain.ErrPhoneExists):
        return platformi18n.ErrorFailed(ctx, "PhoneExists", nil)
    case errors.Is(err, application.ErrLoginUARequired):
        return platformi18n.ErrorFailed(ctx, "LoginInfoAbnormal", nil)
    case errors.Is(err, userdomain.ErrLoginDenied):
        return platformi18n.ErrorFailed(ctx, "LoginFailed", nil)
    case authsession.IsSessionError(err):
        return authsession.ToEcodeContext(ctx, err)
    case errors.Is(err, userdomain.ErrBuiltinPasswordReset):
        return platformi18n.ErrorFailed(ctx, "BuiltinPasswordResetDenied", nil)
    case errors.Is(err, userdomain.ErrInvalidDefaultPasswd):
        return platformi18n.ErrorFailed(ctx, "DefaultPasswordPhoneTooShort", nil)
    default:
        return err  // 未知错误原样返回，交给框架兜底
    }
}
```

::: warning 注意
`mapXxxDomainError` 函数放在 Controller 层（不是 Service 层或 Domain 层），因为国际化错误码属于接口契约，不应侵入业务逻辑。
:::

### 3. 函数设计

#### Controller 方法签名

所有 gRPC Controller 方法遵循统一签名：接收 `context.Context` 和 proto 请求，返回 proto 响应和 error。使用命名返回值。

```go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    // 1. 初始化 out，防止 panic
    out = &userv1.AddUserResponse{}

    // 2. 参数校验
    if in.GetUser().GetPassword() != "" {
        err = mapUserDomainError(ctx, application.ErrSubmittedPassword)
        return
    }

    // 3. 调用 Service 层
    err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
    if err != nil {
        return
    }

    // 4. 填充响应
    out.Id = user.ID
    return
}
```

关键规则：

- **必须**在方法开头初始化 `out`，避免 nil pointer panic
- 使用 `in.GetXxx()` 访问 proto 字段（自动处理零值和 nil）
- Controller 层只做参数转换和错误映射，**不写业务逻辑**
- 使用命名返回值 + naked return，保持简洁

#### 延迟审计日志

写操作使用 `defer` 记录审计日志，仅在成功时记录：

```go
func (s *UserGRPC) DeleteUser(ctx context.Context, in *userv1.DeleteUserRequest) (out *userv1.DeleteUserResponse, err error) {
    out = &userv1.DeleteUserResponse{}

    // 成功时记录审计日志
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "删除", "删除用户", in)
        }
    }()

    err = s.user.DeleteUser(ctx, in.GetIds())
    return
}
```

::: tip 为什么只在成功时记录？
失败的操作不应写入审计日志，避免产生误导性的操作记录。如果需要记录失败尝试，应使用独立的失败日志表。
:::

### 4. 依赖注入（Wire）

#### ProviderSet 声明

每个包声明自己的 `ProviderSet`，列出本包提供的构造函数和绑定关系。

```go
// internal/app/user/controller/controller.go
package controller

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
    NewInternalAuthGRPCController,
    NewLogGRPCController,
    NewUserGRPCController,
    NewRoleGRPCController,
    NewDeptGRPCController,
    NewCenterGRPCController,
)

// internal/app/user/service/service.go
var ProviderSet = wire.NewSet(
    NewLogService,
    NewUserService,
    NewRoleService,
    NewDeptService,
    NewConfigService,
    NewAuthSession,
    NewLoginCrypto,
    wire.Struct(new(Options), "*"),  // 使用通配符注入所有字段
    wire.Bind(new(authsession.Interface), new(*authsession.Component)),
    wire.Bind(new(LoginCryptoInterface), new(*logincrypto.Component)),
    redis.CoreProviderSet,
    store.ProviderSet,
    // ...
)
```

#### Wire Injector 文件

每个服务的 `server/wire.go` 使用 `//go:build wireinject` 构建标签，声明编译时依赖图。

```go
// internal/app/user/server/wire.go
//go:build wireinject
// +build wireinject

package server

import (
    usercache "github.com/egoadmin/egoadmin/internal/app/user/adapter/cache"
    userpermission "github.com/egoadmin/egoadmin/internal/app/user/adapter/permission"
    usermysql "github.com/egoadmin/egoadmin/internal/app/user/adapter/persistence/mysql"
    "github.com/egoadmin/egoadmin/internal/app/user/application"
    "github.com/egoadmin/egoadmin/internal/app/user/controller"
    "github.com/egoadmin/egoadmin/internal/app/user/service"
    "github.com/google/wire"
)

func NewApp() (*App, error) {
    panic(wire.Build(
        newEgo,
        newConfig,
        newHealth,
        application.ProviderSet,
        usermysql.ProviderSet,
        usercache.ProviderSet,
        userpermission.ProviderSet,
        wire.Bind(new(application.UserLocks), new(*usercache.UserLocks)),
        wire.Bind(new(application.RoleLocks), new(*usercache.UserLocks)),
        wire.Bind(new(application.DeptLocks), new(*usercache.UserLocks)),
        wire.Bind(new(application.RoleAssignments), new(*usermysql.UserRepository)),
        controller.ProviderSet,
        service.ProviderSet,
        newApp,
    ))
}
```

::: danger 禁止手动编辑 wire_gen.go
`wire_gen.go` 是 Wire 工具自动生成的文件。修改依赖关系时只改 `wire.go`，然后运行 `make gen` 重新生成。
:::

#### Wire 绑定规则

- 接口绑定使用 `wire.Bind(new(Interface), new(*Concrete))`
- 选项结构体注入使用 `wire.Struct(new(Options), "*")`，通配符表示注入所有字段
- 每个 `adapter/` 子包（如 `persistence/mysql`、`cache`）有自己的 `ProviderSet`

### 5. 事务模式

#### Platform 层 Transaction API

项目提供两种事务 API：`MysqlInterface.Transaction`（面向 Service 层）和底层 `mysql.Transaction`（面向 Store 内部）。

```go
// internal/platform/database/mysql/interface_mysql.go
type MysqlInterface interface {
    Transaction(ctx context.Context, callback func(context.Context) error) error
    WithTx(ctx context.Context) *gorm.DB
}
```

#### Service 层事务用法

Service 层通过 `s.Mysql.Transaction` 开启事务，回调内所有数据库操作自动共享同一事务上下文。

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // ... 权限校验 ...

    if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
        // 步骤 1：业务检查（在事务内）
        if er := s.addCheck(txCtx, user); er != nil {
            return er
        }
        // 步骤 2：生成默认密码
        defaultPassword, er := userdomain.DefaultPasswordFromPhone(stringValue(user.Phone))
        if er != nil {
            return er
        }
        // 步骤 3：哈希密码
        hashPass, er := xcrypt.HashAndSalt(defaultPassword)
        if er != nil {
            return er
        }
        user.Password = hashPass
        // 步骤 4：持久化（在事务内）
        return s.User.Add(txCtx, user)
    }); err != nil {
        return
    }

    // 事务提交后的操作（如重新加载缓存）
    err = s.reloadCasbinRolesForUser(ctx, user.ID)
    return
}
```

::: warning 事务内操作要谨慎
事务回调中应只包含必须原子执行的写操作。查询和缓存操作尽量放在事务外，减少锁持有时间。
:::

#### Store 层 WithTx

Store 层通过 `r.db.WithTx(ctx)` 自动获取上下文中的事务对象。如果外层没有事务，使用正常 DB 连接。

```go
// Store 内部的事务感知查询
func (m *UserRepository) Get(ctx context.Context, id uint64) (*UserModel, error) {
    db := m.db.WithTx(ctx)  // 自动使用外层事务
    var user UserModel
    err = db.Preload("Roles").Preload("Roles.Policies").Model(&UserModel{}).First(&user, id).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}
```

### 6. 并发与锁

#### Redis 分布式锁

对并发写操作（新增、修改、删除用户），使用 Redis 分布式锁确保互斥。

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // 获取"新增"锁
    addlock := s.UserRedis.LockAdd()
    if err = addlock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() {
        _ = addlock.Unlock(ctx)
    }()

    // 获取"修改"锁
    lock := s.UserRedis.LockUpdate()
    if err = lock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() {
        _ = lock.Unlock(ctx)
    }()

    // ... 执行业务逻辑 ...
}
```

锁的使用规则：

- **必须**使用 `defer` 释放锁，防止异常路径死锁
- 获取锁失败直接返回错误，不重试
- 锁粒度按操作类型区分：`LockAdd()`、`LockUpdate()` 等
- 锁等待超时统一使用 `time.Second * 5`

#### Context 与取消传播

所有函数第一个参数必须是 `context.Context`，用于传递取消信号和截止时间。

```go
// 正确：传递 context
func (s *UserService) GetUser(ctx context.Context, id uint64) (*UserModel, error) {
    return s.User.Get(ctx, id)  // ctx 向下传递
}

// 错误：忽略 context
func (s *UserService) GetUser(id uint64) (*UserModel, error) {  // 禁止！
    return s.User.Get(context.Background(), id)
}
```

### 7. GORM 模型模式

#### 模型定义

所有数据模型嵌入 `xorm.Model`，自动获得 `ID`、`CreatedAt`、`UpdatedAt`、`DeletedAt` 字段。

```go
// internal/app/user/internal/store/user.go
type UserModel struct {
    xorm.Model
    BuiltIn       int32      `gorm:"int(10);not null;default:2;comment:是否内置用户,1内置用户,2普通用户"`
    Username      string     `gorm:"uniqueIndex;type:varchar(255);not null;comment:用户名"`
    Password      string     `gorm:"type:varchar(255);not null;comment:用户密码"`
    Name          string     `gorm:"type:varchar(255);not null;default:'';comment:姓名"`
    Phone         *string    `gorm:"uniqueIndex;type:varchar(255);comment:手机号"`
    Gender        int8       `gorm:"type:tinyint(2);comment:性别,0:保密,1:男,2:女"`
    UserStatus    int32      `gorm:"type:int(10);not null;default:1;comment:用户状态,1有效,2无效"`
    DeptID        uint64     `gorm:"index:idx_user_deptid,priority:1;type:bigint(20) unsigned;not null;default:0;comment:组织id"`
    Roles         []RoleModel `gorm:"many2many:user_role"` // 多对多关联
}
```

GORM 标签规范：

| 标签 | 用途 | 示例 |
|------|------|------|
| `type` | 列数据类型 | `type:varchar(255)` |
| `not null` | 非空约束 | `not null` |
| `default` | 默认值 | `default:2` |
| `index` | 索引 | `index:idx_user_deptid,priority:1` |
| `uniqueIndex` | 唯一索引 | `uniqueIndex` |
| `comment` | 列注释 | `comment:用户名` |
| `many2many` | 多对多关联表 | `many2many:user_role` |
| `foreignKey` | 外键字段 | `foreignKey:RoleModelID` |
| `references` | 引用字段 | `references:ID` |

#### Repository 模式

持久层采用接口 + 实现的 Repository 模式。接口定义在 `store` 包，实现在 `adapter/persistence/mysql` 包。

```go
// 接口定义（internal/app/user/internal/store/interface_user.go）
type UserInterface interface {
    Add(ctx context.Context, user *UserModel) error
    Get(ctx context.Context, id uint64) (*UserModel, error)
    Delete(ctx context.Context, ids []uint64) error
    Update(ctx context.Context, id uint64, user *UserModel) error
    GetList(ctx context.Context, opt UserModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) ([]*UserModel, int64, error)
    GetByIds(ctx context.Context, ids []uint64) ([]*UserModel, error)
    // ...
}
```

#### Preload 关联查询

使用 `Preload` 加载关联数据，通过 `Association` 管理多对多关系。

```go
// 查询用户时预加载角色和权限策略
err = db.Preload("Roles").Preload("Roles.Policies").
    Model(&UserModel{}).First(&user, id).Error

// 替换用户的角色关联
if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).
    Association("Roles").Replace(roles); er != nil {
    return er
}
```

### 8. Lint 配置

项目使用 golangci-lint 进行代码质量检查。运行 `make lint` 执行检查。

启用的 Linter 列表：

| Linter | 作用 |
|--------|------|
| `errcheck` | 检查未处理的 error 返回值 |
| `govet` | 运行 `go vet` 检查可疑代码 |
| `ineffassign` | 检查无效赋值 |
| `staticcheck` | 高级静态分析 |
| `typecheck` | 类型检查 |
| `unused` | 检查未使用的代码 |
| `bodyclose` | 检查 HTTP Body 是否关闭 |
| `goconst` | 检查可提取为常量的重复字符串 |
| `gofmt` | 格式化检查 |
| `goimports` | import 排序检查 |
| `gosec` | 安全检查 |
| `misspell` | 拼写检查 |
| `prealloc` | slice 预分配建议 |
| `unconvert` | 检查不必要的类型转换 |

排除规则：

```yaml
# .golangci.yml
issues:
  exclude-dirs:
    - web/         # 前端代码
  exclude-files:
    - ".*pb.go"    # protobuf 生成文件

linters-settings:
  gosec:
    excludes:
      - G115  # 整数溢出转换，在业务上下文中是安全的
```

::: tip nolint 注释
当 Linter 报告的警告经确认是合理的时候，使用 `//nolint:linter_name // 原因` 注释抑制。**必须附带原因说明**。

```go
//nolint:gosec // total is checked above and fits int32.
out.Total = int32(total)
```
:::

### 9. Copier 与响应转换

Controller 层使用 `github.com/jinzhu/copier` 将 store 层模型转换为 proto 响应消息。

```go
// 简单字段映射
user, err := s.user.GetUser(ctx, in.GetId())
if err != nil {
    return
}
err = copier.Copy(&out.User, &user)

// 列表映射
users, total, err := s.user.GetUserListNameOrUserName(ctx, opt, in.GetName())
if err != nil {
    return
}
err = copier.Copy(&out.Users, &users)
```

Proto 字段名与模型字段名不同时，通过 `copier` tag 映射：

```protobuf
// api/proto/user/v1/role.proto
message Role {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];
  google.protobuf.Timestamp updated_at = 3 [(tagger.tags) = "copier:\"UpdatedAtToRPC\""];
}
```

::: warning copier 限制
copier 只做浅拷贝，不会递归复制嵌套结构。复杂映射或需要自定义转换逻辑时，手动编写转换函数。
:::

## 配置示例

以下为与 Go 服务相关的运行时配置片段。

### 用户服务配置

```toml
# configs/user/config.toml
[app.user]
jwtExpire = 604800                # access token 有效期（秒）
refreshTokenExpire = 2592000      # refresh token 有效期（秒）
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true          # 是否允许多端登录
maxLoginClient = 2                # 最大登录客户端数
heartbeatOfflineEnabled = true    # 心跳超时自动离线
heartbeatOfflineSeconds = 660     # 心跳超时时间（秒）
```

### 登录加密组件配置

```toml
[component.logincrypto]
challengeTTL = "3m0s"         # 挑战有效期
timestampSkew = "2m0s"        # 时间戳容差
rsaKeyBits = 4096             # RSA 密钥长度
enableMetrics = true          # 是否启用指标采集
```

### 环境变量覆盖

所有 TOML 配置均可通过 `EGOADMIN_*` 前缀的环境变量覆盖：

```bash
# 覆盖 JWT 签名密钥
EGOADMIN_APP_USER_JWTSIGNKEY=production-secret-key

# 覆盖多端登录配置
EGOADMIN_APP_USER_MULTILOGINENABLED=false

# 覆盖心跳超时
EGOADMIN_APP_USER_HEARTBEATOFFLINESECONDS=300
```

| 配置路径 | 环境变量 | 说明 |
|----------|----------|------|
| `app.user.jwtSignKey` | `EGOADMIN_APP_USER_JWTSIGNKEY` | JWT 签名密钥 |
| `app.user.multiLoginEnabled` | `EGOADMIN_APP_USER_MULTILOGINENABLED` | 多端登录开关 |
| `app.user.heartbeatOfflineEnabled` | `EGOADMIN_APP_USER_HEARTBEATOFFLINEENABLED` | 心跳离线开关 |
| `component.logincrypto.enableMetrics` | `EGOADMIN_COMPONENT_LOGINCRYPTO_ENABLEMETRICS` | 加密指标开关 |

## 实战示例

### 完整的 CRUD Controller 流程

以下展示从 Controller 到 Store 的完整新增用户流程。

**第一步：Controller 接收请求并转换参数**

```go
// internal/app/user/controller/user_grpc.go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    out = &userv1.AddUserResponse{}

    // 审计日志：成功时记录
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
        }
    }()

    // 校验：不允许前端提交密码
    if in.GetUser().GetPassword() != "" {
        err = mapUserDomainError(ctx, application.ErrSubmittedPassword)
        return
    }

    // 转换 gender 枚举
    gender, ok := store.UserModelGenderToInt8(in.GetUser().GetGender())
    if !ok {
        err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
        return
    }

    // 构建 store model
    user := &store.UserModel{
        Username:   in.GetUser().GetUsername(),
        Name:       in.GetUser().GetName(),
        Phone:      lo.ToPtr(in.GetUser().GetPhone()),
        Gender:     gender,
        UserStatus: in.GetUser().GetUserStatus(),
        DeptID:     in.GetUser().GetDeptId(),
        Remark:     in.GetUser().GetRemark(),
        Roles:      userRolesFromIDs(in.GetRoleIds()),
    }

    // 调用 Service 层并映射错误
    err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
    if err != nil {
        return
    }
    out.Id = user.ID
    return
}
```

**第二步：Service 层编排锁、事务和业务规则**

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // 数据权限校验
    scope, err := s.resolveDataScope(ctx)
    if err != nil {
        return err
    }
    if user.DeptID != 0 {
        if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
            return err
        }
    }
    if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
        return err
    }
    user.UserType = store.UserModelTypePlatform

    // 获取分布式锁
    addlock := s.UserRedis.LockAdd()
    if err = addlock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() { _ = addlock.Unlock(ctx) }()

    lock := s.UserRedis.LockUpdate()
    if err = lock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() { _ = lock.Unlock(ctx) }()

    // 事务：检查 + 生成密码 + 持久化
    if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
        if er := s.addCheck(txCtx, user); er != nil {
            return er
        }
        defaultPassword, er := userdomain.DefaultPasswordFromPhone(stringValue(user.Phone))
        if er != nil {
            return er
        }
        hashPass, er := xcrypt.HashAndSalt(defaultPassword)
        if er != nil {
            return er
        }
        user.Password = hashPass
        return s.User.Add(txCtx, user)
    }); err != nil {
        return
    }

    // 事务外：重新加载权限缓存
    err = s.reloadCasbinRolesForUser(ctx, user.ID)
    return
}
```

**第三步：Store 层执行数据库操作**

```go
// Store 层通过 Preload 管理关联，通过 WithTx 感知事务
func (m *UserRepository) Add(ctx context.Context, user *UserModel) error {
    return m.db.WithTx(ctx).Create(user).Error
}
```

::: warning 分层职责
Controller 负责参数转换和错误映射；Service 负责业务编排（锁、事务、权限校验）；Store 只做数据库操作。**不要在 Controller 中写业务逻辑**。
:::

## 常见问题

### Q1: 为什么 Controller 方法返回 `(out *pb.XxxResponse, err error)` 而不是 `(*pb.XxxResponse, error)`？

命名返回值配合 naked return，使代码更简洁。`out` 在方法开头初始化后，后续代码只需 `return`，不需要重复写返回值。同时 Wire 和 gRPC 框架要求特定的返回签名。

### Q2: 什么时候用 `platformi18n.ErrorFailed`，什么时候直接返回 `err`？

`platformi18n.ErrorFailed` 用于需要前端展示用户友好提示的业务错误（如"用户名已存在"）。底层系统错误（如数据库连接失败）直接返回原 `err`，由框架的全局错误处理器统一处理。

### Q3: 为什么 Lock 的 defer 中用 `_ = lock.Unlock(ctx)` 忽略 Unlock 错误？

锁释放失败通常意味着 Redis 连接已断开，此时无法做有意义的恢复。忽略解锁错误避免掩盖业务逻辑中的真实错误。锁有过期时间（TTL），即使 Unlock 失败也不会永久死锁。

### Q4: Store 层的 `UserInterface` 和 Service 层的 `UserService` 有什么区别？

`UserInterface` 是存储接口，定义数据库 CRUD 操作；`UserService` 是业务服务，编排事务、锁、权限校验、缓存等。Service 依赖 Store 接口，但 Store 不知道 Service 的存在。依赖方向：Controller -> Service -> Store。

### Q5: Proto 文件中 `copier:"ID"` tag 的作用是什么？

proto 生成的 Go 结构体字段名是 `Id`，但 store model 的字段名是 `ID`。copier tag 告诉 copier 在映射时将 `Id` 映射到 `ID`，避免字段名不匹配导致的映射失败。

### Q6: `//nolint:gosec` 会不会掩盖真正的安全问题？

不会。项目中 `nolint:gosec` 仅用于 G115（整数溢出转换）这一个规则，且必须附带原因说明。`gosec` 的其他安全规则（如 SQL 注入、硬编码密钥）仍然生效。如果不确定，不要加 nolint，让 CI 拦截后人工审查。

### Q7: 能否在 Domain 层直接使用 platformi18n 包？

不能。Domain 层是纯业务规则层，不应依赖任何基础设施包（包括 i18n）。Domain 层只定义哨兵错误变量，i18n 映射在 Controller 层完成。这是 DDD 分层的核心约束。

## 参考链接

- 命名约定源码: `internal/app/user/internal/store/interface_user.go`
- 错误处理: `internal/app/user/domain/user/error.go`, `internal/app/user/application/error.go`
- Controller 示例: `internal/app/user/controller/user_grpc.go`, `internal/app/user/controller/role_grpc.go`
- Service 示例: `internal/app/user/service/user_service.go`, `internal/app/user/service/service.go`
- Wire 配置: `internal/app/user/server/wire.go`, `internal/app/user/controller/controller.go`
- Store 模型: `internal/app/user/internal/store/user.go`, `internal/app/user/internal/store/role.go`
- MysqlInterface: `internal/platform/database/mysql/interface_mysql.go`
- Lint 配置: `.golangci.yml`
