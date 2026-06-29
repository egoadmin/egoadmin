# 数据库与迁移

EgoAdmin 使用 GORM 定义模型，通过 Atlas 生成和执行版本化迁移。迁移边界按服务划分。

## 数据库边界

| 服务 | 数据库 | 迁移目录 |
|------|--------|----------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

规则：

- 服务只能直接访问自己的数据库。
- 迁移目录必须和服务数据库匹配。
- GORM model 是 schema 源，Atlas migration 是部署源。
- 不手写维护 HCL 作为唯一 schema 源。

## GORM 模型

```go
type UserModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex;comment:用户名"`
  Nickname  string    `gorm:"column:nickname;type:varchar(64);not null;default:'';comment:昵称"`
  DeptID    uint64    `gorm:"column:dept_id;not null;default:0;index;comment:部门ID"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string {
  return "user"
}
```

模型规则：

- 表名通过 `TableName()` 显式声明。
- 字段写清 `column`、类型、索引、默认值、注释。
- 服务自己的 model 放在 `internal/app/<service>/adapter/persistence/mysql`。
- join table 和 Casbin 表也通过服务 schema 清单导出。

## 迁移模型注册

```go
// internal/app/user/schema/schema.go
package schema

import (
  casbinadapter "github.com/casbin/gorm-adapter/v3"
  "github.com/egoadmin/egoadmin/internal/app/user/adapter/persistence/mysql"
)

func MigrationModels() []any {
  return []any{
    &mysql.UserModel{},
    &mysql.RoleModel{},
    &mysql.DeptModel{},
    &mysql.AuditLogModel{},
    &casbinadapter.CasbinRule{},
  }
}

func MigrationJoinTables() []any {
  return []any{
    &mysql.UserRoleModel{},
    &mysql.RoleDeptModel{},
  }
}
```

## 生成迁移

```bash
make install
make gen

make migrate.new SERVICE=user NAME=add_user_profile
```

输出：

```text
atlas/migrations/user/
├── 20260629120000_add_user_profile.sql
└── atlas.sum
```

校验迁移：

```bash
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

## 生成 schema 快照

用于评审最终 schema，而不是部署：

```bash
make migrate.schema SERVICE=user
make migrate.schema SERVICE=user DIALECT=postgres
```

输出：

```text
atlas/schema/mysql/user.hcl
atlas/schema/postgres/user.hcl
```

## 多方言支持

```bash
go run ./tools/atlasloader --service user --dialect mysql
go run ./tools/atlasloader --service user --dialect postgres
go run ./tools/atlasloader --service user --dialect sqlite
go run ./tools/atlasloader --service user --dialect sqlserver
```

生成 PostgreSQL 迁移：

```bash
make migrate.new SERVICE=user NAME=add_field DIALECT=postgres
```

目录：

```text
atlas/migrations/postgres/user
```

## 运行时迁移配置

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

`app.service.autoMigrate` 默认保持 `false`。

```toml
[app.service]
autoMigrate = false
```

::: warning
生产部署依赖 Atlas 版本化迁移，不依赖 GORM AutoMigrate。AutoMigrate 只作为本地开发兜底能力。
:::

## 查询模式

Repository 中使用 context 传播超时和取消：

```go
func (r *UserRepository) List(ctx context.Context, q user.Query) ([]*user.User, int64, error) {
  db := r.db.WithContext(ctx).Model(&UserModel{})

  if q.Username != "" {
    db = db.Where("username LIKE ?", "%"+q.Username+"%")
  }
  if q.DeptID > 0 {
    db = db.Where("dept_id = ?", q.DeptID)
  }

  var total int64
  if err := db.Count(&total).Error; err != nil {
    return nil, 0, err
  }

  var rows []UserModel
  if err := db.Offset(int((q.Page - 1) * q.Limit)).Limit(int(q.Limit)).Find(&rows).Error; err != nil {
    return nil, 0, err
  }

  return toUsers(rows), total, nil
}
```

## 事务模式

单服务事务在 application 层编排：

```go
func (uc *RoleUseCase) UpdateRole(ctx context.Context, cmd UpdateRoleCommand) error {
  return uc.tx.RunInTx(ctx, func(txCtx context.Context) error {
    if err := uc.roles.Update(txCtx, cmd.Role); err != nil {
      return err
    }
    if err := uc.permissions.ReplaceRoleMenus(txCtx, cmd.Role.ID, cmd.MenuIDs); err != nil {
      return err
    }
    return nil
  })
}
```

跨服务写入再考虑 DTM。

## 常见迁移错误

| 问题 | 原因 | 处理 |
|------|------|------|
| Atlas dirty database | 上一次迁移失败 | 修复数据库状态后重新 apply |
| atlas.sum 不匹配 | 手工改了 migration 文件 | 确认 SQL 后执行 `make migrate.hash SERVICE=user` |
| 迁移目录错误 | SERVICE 和数据库不匹配 | 检查 `app.dbMigration.dir` |
| 表已存在 | AutoMigrate 或手工建表造成差异 | 清理本地库或写显式迁移 |

## 验证命令

```bash
make migrate.new SERVICE=user NAME=example
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
go test ./internal/app/user/adapter/persistence/mysql/...
```

