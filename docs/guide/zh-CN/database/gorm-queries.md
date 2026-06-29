# 查询与事务

EgoAdmin 在 Repository 层通过 GORM 进行所有数据库操作，使用 Scopes 模式构建可复用查询条件，通过 `WithTx` 将事务上下文从 Service 层传递到 Repository 层。

## 概览

EgoAdmin 的数据访问遵循分层架构：

- **Service / Application 层**：管理事务边界，调用 `Mysql.Transaction` 开启事务。
- **Repository 层**：通过 `r.db.WithTx(ctx)` 获取当前事务绑定的 `*gorm.DB`，执行 CRUD 操作。
- **Domain 层**：定义错误类型（如 `ErrNotFound`）和聚合根，不直接依赖 GORM。

::: tip 核心约定
所有 Repository 方法必须通过 `r.db.WithTx(ctx)` 获取数据库连接，确保查询自动参与调用方开启的事务。如果 ctx 中没有事务，则退化为普通连接。
:::

## 核心用法

### 通过 ID 查询并预加载关联

使用 `Preload` 在查询主记录的同时加载关联数据，避免 N+1 查询。

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

要点：

- `First(model, id)` 使用主键查询，等价于 `WHERE id = ? LIMIT 1`。
- `Preload("Roles")` 自动执行第二条 SQL 加载关联角色。
- `gorm.ErrRecordNotFound` 转换为领域错误 `ErrNotFound`，使上层无需了解 GORM 细节。

### 通过条件查询

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

::: tip 结构体条件
`Where(&userModel{Username: username})` 使用结构体作为条件时，GORM 只会使用非零值字段生成 `WHERE` 子句。适合按唯一字段精确匹配。
:::

### 创建记录

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

- `Omit("Roles.*")` 跳过嵌套关联的自动创建，仅插入主表记录。
- 创建后将自增 ID 回写到领域对象。

## Scopes 模式

Scopes 是 GORM 提供的查询条件组合机制。EgoAdmin 使用 Scopes 将过滤条件封装为可复用函数，Repository 的 `List` 方法通过可变参数接收。

### 定义 Scopes

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

::: warning 安全提示
始终使用参数化查询（`?` 占位符），避免字符串拼接导致 SQL 注入。
:::

### 命名规范

| 类型 | 命名模式 | 示例 |
|------|---------|------|
| 模糊匹配 | `XxxScope{Field}Like` | `UserScopeUsernameLike` |
| 精确匹配 | `XxxScope{Field}` | `UserScopeDeptIDs` |
| 范围查询 | `XxxScope{Field}Between` | `OrderScopeCreatedAtBetween` |
| 状态过滤 | `XxxScope{Field}Eq` | `UserScopeStatusEq` |

## 配置示例

### 分页查询

分页参数通过 `PaginateOption` 结构体传递，配合 Scopes 实现灵活的列表查询。

```go
type PaginateOption struct {
    Page  int    // 页码，从 1 开始
    Limit int    // 每页条数
    Sort  string // 排序字段
    Order string // 排序方向：asc / desc
}
```

在 Repository 中使用：

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

    // 先统计总数
    db.Model(&userModel{}).Count(&total)

    // 排序
    if opt.Sort != "" {
        order := opt.Sort
        if opt.Order == "desc" {
            order += " DESC"
        }
        db = db.Order(order)
    }

    // 分页偏移
    offset := (opt.Page - 1) * opt.Limit
    err := db.Offset(offset).Limit(opt.Limit).Find(&users).Error

    return users, total, err
}
```

::: warning 排序字段白名单
生产环境应对 `opt.Sort` 做白名单校验，防止通过非法字段名进行注入攻击。
:::

调用示例：

```go
users, total, err := r.userRepo.List(ctx, PaginateOption{
    Page:  1,
    Limit: 20,
    Sort:  "created_at",
    Order: "desc",
}, UserScopeUsernameLike(keyword), UserScopeDeptIDs(deptIDs))
```

## 实战案例

### 事务模式

Service 层通过 `Mysql.Transaction` 开启事务，所有 Repository 操作共享同一个 ctx 即可自动加入事务。

```go
// Service / Application 层
err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
    // 校验业务规则
    if er := s.addCheck(txCtx, user); er != nil {
        return er
    }

    // 生成并哈希默认密码
    defaultPassword, er := userdomain.DefaultPasswordFromPhone(phone)
    if er != nil {
        return er
    }
    hashPass, er := xbcrypt.HashAndSalt(defaultPassword)
    if er != nil {
        return er
    }
    user.Password = hashPass

    // 持久化（自动参与事务）
    return s.User.Add(txCtx, user)
})
```

事务流程说明：

1. `s.Mysql.Transaction(ctx, fn)` 创建一个新的事务上下文 `txCtx`。
2. `fn` 中所有 Repository 调用传入 `txCtx`，Repository 内部通过 `r.db.WithTx(txCtx)` 获取事务连接。
3. `fn` 返回 `nil` 时自动提交，返回 `error` 时自动回滚。
4. 不需要在 Repository 层手动管理事务生命周期。

::: danger 禁止手动管理事务
不要在 Repository 层调用 `db.Begin()` / `tx.Commit()` / `tx.Rollback()`。事务边界由 Service 层通过 `Transaction` 方法统一管理。
:::

### 关联表管理（Join Table）

维护多对多关系时，使用"先删后建"策略确保数据一致性。

```go
func replaceUserRoleJoins(db *gorm.DB, userID uint64, roleIDs []uint64) error {
    // 1. 删除已有关联
    if err := db.Where(map[string]any{"user_model_id": userID}).Delete(&userRoleModel{}).Error; err != nil {
        return err
    }
    if len(roleIDs) == 0 {
        return nil
    }

    // 2. 去重并批量创建
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

要点：

- 删除旧记录后再插入，保证最终一致性。
- 内存去重防止重复插入违反唯一约束。
- `CreateInBatches(joins, 100)` 控制批量大小，避免大事务。

### DB 模型到 RPC 消息的转换

Repository 返回的 DB 模型需要转换为 Protobuf 消息，时间字段等需特殊处理。

```go
// Model 方法：时间转 protobuf Timestamp
func (m *userModel) CreatedAtToRPC() *timestamppb.Timestamp {
    if m.CreatedAt.IsZero() {
        return nil
    }
    return timestamppb.New(m.CreatedAt)
}

// 批量转换示例
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

::: tip 转换方法放在 Model 层
将转换逻辑定义为 Model 的方法，而非散落在 Controller 或 Service 中，保持职责清晰。
:::

## 工作原理

### WithTx 事务传播机制

```
Service 层                      Repository 层
    |                               |
    |-- Mysql.Transaction(ctx,fn)   |
    |   创建 txCtx (含 tx key)      |
    |                               |
    |---传入 txCtx----------------->|
    |                               |-- r.db.WithTx(txCtx)
    |                               |   检测 ctx 中的 tx key
    |                               |   返回绑定到事务的 *gorm.DB
    |                               |
    |                               |-- 执行查询（在事务内）
    |                               |
    |<---返回结果-------------------|
    |
    |-- fn 返回 nil -> COMMIT
    |-- fn 返回 err -> ROLLBACK
```

`WithTx` 是 EgoAdmin 平台层封装的事务传播方法。它从 context 中提取事务对象：

- 如果 ctx 中存在事务（由 `Transaction` 方法注入），返回该事务绑定的 `*gorm.DB`。
- 如果 ctx 中没有事务，返回普通（非事务）的 `*gorm.DB`。

这使得 Repository 代码不需要关心调用方是否开启了事务，行为完全由调用链决定。

### Scopes 执行流程

```
List(ctx, option, scope1, scope2)
    |
    |-- db = r.db.WithTx(ctx)       // 基础连接
    |-- db = db.Scopes(scope1)      // 追加条件 1
    |-- db = db.Scopes(scope2)      // 追加条件 2
    |-- db.Model(...).Count(&total) // 统计总数
    |-- db.Order(...).Offset(...)   // 排序 + 分页
    |-- db.Find(&results)           // 执行查询
```

GORM 的 Scopes 是惰性执行的，每个 `Scopes()` 调用只是向 `*gorm.DB` 追加条件，不会立即执行 SQL。直到调用 `Find`、`First`、`Count` 等终端方法时才生成并执行最终 SQL。

## 常见问题

### N+1 查询问题

**症状**：循环中逐条查询关联数据，性能随数据量线性下降。

**解决**：在查询主记录时使用 `Preload` 批量加载关联。

```go
// 错误：N+1
for _, user := range users {
    roles, _ := r.FindRolesByUserID(user.ID) // 每次一次 SQL
}

// 正确：Preload
r.db.WithTx(ctx).Preload("Roles").Find(&users) // 两次 SQL
```

### 忘记使用 WithTx

**症状**：Service 层开启了事务，但 Repository 的操作没有加入事务，部分操作无法回滚。

**解决**：Repository 方法始终使用 `r.db.WithTx(ctx)` 而非 `r.db`。

```go
// 错误：绕过事务
func (r *UserRepository) Create(ctx context.Context, u *User) error {
    return r.db.Create(u).Error // 在事务外执行
}

// 正确：参与事务
func (r *UserRepository) Create(ctx context.Context, u *User) error {
    return r.db.WithTx(ctx).Create(u).Error // 自动参与调用方事务
}
```

### 排序字段注入

**症状**：用户可控的排序参数直接拼接到 SQL 中。

**解决**：维护排序字段白名单，在执行前校验。

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
    return "id" // 默认排序
}
```

### 批量操作性能

**症状**：循环中逐条插入/更新，性能差。

**解决**：使用 `CreateInBatches` 批量插入，`Save` 批量更新。

```go
// 逐条（慢）
for _, item := range items {
    db.Create(&item)
}

// 批量（快）
db.CreateInBatches(items, 100)
```

## 参考链接

- [GORM 官方文档 - 查询](https://gorm.io/docs/query.html)
- [GORM 官方文档 - Scopes](https://gorm.io/docs/scopes.html)
- [GORM 官方文档 - 事务](https://gorm.io/docs/transactions.html)
- [GORM 官方文档 - 关联](https://gorm.io/docs/has_many.html)
- [本项目 - 数据库与迁移概览](/guide/database-migration)
- [本项目 - GORM 模型与仓储](/guide/database/gorm-models)
