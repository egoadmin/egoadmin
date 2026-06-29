# GORM 模型与仓储层

EgoAdmin 使用 GORM 定义持久化模型，通过 DDD 仓储模式将领域对象与数据库记录解耦。本文介绍模型定义、仓储实现、关联处理和 Wire 绑定的完整流程。

## 概览

EgoAdmin 项目中存在两条持久化路径：

| 路径 | 位置 | 说明 |
|------|------|------|
| **DDD 目标** | `internal/app/<service>/adapter/persistence/mysql/` | 新代码应写在这里，遵循领域驱动设计 |
| **迁移过渡期** | `internal/app/<service>/internal/store/` | 已有代码，逐步迁移到 DDD 结构 |

核心数据流向：

```
Domain Aggregate (领域聚合根)
       |                    ^
       v                    |
   Model (GORM 模型)   toDomain() / fromDomain()
       |
       v
   MySQL 表
```

领域层定义 `Repository` 接口，持久化层提供 `MySQL` 实现。模型和领域对象之间通过 `toDomain()` / `fromDomain()` 函数双向转换，确保领域层不依赖 GORM。

## 核心用法

### 1. 定义 GORM 模型

模型放在 `internal/app/<service>/adapter/persistence/mysql/` 目录下，嵌入 `xorm.Model` 获得 ID 和时间戳字段。

```go
// internal/app/user/adapter/persistence/mysql/user_model.go
package mysql

import (
    "time"

    userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
    platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "gorm.io/gorm"
)

type userModel struct {
    xorm.Model
    Username      string
    Password      string
    Name          string
    Phone         *string
    Gender        int8
    UserStatus    int32
    UserType      int32
    UserOnline    int32
    Remark        string
    DeptID        uint64
    Roles         []roleModel `gorm:"many2many:user_role"`
    HeartbeatTime time.Time
}

func (userModel) TableName() string {
    return "user"
}

func (m *userModel) SetID(id uint64) {
    if m.ID == 0 {
        m.ID = id
    }
}

func (m *userModel) BeforeCreate(tx *gorm.DB) error {
    return platformmysql.SetID(m)
}
```

`xorm.Model` 提供以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `uint64` | 雪花 ID 主键 |
| `CreatedAt` | `time.Time` | 创建时间 |
| `UpdatedAt` | `time.Time` | 更新时间 |

`BeforeCreate` 钩子调用 `platformmysql.SetID(m)`，由全局雪花发号器自动分配 ID。每个模型都需要实现 `SetID(id uint64)` 方法和 `TableName() string` 方法。

::: tip
模型字段名和数据库列名之间的映射由 GORM 的 `NamingStrategy` 自动处理（驼峰转下划线）。`SingularTable: true` 确保表名不加复数后缀。如需自定义列名，使用 `gorm:"column:xxx"` 标签。
:::

### 2. 定义领域聚合根

领域对象放在 `internal/app/<service>/domain/<aggregate>/` 目录下，是纯 Go 结构体，不依赖任何框架。

```go
// internal/app/user/domain/user/user.go
package user

import "time"

type User struct {
    ID            uint64
    Username      string
    PasswordHash  string
    Name          string
    Phone         string
    Gender        Gender
    Status        Status
    Type          int32
    OnlineStatus  OnlineStatus
    DeptID        uint64
    Remark        string
    RoleIDs       []uint64
    RoleMenus     []string
    HeartbeatTime time.Time
}

type Gender int32

const (
    GenderHidden Gender = iota
    GenderMale
    GenderFemale
)

type Status int32

const (
    StatusUnknown Status = iota
    StatusValid
    StatusInvalid
)
```

领域错误统一在 `error.go` 中定义：

```go
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
```

::: warning
领域错误使用 `errors.New` 定义为哨兵错误，不要使用 `fmt.Errorf`。调用方通过 `errors.Is(err, userdomain.ErrNotFound)` 进行判断。
:::

### 3. 定义 Repository 接口

仓储接口在领域层定义，描述持久化能力，不暴露 GORM 细节。

```go
// internal/app/user/domain/user/repository.go
package user

import (
    "context"
    "time"
)

// Repository persists user aggregates.
type Repository interface {
    NextID(ctx context.Context) (uint64, error)
    Create(ctx context.Context, aggregate *User) error
    Save(ctx context.Context, aggregate *User) error
    Update(ctx context.Context, id uint64, aggregate *User) error
    FindByID(ctx context.Context, id uint64) (*User, error)
    FindByUsername(ctx context.Context, username string) (*User, error)
    FindByPhone(ctx context.Context, phone string) (*User, error)
    List(ctx context.Context, query ListQuery) ([]*User, int64, error)
    UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
    MarkLoggedIn(ctx context.Context, id uint64, at time.Time, ip string) error
    MarkOnline(ctx context.Context, id uint64, at time.Time) error
    MarkOffline(ctx context.Context, ids []uint64) error
    FindHeartbeatExpiredIDs(ctx context.Context, before time.Time) ([]uint64, error)
    CountOnline(ctx context.Context) (int64, error)
    Delete(ctx context.Context, ids []uint64) error
}

type ListQuery struct {
    Page     int
    Limit    int
    Sort     string
    Order    string
    Name     string
    Phone    string
    RoleID   uint64
    DeptIDs  []uint64
    Status   Status
    Username string
}
```

### 4. 实现 Repository

仓储实现使用 `platformmysql.MysqlInterface` 获取 GORM `*gorm.DB`，通过 `var _ interface = (*Impl)(nil)` 确保编译期接口检查。

```go
// internal/app/user/adapter/persistence/mysql/user_repository.go
package mysql

import (
    "context"
    "errors"

    userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
    platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
    "github.com/egoadmin/elib/pkg/util/xflake"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "gorm.io/gorm"
)

// UserRepository implements user.Repository with MySQL.
type UserRepository struct {
    db    platformmysql.MysqlInterface
    idgen xflake.Geter
}

var _ userdomain.Repository = (*UserRepository)(nil)

// NewUserRepository creates a MySQL-backed user repository.
func NewUserRepository(db platformmysql.MysqlInterface, idgen xflake.Geter) *UserRepository {
    return &UserRepository{
        db:    db,
        idgen: idgen,
    }
}

func (r *UserRepository) NextID(ctx context.Context) (uint64, error) {
    id, err := r.idgen.Get()
    if err != nil {
        return 0, err
    }
    return id, nil
}

func (r *UserRepository) Create(ctx context.Context, aggregate *userdomain.User) error {
    model := userModelFromDomain(aggregate)
    if model == nil {
        return userdomain.ErrNotFound
    }
    if err := r.db.WithTx(ctx).Omit("Roles.*").Create(model).Error; err != nil {
        return err
    }
    aggregate.ID = model.ID
    return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*userdomain.User, error) {
    model := &userModel{}
    err := r.db.WithTx(ctx).Preload("Roles").First(model, id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, userdomain.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return model.toDomain(), nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*userdomain.User, error) {
    model := &userModel{}
    err := r.db.WithTx(ctx).Preload("Roles").Where(&userModel{Username: username}).First(model).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, userdomain.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return model.toDomain(), nil
}
```

关键要点：

- **`r.db.WithTx(ctx)`**：从 context 获取当前事务，如果没有事务则使用普通连接。所有读写都必须通过此方法，以确保同一请求内的操作共享事务。
- **`errors.Is(err, gorm.ErrRecordNotFound)`**：将 GORM 的"记录未找到"转换为领域错误 `ErrNotFound`，调用方无需知道底层 ORM。
- **`Preload("Roles")`**：查询时预加载关联数据，避免 N+1 问题。
- **`Omit("Roles.*")`**：创建时跳过关联的自动创建，由 join table 单独管理。

### 5. 领域与模型的双向转换

在模型文件中定义 `toDomain()` 和 `fromDomain()` 函数，完成持久化模型与领域对象之间的映射。

```go
// internal/app/user/adapter/persistence/mysql/user_model.go

func (m *userModel) toDomain() *userdomain.User {
    if m == nil {
        return nil
    }
    phone := ""
    if m.Phone != nil {
        phone = *m.Phone
    }
    return &userdomain.User{
        ID:            m.ID,
        Username:      m.Username,
        PasswordHash:  m.Password,
        Name:          m.Name,
        Phone:         phone,
        Gender:        userdomain.Gender(m.Gender),
        Status:        userdomain.Status(m.UserStatus),
        Type:          m.UserType,
        OnlineStatus:  userdomain.OnlineStatus(m.UserOnline),
        DeptID:        m.DeptID,
        Remark:        m.Remark,
        RoleIDs:       roleIDsFromModels(m.Roles),
        RoleMenus:     roleMenusFromModels(m.Roles),
        HeartbeatTime: m.HeartbeatTime,
    }
}

func userModelFromDomain(u *userdomain.User) *userModel {
    if u == nil {
        return nil
    }
    phone := u.Phone
    return &userModel{
        Model:         xorm.Model{ID: u.ID},
        Username:      u.Username,
        Password:      u.PasswordHash,
        Name:          u.Name,
        Phone:         &phone,
        Gender:        int8(u.Gender),
        UserStatus:    int32(u.Status),
        UserType:      u.Type,
        UserOnline:    int32(u.OnlineStatus),
        DeptID:        u.DeptID,
        Remark:        u.Remark,
        Roles:         roleModelsFromIDs(u.RoleIDs),
        HeartbeatTime: u.HeartbeatTime,
    }
}

func roleIDsFromModels(roles []roleModel) []uint64 {
    ids := make([]uint64, 0, len(roles))
    for _, role := range roles {
        ids = append(ids, role.ID)
    }
    return ids
}

func roleModelsFromIDs(ids []uint64) []roleModel {
    roles := make([]roleModel, 0, len(ids))
    for _, id := range ids {
        roles = append(roles, roleModel{Model: xorm.Model{ID: id}})
    }
    return roles
}
```

::: tip
对于更新场景，建议定义单独的 `userUpdateModelFromDomain()` 函数，只映射需要更新的字段，避免零值覆盖。参考 `user_model.go` 中的实现。
:::

### 6. 处理关联关系

GORM 关联通过 struct tag 声明，仓储层负责管理关联的增删改。

**多对多（Many-to-Many）**：用户与角色通过 join table 关联。

```go
type userModel struct {
    xorm.Model
    // ...
    Roles []roleModel `gorm:"many2many:user_role"`
}
```

join table 模型需要单独定义：

```go
type userRoleModel struct {
    UserModelID uint64 `gorm:"primaryKey"`
    RoleModelID uint64 `gorm:"primaryKey"`
}

func (userRoleModel) TableName() string {
    return "user_role"
}
```

**替换关联**：先删除旧记录，再批量插入新记录。

```go
func replaceUserRoleJoins(db *gorm.DB, userID uint64, roleIDs []uint64) error {
    // 删除旧关联
    if err := db.Where(map[string]any{"user_model_id": userID}).Delete(&userRoleModel{}).Error; err != nil {
        return err
    }
    if len(roleIDs) == 0 {
        return nil
    }
    // 去重
    seen := make(map[uint64]struct{}, len(roleIDs))
    joins := make([]userRoleModel, 0, len(roleIDs))
    for _, roleID := range roleIDs {
        if roleID == 0 {
            continue
        }
        if _, ok := seen[roleID]; ok {
            continue
        }
        seen[roleID] = struct{}{}
        joins = append(joins, userRoleModel{
            UserModelID: userID,
            RoleModelID: roleID,
        })
    }
    if len(joins) == 0 {
        return nil
    }
    // 批量插入
    return db.CreateInBatches(joins, 100).Error
}
```

**一对多（Has Many）**：角色通过外键关联权限策略。

```go
type roleAggregateModel struct {
    xorm.Model
    Name        string
    Policies    []rolePermissionPolicyModel `gorm:"foreignKey:RoleModelID;references:ID"`
}

type rolePermissionPolicyModel struct {
    RoleModelID uint64
    Service     string
    Method      string
}
```

### 7. 使用事务

`MysqlInterface` 提供两种事务方式：

**方式一：`Transaction` 方法**，适合需要显式事务边界的场景。

```go
func (r *RoleRepository) Update(ctx context.Context, id uint64, aggregate *roledomain.Role) error {
    model := roleModelFromDomain(aggregate)
    if model == nil {
        return roledomain.ErrNotFound
    }
    model.ID = id
    return r.db.Transaction(ctx, func(txCtx context.Context) error {
        db := r.db.WithTx(txCtx)
        if err := db.Model(&roleAggregateModel{Model: xorm.Model{ID: id}}).
            Select("Name", "Typ", "Desc", "Uses", "ViewMenus", "DataPerm").
            Updates(model).Error; err != nil {
            return err
        }
        // 替换关联
        if err := db.Where(map[string]any{"role_model_id": id}).
            Delete(&rolePermissionPolicyModel{}).Error; err != nil {
            return err
        }
        if len(model.Policies) == 0 {
            return nil
        }
        return db.CreateInBatches(model.Policies, 100).Error
    })
}
```

**方式二：`WithTx` 方法**，直接使用 context 中的事务。如果 context 已携带事务则复用，否则使用普通连接。

```go
func (r *UserRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
    return r.db.WithTx(ctx).
        Model(&userModel{Model: xorm.Model{ID: id}}).
        UpdateColumn("password", passwordHash).
        Error
}
```

::: warning
`Transaction` 方法会检测 context 中是否已有事务。如果已存在事务，回调会在已有事务中执行，不会开启嵌套事务。这保证了跨仓储调用的事务一致性。
:::

### 8. Wire 依赖注入绑定

通过 `wire.Bind` 将接口与实现绑定，Wire 在编译期生成注入代码。

```go
// internal/app/user/adapter/persistence/mysql/provider.go
package mysql

import (
    deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
    roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
    userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
    "github.com/google/wire"
)

var ProviderSet = wire.NewSet(
    NewUserRepository,
    NewRoleRepository,
    NewDeptRepository,
    wire.Bind(new(userdomain.Repository), new(*UserRepository)),
    wire.Bind(new(roledomain.Repository), new(*RoleRepository)),
    wire.Bind(new(deptdomain.Repository), new(*DeptRepository)),
)
```

在服务的 Wire injector 中引用此 `ProviderSet`：

```go
// internal/app/user/server/wire.go
//go:build wireinject

package server

import (
    // ...
    "github.com/egoadmin/egoadmin/internal/app/user/adapter/persistence/mysql"
    "github.com/google/wire"
)

func InitServer() (*Server, error) {
    wire.Build(
        mysql.ProviderSet,
        // ... other providers
    )
    return nil, nil
}
```

### 9. 迁移模型注册

所有模型必须注册到 `MigrationModels()` 中，Atlas 用它来生成 DDL 迁移。

```go
// internal/app/user/internal/store/provider.go
package store

import (
    gormadapter "github.com/casbin/gorm-adapter/v3"
    "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

func MigrationModels() []any {
    return []any{
        &LogModel{},
        &RoleModel{},
        &RolePermissionPolicy{},
        &UserModel{},
        &UserRole{},
        &DeptModel{},
        &ConfigModel{},
        &AuthCryptoKeyModel{},
        &gormadapter.CasbinRule{},
    }
}

func MigrationJoinTables() []mysql.MigrationJoinTable {
    return []mysql.MigrationJoinTable{
        {
            Model: &UserModel{},
            Field: "Roles",
            Table: &UserRole{},
        },
    }
}
```

`MigrationJoinTables` 用于注册需要 `SetupJoinTable` 的多对多关联。schema 层将 `store.MigrationModels()` 暴露给 Atlas 迁移命令。

## 配置示例

### GORM 全局配置

GORM 配置在 `internal/platform/database/mysql/mysql.go` 中统一设置：

```go
func GormConfig() *gorm.Config {
    return &gorm.Config{
        DisableForeignKeyConstraintWhenMigrating: true,
        NamingStrategy: &egorm.NamingStrategy{
            SingularTable: true,
        },
    }
}
```

- `DisableForeignKeyConstraintWhenMigrating: true`：迁移时不创建外键约束，由应用层维护引用完整性。
- `SingularTable: true`：表名使用单数形式（`user` 而非 `users`）。

### 数据库连接配置

```toml
# configs/user/config.toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

::: tip
容器部署中使用服务 DNS 名称替代 `127.0.0.1`，并通过 `EGOADMIN_*` 环境变量覆盖 DSN 和密码等敏感配置。
:::

## 实战示例

### 示例一：新增实体模型

假设需要在 user 服务中新增"审计日志"模型：

```go
// internal/app/user/adapter/persistence/mysql/log_model.go
package mysql

import (
    "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
    "github.com/egoadmin/elib/pkg/util/xorm"
    "gorm.io/gorm"
)

type logModel struct {
    xorm.Model
    UserID    uint64
    Username  string
    Action    string
    Detail    string
    IP        string
}

func (logModel) TableName() string {
    return "audit_log"
}

func (m *logModel) SetID(id uint64) {
    if m.ID == 0 {
        m.ID = id
    }
}

func (m *logModel) BeforeCreate(tx *gorm.DB) error {
    return platformmysql.SetID(m)
}
```

然后在 `MigrationModels()` 中注册：

```go
func MigrationModels() []any {
    return []any{
        // ... existing models
        &logModel{},
    }
}
```

运行迁移：

```bash
make migrate.new SERVICE=user NAME=add_audit_log
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

### 示例二：读取并返回领域对象

```go
func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*userdomain.User, error) {
    model := &userModel{}
    err := r.db.WithTx(ctx).Preload("Roles").First(model, id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, userdomain.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return model.toDomain(), nil
}
```

注意 `Preload("Roles")` 必须在 `toDomain()` 之前调用，否则 `m.Roles` 为空，转换后丢失角色数据。

### 示例三：批量更新在线状态

```go
func (r *UserRepository) MarkOffline(ctx context.Context, ids []uint64) error {
    if len(ids) == 0 {
        return nil
    }
    return r.db.WithTx(ctx).
        Model(&userModel{}).
        Where("id IN (?)", ids).
        UpdateColumn("user_online", int32(userdomain.OnlineStatusOffline)).
        Error
}
```

使用 `UpdateColumn` 而非 `Update` 可以跳过 GORM 的零值检查，确保字段被写入。

## 常见问题

::: warning 跨服务访问数据库
每个服务只能直接访问自己的数据库。如果 user 服务需要 gateway 的数据，必须通过 gRPC 客户端调用 gateway 服务，不能直接连接 gateway 的数据库。
:::

::: warning Repository 不要返回 protobuf
Repository 的返回类型必须是领域对象（如 `*userdomain.User`），不能是 protobuf 生成的结构体。protobuf 到领域对象的转换应在 Controller 层完成。
:::

::: warning 不要忘记注册迁移模型
新增模型后必须在 `MigrationModels()` 中注册，否则 Atlas 不会生成对应的 DDL 迁移。join table 也需要在 `MigrationJoinTables()` 中注册。
:::

::: warning 查询时预加载关联
调用 `toDomain()` 之前确保已使用 `Preload()` 加载所有需要的关联数据。否则关联字段会是空值，导致数据丢失。
:::

::: tip 处理可空字段
数据库中的 `NULL` 值使用指针类型表示（如 `Phone *string`）。在 `toDomain()` 中需要做 nil 检查，将 `nil` 转换为空字符串或默认值。
:::

::: tip 避免零值覆盖更新
更新操作建议使用 `Updates()` + `Select()` 显式指定需要更新的字段，或者定义独立的 `updateModelFromDomain()` 只映射有值的字段。直接使用 `Save()` 会覆盖所有字段。
:::

## 参考链接

- [GORM 官方文档](https://gorm.io/docs/)
- [GORM 关联](https://gorm.io/docs/has_many.html)
- [GORM 预加载](https://gorm.io/docs/preload.html)
- [google/wire 依赖注入](https://github.com/google/wire)
- [Atlas 迁移](https://atlasgo.io/)
- 本文源码参考：`internal/app/user/adapter/persistence/mysql/`
