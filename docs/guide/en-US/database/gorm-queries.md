# Queries & Transactions

EgoAdmin performs all database operations at the Repository layer through GORM, uses the Scopes pattern to build reusable query conditions, and propagates transaction context from the Service layer to the Repository layer via `WithTx`.

## Overview

EgoAdmin's data access follows a layered architecture:

- **Service / Application layer**: Manages transaction boundaries by calling `Mysql.Transaction`.
- **Repository layer**: Obtains the transaction-bound `*gorm.DB` via `r.db.WithTx(ctx)` and executes CRUD operations.
- **Domain layer**: Defines error types (e.g. `ErrNotFound`) and aggregates without depending on GORM directly.

::: tip Core Convention
All Repository methods must obtain the database connection through `r.db.WithTx(ctx)`, ensuring queries automatically participate in the caller's transaction. If no transaction exists in the context, it falls back to a regular connection.
:::

## Core Usage

### Find by ID with Preload

Use `Preload` to load associations alongside the main record, avoiding N+1 queries.

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

Key points:

- `First(model, id)` queries by primary key, equivalent to `WHERE id = ? LIMIT 1`.
- `Preload("Roles")` automatically executes a second SQL to load associated roles.
- `gorm.ErrRecordNotFound` is translated to the domain error `ErrNotFound`, keeping upper layers unaware of GORM details.

### Find by Condition

```go
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

::: tip Struct Conditions
When using a struct as a condition, `Where(&userModel{Username: username})` only generates `WHERE` clauses for non-zero fields. This is suitable for exact matches on unique fields.
:::

### Create a Record

```go
func (r *UserRepository) Create(ctx context.Context, aggregate *userdomain.User) error {
    model := userModelFromDomain(aggregate)
    if err := r.db.WithTx(ctx).Omit("Roles.*").Create(model).Error; err != nil {
        return err
    }
    aggregate.ID = model.ID
    return nil
}
```

- `Omit("Roles.*")` skips automatic creation of nested associations, inserting only the main table row.
- After creation, the auto-increment ID is written back to the domain object.

## Scopes Pattern

Scopes is GORM's mechanism for composing query conditions. EgoAdmin encapsulates filter conditions as reusable functions that Repository `List` methods accept via variadic parameters.

### Defining Scopes

```go
func UserScopeUsernameLike(name string) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if name == "" {
            return db
        }
        return db.Where("username LIKE ?", "%"+name+"%")
    }
}

func UserScopeDeptIDs(ids []uint64) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        if len(ids) == 0 {
            return db
        }
        return db.Where("dept_id IN (?)", ids)
    }
}
```

::: warning Security
Always use parameterized queries (`?` placeholders). Never concatenate strings to build SQL.
:::

### Naming Convention

| Type | Pattern | Example |
|------|---------|---------|
| Fuzzy match | `XxxScope{Field}Like` | `UserScopeUsernameLike` |
| Exact match | `XxxScope{Field}` | `UserScopeDeptIDs` |
| Range query | `XxxScope{Field}Between` | `OrderScopeCreatedAtBetween` |
| Status filter | `XxxScope{Field}Eq` | `UserScopeStatusEq` |

## Configuration Examples

### Pagination & Sorting

Pagination parameters are passed via a `PaginateOption` struct, combined with Scopes for flexible list queries.

```go
type PaginateOption struct {
    Page  int    // Page number, starting from 1
    Limit int    // Items per page
    Sort  string // Sort field
    Order string // Sort direction: asc / desc
}
```

Used in the Repository:

```go
func (r *UserRepository) List(
    ctx context.Context,
    opt PaginateOption,
    scopes ...func(*gorm.DB) *gorm.DB,
) ([]*userModel, int64, error) {
    var users []*userModel
    var total int64

    db := r.db.WithTx(ctx)
    for _, scope := range scopes {
        db = db.Scopes(scope)
    }

    // Count total first
    db.Model(&userModel{}).Count(&total)

    // Sort
    if opt.Sort != "" {
        order := opt.Sort
        if opt.Order == "desc" {
            order += " DESC"
        }
        db = db.Order(order)
    }

    // Pagination offset
    offset := (opt.Page - 1) * opt.Limit
    err := db.Offset(offset).Limit(opt.Limit).Find(&users).Error

    return users, total, err
}
```

::: warning Sort Field Whitelist
In production, validate `opt.Sort` against a whitelist of allowed column names to prevent injection via arbitrary field names.
:::

Usage example:

```go
users, total, err := r.userRepo.List(ctx, PaginateOption{
    Page:  1,
    Limit: 20,
    Sort:  "created_at",
    Order: "desc",
}, UserScopeUsernameLike(keyword), UserScopeDeptIDs(deptIDs))
```

## Real-World Examples

### Transaction Pattern

The Service layer opens a transaction via `Mysql.Transaction`. All Repository operations share the same ctx to automatically join the transaction.

```go
// Service / Application layer
err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
    // Validate business rules
    if er := s.addCheck(txCtx, user); er != nil {
        return er
    }

    // Generate and hash default password
    defaultPassword, er := userdomain.DefaultPasswordFromPhone(phone)
    if er != nil {
        return er
    }
    hashPass, er := xbcrypt.HashAndSalt(defaultPassword)
    if er != nil {
        return er
    }
    user.Password = hashPass

    // Persist (automatically joins the transaction)
    return s.User.Add(txCtx, user)
})
```

Transaction flow:

1. `s.Mysql.Transaction(ctx, fn)` creates a new transaction context `txCtx`.
2. All Repository calls inside `fn` pass `txCtx`. The Repository obtains the transaction connection via `r.db.WithTx(txCtx)`.
3. If `fn` returns `nil`, the transaction auto-commits. If it returns `error`, it auto-rolls back.
4. Repository layer does not need to manage transaction lifecycle manually.

::: danger Never Manage Transactions in Repository
Do not call `db.Begin()` / `tx.Commit()` / `tx.Rollback()` in the Repository layer. Transaction boundaries are managed uniformly by the Service layer through the `Transaction` method.
:::

### Association Management (Join Table)

When maintaining many-to-many relationships, use a "delete-then-create" strategy for data consistency.

```go
func replaceUserRoleJoins(db *gorm.DB, userID uint64, roleIDs []uint64) error {
    // 1. Delete existing associations
    if err := db.Where(map[string]any{"user_model_id": userID}).Delete(&userRoleModel{}).Error; err != nil {
        return err
    }
    if len(roleIDs) == 0 {
        return nil
    }

    // 2. Deduplicate and batch-create
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
            UserModelID:  userID,
            RoleModelID:  roleID,
        })
    }

    return db.CreateInBatches(joins, 100).Error
}
```

Key points:

- Delete old records first, then insert, ensuring eventual consistency.
- In-memory deduplication prevents unique constraint violations on duplicate inserts.
- `CreateInBatches(joins, 100)` controls batch size, avoiding oversized transactions.

### DB Model to RPC Message Conversion

DB models returned by the Repository need conversion to Protobuf messages, with special handling for fields like timestamps.

```go
// Model method: convert time to protobuf Timestamp
func (m *userModel) CreatedAtToRPC() *timestamppb.Timestamp {
    if m.CreatedAt.IsZero() {
        return nil
    }
    return timestamppb.New(m.CreatedAt)
}

// Batch conversion example
func modelsToRPC(models []*userModel) []*userv1.User {
    result := make([]*userv1.User, 0, len(models))
    for _, m := range models {
        result = append(result, &userv1.User{
            Id:        m.ID,
            Username:  m.Username,
            Nickname:  m.Nickname,
            CreatedAt: m.CreatedAtToRPC(),
        })
    }
    return result
}
```

::: tip Keep Conversion Methods on the Model
Define conversion logic as Model methods rather than scattering them across Controller or Service code, maintaining clear responsibilities.
:::

## How It Works

### WithTx Transaction Propagation

```
Service Layer                      Repository Layer
    |                                   |
    |-- Mysql.Transaction(ctx, fn)      |
    |   Creates txCtx (with tx key)     |
    |                                   |
    |--- Passes txCtx ----------------->|
    |                                   |-- r.db.WithTx(txCtx)
    |                                   |   Detects tx key in ctx
    |                                   |   Returns tx-bound *gorm.DB
    |                                   |
    |                                   |-- Executes query (inside tx)
    |                                   |
    |<--- Returns result ---------------|
    |
    |-- fn returns nil  -> COMMIT
    |-- fn returns err  -> ROLLBACK
```

`WithTx` is a transaction propagation method encapsulated in EgoAdmin's platform layer. It extracts the transaction object from the context:

- If a transaction exists in ctx (injected by `Transaction`), it returns the transaction-bound `*gorm.DB`.
- If no transaction exists in ctx, it returns a regular (non-transaction) `*gorm.DB`.

This allows Repository code to remain agnostic about whether the caller opened a transaction -- behavior is fully determined by the call chain.

### Scopes Execution Flow

```
List(ctx, option, scope1, scope2)
    |
    |-- db = r.db.WithTx(ctx)        // Base connection
    |-- db = db.Scopes(scope1)       // Append condition 1
    |-- db = db.Scopes(scope2)       // Append condition 2
    |-- db.Model(...).Count(&total)  // Count total
    |-- db.Order(...).Offset(...)    // Sort + paginate
    |-- db.Find(&results)            // Execute query
```

GORM Scopes are lazily evaluated. Each `Scopes()` call merely appends conditions to `*gorm.DB` without executing SQL immediately. The final SQL is generated and executed only when a terminal method like `Find`, `First`, or `Count` is called.

## Common Issues

### N+1 Query Problem

**Symptom**: Association data is queried one-by-one in a loop. Performance degrades linearly with data volume.

**Solution**: Use `Preload` to batch-load associations when querying the main record.

```go
// Wrong: N+1
for _, user := range users {
    roles, _ := r.FindRolesByUserID(user.ID) // One SQL per iteration
}

// Correct: Preload
r.db.WithTx(ctx).Preload("Roles").Find(&users) // Two SQLs total
```

### Forgetting WithTx

**Symptom**: The Service layer opens a transaction, but Repository operations don't join it. Partial operations cannot be rolled back.

**Solution**: Always use `r.db.WithTx(ctx)` instead of `r.db` in Repository methods.

```go
// Wrong: bypasses transaction
func (r *UserRepository) Create(ctx context.Context, u *User) error {
    return r.db.Create(u).Error // Executes outside transaction
}

// Correct: joins transaction
func (r *UserRepository) Create(ctx context.Context, u *User) error {
    return r.db.WithTx(ctx).Create(u).Error // Auto-joins caller's transaction
}
```

### Sort Field Injection

**Symptom**: User-controlled sort parameters are directly interpolated into SQL.

**Solution**: Maintain a sort field whitelist and validate before execution.

```go
var allowedSortFields = map[string]bool{
    "id":         true,
    "username":   true,
    "created_at": true,
    "updated_at": true,
}

func validateSort(field string) string {
    if allowedSortFields[field] {
        return field
    }
    return "id" // Default sort
}
```

### Batch Operation Performance

**Symptom**: Inserting/updating one record at a time in a loop.

**Solution**: Use `CreateInBatches` for batch inserts and `Save` for batch updates.

```go
// One-by-one (slow)
for _, item := range items {
    db.Create(&item)
}

// Batch (fast)
db.CreateInBatches(items, 100)
```

## Reference Links

- [GORM Official Docs - Query](https://gorm.io/docs/query.html)
- [GORM Official Docs - Scopes](https://gorm.io/docs/scopes.html)
- [GORM Official Docs - Transactions](https://gorm.io/docs/transactions.html)
- [GORM Official Docs - Associations](https://gorm.io/docs/has_many.html)
- [This Project - Database & Migration Overview](/en-US/guide/database-migration)
- [This Project - GORM Models & Repositories](/en-US/guide/database/gorm-models)
