# Go Coding Standards

This page defines the Go coding standards for the EgoAdmin project, covering naming conventions, error handling, function design, dependency injection, transactions, concurrency, data models, lint configuration, and common anti-patterns. All examples are drawn from the actual project source code.

## Overview

The EgoAdmin backend follows a DDD layered architecture, with code distributed across `cmd/<service>`, `internal/app/<service>`, `internal/platform`, and `internal/component`. To maintain consistency and maintainability, all Go code across services follows a unified set of standards.

This guide starts with naming conventions and progresses through error handling, function design, dependency injection, transaction patterns, concurrency control, data models, and the lint toolchain. Each section includes real examples extracted from project source code that can serve directly as reference.

::: tip Reading guide
If you are new to the project, start with the "Naming Conventions" and "Error Handling" sections. If you are writing Service layer code, focus on "Transaction Patterns" and "Concurrency & Locking".
:::

## Core Usage

### 1. Naming Conventions

#### Package Names

Package names must be short, lowercase, and match the responsibility of the service. Avoid underscores, uppercase letters, or overly generic names (like `utils`, `common`).

```go
// Correct: short, semantically clear
package store        // persistence layer
package controller   // gRPC controllers
package service      // service layer
package application  // application layer (use cases)
package user         // user domain

// Wrong: verbose or generic
package user_store_helpers   // no underscores
package common               // vague semantics
```

Actual package names in the project:

| Layer | Path | Package |
|-------|------|---------|
| Controller | `internal/app/user/controller` | `controller` |
| Service | `internal/app/user/service` | `service` |
| Application | `internal/app/user/application` | `application` |
| Domain | `internal/app/user/domain/user` | `user` |
| Store | `internal/app/user/internal/store` | `store` |

#### Exported and Unexported Identifiers

Exported names use PascalCase, unexported names use camelCase.

```go
// Exported: types, functions, methods used by other packages
type UserGRPC struct { ... }
func NewUserGRPCController(...) *UserGRPC { ... }
func (s *UserGRPC) AddUser(ctx context.Context, ...) error { ... }

// Unexported: internal to the package
type Options struct { ... }
func mapUserDomainError(ctx context.Context, err error) error { ... }
func userRolesFromIDs(ids []uint64) []RoleModel { ... }
```

#### Constant Naming

The project uses a mixed style: a few global flags use ALL_CAPS, while most constants use PascalCase with descriptive names.

```go
// iota constants: PascalCase prefix + description
const (
    UserModelBuiltIn     int32 = iota + 1  // built-in user
    UserModelNonBuiltIn                     // regular user
)

const (
    UserModelStatusValid   = iota + 1  // user is active
    UserModelStatusInvalid              // user is inactive
)

// Special value constants: direct PascalCase
const UserModelUsernameRoot  = "root"
const UserModelUsernameAdmin = "admin"
```

#### Interface Naming

Interfaces prefer descriptive nouns or the `-er` suffix. The store layer mainly uses the `Interface` suffix:

```go
// Store layer interfaces: XxxInterface
type UserInterface interface {
    Add(ctx context.Context, user *UserModel) error
    Get(ctx context.Context, id uint64) (*UserModel, error)
    Delete(ctx context.Context, ids []uint64) error
    // ...
}

type RoleInterface interface { ... }
type DeptInterface interface { ... }

// Infrastructure interfaces: XxxInterface
type MysqlInterface interface {
    Transaction(ctx context.Context, callback func(context.Context) error) error
    WithTx(ctx context.Context) *gorm.DB
    Migrate(ctx context.Context, models []any, joinTables []MigrationJoinTable) error
}

// Component interfaces
type authsession.Interface interface { ... }
type LoginCryptoInterface interface { ... }
```

#### Boolean Fields

Boolean fields use `Is`, `Has`, or `Enable` prefixes for improved readability.

```go
// Boolean fields in config structs
type LoginCryptoConfig struct {
    EnableMetrics     bool   `toml:"enableMetrics"`
    ChallengeTTL      string `toml:"challengeTTL"`
}

type UserConf struct {
    MultiLoginEnabled               bool `toml:"multiLoginEnabled"`
    HeartbeatOfflineEnabled         bool `toml:"heartbeatOfflineEnabled"`
    RevokeSessionOnHeartbeatOffline bool `toml:"revokeSessionOnHeartbeatOffline"`
}

// Model constants representing boolean semantics
const (
    UserModelBuiltIn     int32 = 1
    UserModelNonBuiltIn  int32 = 2
)
```

### 2. Error Handling

#### Basic Principles

Never ignore errors with `_`. Every error must be explicitly checked and handled.

```go
// Correct: explicit check
user, err := s.User.Get(ctx, id)
if err != nil {
    return nil, err
}

// Wrong: ignoring the error
user, _ := s.User.Get(ctx, id)  // Forbidden!
```

#### Domain Error Definitions

Each layer defines its own sentinel error variables using `errors.New` with `package: description` format.

```go
// Domain layer: business rule validation errors
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

// Application layer: use-case level business errors
// internal/app/user/application/error.go
package application

import "errors"

var (
    ErrSubmittedPassword = errors.New("user application: submitted password denied")
    ErrLoginUARequired   = errors.New("user application: login user agent required")
)
```

#### Error Classification: errors.Is / errors.As

Use `errors.Is` for sentinel error comparison and `errors.As` for type assertions. Never use `==` to compare wrapped errors.

```go
// Correct: errors.Is
case errors.Is(err, userdomain.ErrBuiltinUsername):
    return platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil)

// Wrong: direct comparison (fails after %w wrapping)
case err == userdomain.ErrBuiltinUsername:  // Forbidden!
```

#### i18n Error Wrapping

The Controller layer uniformly uses `platformi18n.ErrorFailed` to map domain errors to internationalized error codes. The frontend displays localized messages based on these codes.

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
        return err  // unknown errors pass through for the framework to handle
    }
}
```

::: warning Important
The `mapXxxDomainError` function lives in the Controller layer (not Service or Domain), because i18n error codes belong to the API contract and should not penetrate business logic.
:::

### 3. Function Design

#### Controller Method Signatures

All gRPC Controller methods follow a uniform signature: receiving `context.Context` and a proto request, returning a proto response and error. Named return values are used.

```go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    // 1. Initialize out to prevent nil pointer panic
    out = &userv1.AddUserResponse{}

    // 2. Parameter validation
    if in.GetUser().GetPassword() != "" {
        err = mapUserDomainError(ctx, application.ErrSubmittedPassword)
        return
    }

    // 3. Call Service layer
    err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
    if err != nil {
        return
    }

    // 4. Populate response
    out.Id = user.ID
    return
}
```

Key rules:

- **Must** initialize `out` at the start of the method to avoid nil pointer panics
- Use `in.GetXxx()` to access proto fields (handles zero values and nil automatically)
- Controller layer only does parameter conversion and error mapping -- **no business logic**
- Use named return values with naked returns for conciseness

#### Deferred Audit Logging

Write operations use `defer` to record audit logs, only on success:

```go
func (s *UserGRPC) DeleteUser(ctx context.Context, in *userv1.DeleteUserRequest) (out *userv1.DeleteUserResponse, err error) {
    out = &userv1.DeleteUserResponse{}

    // Record audit log on success only
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "删除", "删除用户", in)
        }
    }()

    err = s.user.DeleteUser(ctx, in.GetIds())
    return
}
```

::: tip Why log only on success?
Failed operations should not be written to the audit log, to avoid misleading operation records. If failed attempts need to be tracked, use a separate failure log table.
:::

### 4. Dependency Injection (Wire)

#### ProviderSet Declaration

Each package declares its own `ProviderSet` listing the constructors and bindings it provides.

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
    wire.Struct(new(Options), "*"),  // inject all fields with wildcard
    wire.Bind(new(authsession.Interface), new(*authsession.Component)),
    wire.Bind(new(LoginCryptoInterface), new(*logincrypto.Component)),
    redis.CoreProviderSet,
    store.ProviderSet,
    // ...
)
```

#### Wire Injector Files

Each service's `server/wire.go` uses the `//go:build wireinject` build tag to declare the compile-time dependency graph.

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

::: danger Never manually edit wire_gen.go
`wire_gen.go` is auto-generated by the Wire tool. When changing dependencies, only modify `wire.go` and then run `make gen` to regenerate.
:::

#### Wire Binding Rules

- Interface bindings use `wire.Bind(new(Interface), new(*Concrete))`
- Options struct injection uses `wire.Struct(new(Options), "*")` -- wildcard means inject all fields
- Each `adapter/` sub-package (like `persistence/mysql`, `cache`) has its own `ProviderSet`

### 5. Transaction Patterns

#### Platform Layer Transaction API

The project provides two transaction APIs: `MysqlInterface.Transaction` (for the Service layer) and the lower-level `mysql.Transaction` (for internal Store usage).

```go
// internal/platform/database/mysql/interface_mysql.go
type MysqlInterface interface {
    Transaction(ctx context.Context, callback func(context.Context) error) error
    WithTx(ctx context.Context) *gorm.DB
}
```

#### Service Layer Transaction Usage

The Service layer starts a transaction via `s.Mysql.Transaction`. All database operations within the callback automatically share the same transaction context.

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // ... permission checks ...

    if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
        // Step 1: business checks (within transaction)
        if er := s.addCheck(txCtx, user); er != nil {
            return er
        }
        // Step 2: generate default password
        defaultPassword, er := userdomain.DefaultPasswordFromPhone(stringValue(user.Phone))
        if er != nil {
            return er
        }
        // Step 3: hash password
        hashPass, er := xcrypt.HashAndSalt(defaultPassword)
        if er != nil {
            return er
        }
        user.Password = hashPass
        // Step 4: persist (within transaction)
        return s.User.Add(txCtx, user)
    }); err != nil {
        return
    }

    // Post-commit operations (e.g., reload cache)
    err = s.reloadCasbinRolesForUser(ctx, user.ID)
    return
}
```

::: warning Be careful inside transactions
Only include writes that must be atomic inside the transaction callback. Queries and cache operations should be placed outside the transaction to reduce lock hold time.
:::

#### Store Layer WithTx

The Store layer uses `r.db.WithTx(ctx)` to automatically pick up the transaction object from context. If there is no outer transaction, it uses the normal DB connection.

```go
// Transaction-aware query inside the Store layer
func (m *UserRepository) Get(ctx context.Context, id uint64) (*UserModel, error) {
    db := m.db.WithTx(ctx)  // automatically uses outer transaction
    var user UserModel
    err = db.Preload("Roles").Preload("Roles.Policies").Model(&UserModel{}).First(&user, id).Error
    if err != nil {
        return nil, err
    }
    return &user, nil
}
```

### 6. Concurrency & Locking

#### Redis Distributed Locks

Concurrent write operations (add, update, delete users) use Redis distributed locks to ensure mutual exclusion.

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // Acquire "add" lock
    addlock := s.UserRedis.LockAdd()
    if err = addlock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() {
        _ = addlock.Unlock(ctx)
    }()

    // Acquire "update" lock
    lock := s.UserRedis.LockUpdate()
    if err = lock.Lock(ctx, time.Second*5); err != nil {
        return
    }
    defer func() {
        _ = lock.Unlock(ctx)
    }()

    // ... execute business logic ...
}
```

Locking rules:

- **Must** use `defer` to release locks, preventing deadlocks on exception paths
- Return the error immediately if lock acquisition fails; do not retry
- Lock granularity is by operation type: `LockAdd()`, `LockUpdate()`, etc.
- Lock wait timeout is uniformly `time.Second * 5`

#### Context and Cancellation Propagation

The first parameter of every function must be `context.Context`, used to pass cancellation signals and deadlines.

```go
// Correct: pass context through
func (s *UserService) GetUser(ctx context.Context, id uint64) (*UserModel, error) {
    return s.User.Get(ctx, id)  // ctx is passed down
}

// Wrong: ignoring context
func (s *UserService) GetUser(id uint64) (*UserModel, error) {  // Forbidden!
    return s.User.Get(context.Background(), id)
}
```

### 7. GORM Model Patterns

#### Model Definitions

All data models embed `xorm.Model`, automatically gaining `ID`, `CreatedAt`, `UpdatedAt`, and `DeletedAt` fields.

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
    Roles         []RoleModel `gorm:"many2many:user_role"` // many-to-many association
}
```

GORM tag conventions:

| Tag | Purpose | Example |
|-----|---------|---------|
| `type` | Column data type | `type:varchar(255)` |
| `not null` | NOT NULL constraint | `not null` |
| `default` | Default value | `default:2` |
| `index` | Index | `index:idx_user_deptid,priority:1` |
| `uniqueIndex` | Unique index | `uniqueIndex` |
| `comment` | Column comment | `comment:用户名` |
| `many2many` | Join table for many-to-many | `many2many:user_role` |
| `foreignKey` | Foreign key field | `foreignKey:RoleModelID` |
| `references` | Referenced field | `references:ID` |

#### Repository Pattern

The persistence layer uses an interface + implementation Repository pattern. Interfaces are defined in the `store` package; implementations live in the `adapter/persistence/mysql` package.

```go
// Interface definition (internal/app/user/internal/store/interface_user.go)
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

#### Preloading Associations

Use `Preload` to load associated data and `Association` to manage many-to-many relationships.

```go
// Query user with roles and permission policies preloaded
err = db.Preload("Roles").Preload("Roles.Policies").
    Model(&UserModel{}).First(&user, id).Error

// Replace a user's role associations
if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).
    Association("Roles").Replace(roles); er != nil {
    return er
}
```

### 8. Lint Configuration

The project uses golangci-lint for code quality checks. Run `make lint` to execute.

Enabled linters:

| Linter | Purpose |
|--------|---------|
| `errcheck` | Checks for unhandled error return values |
| `govet` | Runs `go vet` to detect suspicious code |
| `ineffassign` | Detects ineffective assignments |
| `staticcheck` | Advanced static analysis |
| `typecheck` | Type checking |
| `unused` | Detects unused code |
| `bodyclose` | Checks that HTTP response bodies are closed |
| `goconst` | Detects repeated strings that could be constants |
| `gofmt` | Formatting checks |
| `goimports` | Import ordering checks |
| `gosec` | Security checks |
| `misspell` | Spelling checks |
| `prealloc` | Slice preallocation suggestions |
| `unconvert` | Detects unnecessary type conversions |

Exclusion rules:

```yaml
# .golangci.yml
issues:
  exclude-dirs:
    - web/         # frontend code
  exclude-files:
    - ".*pb.go"    # protobuf generated files

linters-settings:
  gosec:
    excludes:
      - G115  # integer overflow conversion -- safe in business context
```

::: tip nolint comments
When a linter warning is confirmed to be valid, suppress it with `//nolint:linter_name // reason`. **A reason is mandatory**.

```go
//nolint:gosec // total is checked above and fits int32.
out.Total = int32(total)
```
:::

### 9. Copier & Response Conversion

The Controller layer uses `github.com/jinzhu/copier` to convert store layer models into proto response messages.

```go
// Simple field mapping
user, err := s.user.GetUser(ctx, in.GetId())
if err != nil {
    return
}
err = copier.Copy(&out.User, &user)

// List mapping
users, total, err := s.user.GetUserListNameOrUserName(ctx, opt, in.GetName())
if err != nil {
    return
}
err = copier.Copy(&out.Users, &users)
```

When proto field names differ from model field names, the `copier` tag maps them:

```protobuf
// api/proto/user/v1/role.proto
message Role {
  uint64 id = 1 [(tagger.tags) = "copier:\"ID\""];
  google.protobuf.Timestamp created_at = 2 [(tagger.tags) = "copier:\"CreatedAtToRPC\""];
  google.protobuf.Timestamp updated_at = 3 [(tagger.tags) = "copier:\"UpdatedAtToRPC\""];
}
```

::: warning Copier limitations
Copier performs shallow copies only; it does not recursively copy nested structures. For complex mappings or custom conversion logic, write the conversion function manually.
:::

## Configuration Examples

Below are runtime configuration snippets relevant to Go services.

### User Service Configuration

```toml
# configs/user/config.toml
[app.user]
jwtExpire = 604800                # access token TTL (seconds)
refreshTokenExpire = 2592000      # refresh token TTL (seconds)
jwtSignKey = "local-egoadmin-jwt-sign-key"
multiLoginEnabled = true          # allow multi-device login
maxLoginClient = 2                # max concurrent login clients
heartbeatOfflineEnabled = true    # auto-offline on heartbeat timeout
heartbeatOfflineSeconds = 660     # heartbeat timeout (seconds)
```

### Login Crypto Component Configuration

```toml
[component.logincrypto]
challengeTTL = "3m0s"         # challenge validity period
timestampSkew = "2m0s"        # timestamp tolerance
rsaKeyBits = 4096             # RSA key size
enableMetrics = true          # enable metrics collection
```

### Environment Variable Overrides

All TOML config values can be overridden via `EGOADMIN_*` prefixed environment variables:

```bash
# Override JWT signing key
EGOADMIN_APP_USER_JWTSIGNKEY=production-secret-key

# Override multi-device login config
EGOADMIN_APP_USER_MULTILOGINENABLED=false

# Override heartbeat timeout
EGOADMIN_APP_USER_HEARTBEATOFFLINESECONDS=300
```

| Config Path | Env Variable | Description |
|-------------|--------------|-------------|
| `app.user.jwtSignKey` | `EGOADMIN_APP_USER_JWTSIGNKEY` | JWT signing key |
| `app.user.multiLoginEnabled` | `EGOADMIN_APP_USER_MULTILOGINENABLED` | Multi-device login toggle |
| `app.user.heartbeatOfflineEnabled` | `EGOADMIN_APP_USER_HEARTBEATOFFLINEENABLED` | Heartbeat offline toggle |
| `component.logincrypto.enableMetrics` | `EGOADMIN_COMPONENT_LOGINCRYPTO_ENABLEMETRICS` | Crypto metrics toggle |

## Real-World Examples

### Complete CRUD Controller Flow

Below is the full user creation flow from Controller to Store.

**Step 1: Controller receives the request and converts parameters**

```go
// internal/app/user/controller/user_grpc.go
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
    out = &userv1.AddUserResponse{}

    // Audit log: recorded on success
    defer func() {
        if err == nil {
            s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
        }
    }()

    // Validation: frontend must not submit a password
    if in.GetUser().GetPassword() != "" {
        err = mapUserDomainError(ctx, application.ErrSubmittedPassword)
        return
    }

    // Convert gender enum
    gender, ok := store.UserModelGenderToInt8(in.GetUser().GetGender())
    if !ok {
        err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
        return
    }

    // Build store model
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

    // Call Service layer and map errors
    err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
    if err != nil {
        return
    }
    out.Id = user.ID
    return
}
```

**Step 2: Service layer orchestrates locks, transactions, and business rules**

```go
// internal/app/user/service/user_service.go
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
    // Data permission check
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

    // Acquire distributed locks
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

    // Transaction: check + generate password + persist
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

    // Post-transaction: reload permission cache
    err = s.reloadCasbinRolesForUser(ctx, user.ID)
    return
}
```

**Step 3: Store layer executes database operations**

```go
// Store layer manages associations via Preload and senses transactions via WithTx
func (m *UserRepository) Add(ctx context.Context, user *UserModel) error {
    return m.db.WithTx(ctx).Create(user).Error
}
```

::: warning Layer responsibilities
Controller handles parameter conversion and error mapping; Service handles business orchestration (locks, transactions, permission checks); Store only performs database operations. **Do not write business logic in the Controller**.
:::

## Common Issues

### Q1: Why do Controller methods return `(out *pb.XxxResponse, err error)` instead of `(*pb.XxxResponse, error)`?

Named return values combined with naked returns make the code more concise. Once `out` is initialized at the start of the method, subsequent code only needs `return` without repeating the return values. Wire and gRPC frameworks also require specific return signatures.

### Q2: When should I use `platformi18n.ErrorFailed` vs. returning `err` directly?

Use `platformi18n.ErrorFailed` for business errors that need user-friendly messages on the frontend (e.g., "username already exists"). For low-level system errors (e.g., database connection failure), return the original `err` directly and let the framework's global error handler deal with it.

### Q3: Why does the Lock's defer use `_ = lock.Unlock(ctx)` to ignore the Unlock error?

A lock release failure typically means the Redis connection is lost, making meaningful recovery impossible. Ignoring the unlock error prevents masking real errors in business logic. Locks have a TTL, so even if Unlock fails, there will be no permanent deadlock.

### Q4: What is the difference between `UserInterface` in the Store layer and `UserService` in the Service layer?

`UserInterface` is the storage interface that defines database CRUD operations. `UserService` is the business service that orchestrates transactions, locks, permission checks, and caching. Service depends on the Store interface, but Store does not know about Service. The dependency direction is: Controller -> Service -> Store.

### Q5: What does the `copier:"ID"` tag in proto files do?

The Go struct generated from proto has a field named `Id`, but the store model field is named `ID`. The copier tag tells copier to map `Id` to `ID` during conversion, preventing mapping failures due to field name mismatches.

### Q6: Will `//nolint:gosec` mask real security issues?

No. In this project, `nolint:gosec` is only used for the G115 rule (integer overflow conversion) and always requires a reason comment. All other gosec security rules (e.g., SQL injection, hardcoded credentials) remain active. If in doubt, do not add nolint -- let CI catch it and review manually.

### Q7: Can I use the `platformi18n` package directly in the Domain layer?

No. The Domain layer is a pure business rule layer and should not depend on any infrastructure package (including i18n). Domain only defines sentinel error variables; i18n mapping happens in the Controller layer. This is a core constraint of the DDD layered architecture.

## Reference Links

- Naming conventions: `internal/app/user/internal/store/interface_user.go`
- Error handling: `internal/app/user/domain/user/error.go`, `internal/app/user/application/error.go`
- Controller examples: `internal/app/user/controller/user_grpc.go`, `internal/app/user/controller/role_grpc.go`
- Service examples: `internal/app/user/service/user_service.go`, `internal/app/user/service/service.go`
- Wire configuration: `internal/app/user/server/wire.go`, `internal/app/user/controller/controller.go`
- Store models: `internal/app/user/internal/store/user.go`, `internal/app/user/internal/store/role.go`
- MysqlInterface: `internal/platform/database/mysql/interface_mysql.go`
- Lint configuration: `.golangci.yml`
