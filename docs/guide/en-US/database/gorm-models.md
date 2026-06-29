# GORM Models and Repository Layer

EgoAdmin uses GORM to define persistence models and decouples domain objects from database records through the DDD repository pattern. This article covers the complete workflow of model definition, repository implementation, association handling, and Wire binding.

## Overview

There are two persistence paths in the EgoAdmin project:

| Path | Location | Description |
|------|----------|-------------|
| **DDD Target** | `internal/app/<service>/adapter/persistence/mysql/` | New code should be written here, following Domain-Driven Design |
| **Migration Transition** | `internal/app/<service>/internal/store/` | Existing code, gradually migrating to DDD structure |

Core data flow:

```
Domain Aggregate (领域聚合根)
       |                    ^
       v                    |
   Model (GORM Model)  toDomain() / fromDomain()
       |
       v
   MySQL Table
```

The domain layer defines the `Repository` interface, and the persistence layer provides the `MySQL` implementation. Models and domain objects are converted bidirectionally through `toDomain()` / `fromDomain()` functions, ensuring the domain layer does not depend on GORM.

## Core Usage

### 1. Define a GORM Model

Models are placed in the `internal/app/<service>/adapter/persistence/mysql/` directory, embedding `xorm.Model` to obtain ID and timestamp fields.

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

`xorm.Model` provides the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `uint64` | Snowflake ID primary key |
| `CreatedAt` | `time.Time` | Creation time |
| `UpdatedAt` | `time.Time` | Update time |

The `BeforeCreate` hook calls `platformmysql.SetID(m)`, which automatically assigns an ID via the global snowflake generator. Each model needs to implement both the `SetID(id uint64)` method and the `TableName() string` method.

::: tip
Mapping between model field names and database column names is handled automatically by GORM's `NamingStrategy` (camelCase to snake_case). `SingularTable: true` ensures table names do not have a plural suffix. To customize column names, use the `gorm:"column:xxx"` tag.
:::

### 2. Define a Domain Aggregate Root

Domain objects are placed in the `internal/app/<service>/domain/<aggregate>/` directory and are pure Go structs with no framework dependencies.

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

Domain errors are defined uniformly in `error.go`:

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
Domain errors are defined as sentinel errors using `errors.New`; do not use `fmt.Errorf`. Callers check errors using `errors.Is(err, userdomain.ErrNotFound)`.
:::

### 3. Define the Repository Interface

The repository interface is defined in the domain layer, describing persistence capabilities without exposing GORM details.

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

### 4. Implement the Repository

The repository implementation uses `platformmysql.MysqlInterface` to obtain a GORM `*gorm.DB`, and uses `var _ interface = (*Impl)(nil)` to ensure compile-time interface checking.

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

Key points:

- **`r.db.WithTx(ctx)`**: Retrieves the current transaction from the context, falling back to a regular connection if no transaction exists. All reads and writes must go through this method to ensure operations within the same request share a transaction.
- **`errors.Is(err, gorm.ErrRecordNotFound)`**: Converts GORM's "record not found" error into the domain error `ErrNotFound`, so callers do not need to know about the underlying ORM.
- **`Preload("Roles")`**: Preloads associated data during queries to avoid the N+1 problem.
- **`Omit("Roles.*")`**: Skips automatic creation of associations during creation, which are managed separately via the join table.

### 5. Bidirectional Conversion Between Domain and Model

Define `toDomain()` and `fromDomain()` functions in the model file to map between the persistence model and the domain object.

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
For update scenarios, it is recommended to define a separate `userUpdateModelFromDomain()` function that only maps the fields that need to be updated, avoiding zero-value overwrite. Refer to the implementation in `user_model.go`.
:::

### 6. Handle Associations

GORM associations are declared via struct tags, and the repository layer is responsible for managing the creation, deletion, and modification of associations.

**Many-to-Many**: Users and roles are associated through a join table.

```go
type userModel struct {
    xorm.Model
    // ...
    Roles []roleModel `gorm:"many2many:user_role"`
}
```

The join table model needs to be defined separately:

```go
type userRoleModel struct {
    UserModelID uint64 `gorm:"primaryKey"`
    RoleModelID uint64 `gorm:"primaryKey"`
}

func (userRoleModel) TableName() string {
    return "user_role"
}
```

**Replacing Associations**: Delete old records first, then batch insert new records.

```go
func replaceUserRoleJoins(db *gorm.DB, userID uint64, roleIDs []uint64) error {
    // Delete old associations
    if err := db.Where(map[string]any{"user_model_id": userID}).Delete(&userRoleModel{}).Error; err != nil {
        return err
    }
    if len(roleIDs) == 0 {
        return nil
    }
    // Deduplicate
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
    // Batch insert
    return db.CreateInBatches(joins, 100).Error
}
```

**Has Many**: Roles are associated with permission policies through a foreign key.

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

### 7. Using Transactions

`MysqlInterface` provides two transaction approaches:

**Approach 1: `Transaction` method**, suitable for scenarios that require explicit transaction boundaries.

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
        // Replace associations
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

**Approach 2: `WithTx` method**, directly uses the transaction in the context. If the context already carries a transaction, it reuses it; otherwise, it uses a regular connection.

```go
func (r *UserRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
    return r.db.WithTx(ctx).
        Model(&userModel{Model: xorm.Model{ID: id}}).
        UpdateColumn("password", passwordHash).
        Error
}
```

::: warning
The `Transaction` method detects whether a transaction already exists in the context. If a transaction already exists, the callback executes within the existing transaction without opening a nested transaction. This ensures transaction consistency across cross-repository calls.
:::

### 8. Wire Dependency Injection Binding

Bind interfaces to implementations using `wire.Bind`; Wire generates injection code at compile time.

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

Reference this `ProviderSet` in the service's Wire injector:

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

### 9. Migration Model Registration

All models must be registered in `MigrationModels()`, which Atlas uses to generate DDL migrations.

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

`MigrationJoinTables` is used to register many-to-many associations that require `SetupJoinTable`. The schema layer exposes `store.MigrationModels()` to the Atlas migration commands.

## Configuration Examples

### GORM Global Configuration

GORM configuration is set uniformly in `internal/platform/database/mysql/mysql.go`:

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

- `DisableForeignKeyConstraintWhenMigrating: true`: Does not create foreign key constraints during migrations; referential integrity is maintained by the application layer.
- `SingularTable: true`: Table names use the singular form (`user` instead of `users`).

### Database Connection Configuration

```toml
# configs/user/config.toml
[client.mysql]
dsn = "egoadmin:egoadmin@tcp(127.0.0.1:3307)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

::: tip
In container deployments, use service DNS names instead of `127.0.0.1`, and override sensitive configuration such as DSN and passwords via `EGOADMIN_*` environment variables.
:::

## Practical Examples

### Example 1: Adding a New Entity Model

Suppose you need to add an "audit log" model to the user service:

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

Then register it in `MigrationModels()`:

```go
func MigrationModels() []any {
    return []any{
        // ... existing models
        &logModel{},
    }
}
```

Run migrations:

```bash
make migrate.new SERVICE=user NAME=add_audit_log
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

### Example 2: Reading and Returning Domain Objects

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

Note that `Preload("Roles")` must be called before `toDomain()`; otherwise `m.Roles` will be empty, causing role data to be lost after conversion.

### Example 3: Batch Updating Online Status

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

Using `UpdateColumn` instead of `Update` bypasses GORM's zero-value check, ensuring the field is written.

## Common Issues

::: warning Cross-Service Database Access
Each service can only directly access its own database. If the user service needs gateway data, it must call the gateway service through a gRPC client and cannot directly connect to the gateway's database.
:::

::: warning Repository Should Not Return Protobuf
The return type of a Repository must be a domain object (such as `*userdomain.User`), not a protobuf-generated struct. The conversion from protobuf to domain objects should be completed at the Controller layer.
:::

::: warning Do Not Forget to Register Migration Models
After adding a new model, it must be registered in `MigrationModels()`; otherwise Atlas will not generate the corresponding DDL migration. Join tables also need to be registered in `MigrationJoinTables()`.
:::

::: warning Preload Associations During Queries
Before calling `toDomain()`, ensure that all required associated data has been loaded using `Preload()`. Otherwise, association fields will be empty values, resulting in data loss.
:::

::: tip Handling Nullable Fields
`NULL` values in the database are represented using pointer types (such as `Phone *string`). In `toDomain()`, you need to perform a nil check, converting `nil` to an empty string or default value.
:::

::: tip Avoid Zero-Value Overwrite on Update
For update operations, it is recommended to use `Updates()` + `Select()` to explicitly specify the fields to update, or define a separate `updateModelFromDomain()` that only maps fields with values. Using `Save()` directly will overwrite all fields.
:::

## Reference Links

- [GORM Official Documentation](https://gorm.io/docs/)
- [GORM Associations](https://gorm.io/docs/has_many.html)
- [GORM Preloading](https://gorm.io/docs/preload.html)
- [google/wire Dependency Injection](https://github.com/google/wire)
- [Atlas Migrations](https://atlasgo.io/)
- Source code reference for this article: `internal/app/user/adapter/persistence/mysql/`
