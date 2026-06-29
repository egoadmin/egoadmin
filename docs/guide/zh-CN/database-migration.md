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

### 模型定义规范

```go
// internal/app/user/adapter/persistence/mysql/user_model.go
package mysql

import "time"

type UserModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex;comment:用户名"`
  Nickname  string    `gorm:"column:nickname;type:varchar(64);not null;default:'';comment:昵称"`
  Email     string    `gorm:"column:email;type:varchar(128);default:'';index;comment:邮箱"`
  Phone     string    `gorm:"column:phone;type:varchar(20);default:'';index;comment:手机号"`
  Avatar    string    `gorm:"column:avatar;type:varchar(512);default:'';comment:头像URL"`
  Password  string    `gorm:"column:password;type:varchar(128);not null;comment:密码哈希"`
  DeptID    uint64    `gorm:"column:dept_id;not null;default:0;index;comment:部门ID"`
  OwnerUID  uint64    `gorm:"column:owner_user_id;not null;default:0;index;comment:创建者ID"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态 1=启用 0=禁用"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string {
  return "user"
}
```

### 角色模型

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
type RoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name      string    `gorm:"column:name;type:varchar(64);not null;uniqueIndex;comment:角色名"`
  Code      string    `gorm:"column:code;type:varchar(64);not null;uniqueIndex;comment:角色编码"`
  DataScope int32     `gorm:"column:data_scope;type:tinyint;not null;default:1;comment:数据范围"`
  Sort      int32     `gorm:"column:sort;not null;default:0;comment:排序"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
  Remark    string    `gorm:"column:remark;type:varchar(255);default:'';comment:备注"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RoleModel) TableName() string {
  return "role"
}
```

### 部门模型

```go
// internal/app/user/adapter/persistence/mysql/dept_model.go
type DeptModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  ParentID  uint64    `gorm:"column:parent_id;not null;default:0;index;comment:上级部门ID"`
  Name      string    `gorm:"column:name;type:varchar(64);not null;comment:部门名"`
  Sort      int32     `gorm:"column:sort;not null;default:0;comment:排序"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (DeptModel) TableName() string {
  return "dept"
}
```

模型规则：

- 表名通过 `TableName()` 显式声明。
- 字段写清 `column`、类型、索引、默认值、注释。
- 服务自己的 model 放在 `internal/app/<service>/adapter/persistence/mysql`。
- join table 和 Casbin 表也通过服务 schema 清单导出。
- `owner_user_id` 字段用于 DataScope 本人权限过滤。

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

### Join Table 定义

Join table 用于多对多关联，不使用 GORM 的 `many2many` tag 自动建表，而是显式定义模型：

```go
// internal/app/user/adapter/persistence/mysql/user_role_model.go
type UserRoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  UserID    uint64    `gorm:"column:user_id;not null;index:idx_user_role,unique;comment:用户ID"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_user_role,unique;comment:角色ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserRoleModel) TableName() string {
  return "user_role"
}

// internal/app/user/adapter/persistence/mysql/role_menu_model.go
type RoleMenuModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_role_menu,unique;comment:角色ID"`
  MenuID    uint64    `gorm:"column:menu_id;not null;index:idx_role_menu,unique;comment:菜单ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RoleMenuModel) TableName() string {
  return "role_menu"
}

// internal/app/user/adapter/persistence/mysql/role_dept_model.go
type RoleDeptModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_role_dept,unique;comment:角色ID"`
  DeptID    uint64    `gorm:"column:dept_id;not null;index:idx_role_dept,unique;comment:部门ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RoleDeptModel) TableName() string {
  return "role_dept"
}
```

::: tip
Join table 需要在 `MigrationJoinTables()` 中注册才能被 Atlas 导出。联合唯一索引确保不会产生重复关联。
:::

## Atlas 迁移工作流

### 完整工作流示例

```bash
# 1. 确保工具链安装
make install

# 2. 生成 proto 和 Wire 代码
make gen

# 3. 修改 GORM model（例如添加 Profile 字段）
# internal/app/user/adapter/persistence/mysql/user_model.go

# 4. 在 schema 注册中添加新 model（如有新表）
# internal/app/user/schema/schema.go

# 5. 生成迁移 SQL
make migrate.new SERVICE=user NAME=add_user_profile

# 6. 检查生成的 SQL 文件
cat atlas/migrations/user/20260629120000_add_user_profile.sql

# 7. 校验迁移文件语法
make migrate.validate SERVICE=user

# 8. 更新 atlas.sum 校验和
make migrate.hash SERVICE=user

# 9. 运行测试
go test ./internal/app/user/...

# 10. 本地执行迁移（如果使用 make run，会自动执行）
make dev-up
make run SERVICE=user
```

输出：

```text
atlas/migrations/user/
├── 20260629120000_add_user_profile.sql
└── atlas.sum
```

### 生成的迁移文件示例

```sql
-- atlas/migrations/user/20260629120000_add_user_profile.sql
-- Add column "email" to table: "user"
ALTER TABLE `user` ADD COLUMN `email` varchar(128) NOT NULL DEFAULT '' COMMENT '邮箱';
-- Create index "idx_user_email" to table: "user"
CREATE INDEX `idx_user_email` ON `user` (`email`);
-- Add column "phone" to table: "user"
ALTER TABLE `user` ADD COLUMN `phone` varchar(20) NOT NULL DEFAULT '' COMMENT '手机号';
CREATE INDEX `idx_user_phone` ON `user` (`phone`);
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

## 环境变量覆盖 DSN

生产部署时通过环境变量覆盖数据库连接，不修改配置文件：

```bash
# User 服务数据库
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

# Gateway 数据库
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local"

# IDGen 数据库
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_idgen?charset=utf8mb4&parseTime=True&loc=Local"
```

::: tip
环境变量路径遵循配置路径的层级结构，将 `.` 替换为 `_` 并全部大写。`app.dbMigration.url` 变为 `EGOADMIN_APP_DBMIGRATION_URL`。
:::

## 多数据库架构

每个服务拥有独立数据库，实现数据隔离：

```text
egoadmin_gateway   -- gateway 服务专属，存储网关配置
egoadmin_user      -- user 服务专属，存储用户、角色、部门、权限、Casbin 策略
egoadmin_idgen     -- idgen 服务专属，存储雪花 ID 分配、号段租约
```

设计原则：

- 服务间不共享数据库，通过 gRPC 通信。
- 每个服务的迁移目录只管理自己数据库的 schema。
- Casbin 策略表 `casbin_rule` 存放在 `egoadmin_user`。
- 如果需要跨服务数据，通过 client 调用而非直连对方数据库。

## 服务器启动时的运行时迁移

`make run` 或容器启动时，服务会自动执行运行时迁移：

```go
// internal/platform/database/mysql/migration.go
func RunMigration(cfg MigrationConfig) error {
  if !cfg.Enabled {
    return nil
  }

  client, err := migrate.NewClient(cfg.Driver, cfg.URL)
  if err != nil {
    return fmt.Errorf("创建迁移客户端失败: %w", err)
  }
  defer client.Close()

  dir, err := migrate.NewLocalDir(cfg.Dir)
  if err != nil {
    return fmt.Errorf("加载迁移目录失败: %w", err)
  }

  return client.Execute(context.Background(), dir, migrate.WithExecOrder(migrate.ExecOrderLinear))
}
```

Server 启动流程：

```text
1. 读取配置文件 + 环境变量覆盖
2. 初始化数据库连接
3. 执行 Atlas 运行时迁移（如果 enabled = true）
4. 初始化组件（AuthSession、IDGen、Redis 等）
5. 注册 gRPC service
6. 启动 HTTP/gRPC 监听
```

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
| 唯一索引冲突 | 数据不满足唯一约束 | 先清理数据再添加唯一索引 |
| 迁移超时 | 大表 ALTER TABLE 锁定时间长 | 使用 online DDL 或在低峰期执行 |
| 模型未注册 | 新 model 未加入 `MigrationModels()` | 检查 `schema.go` 注册列表 |
| 字段类型不匹配 | GORM tag 与数据库实际类型不一致 | 确认 `type:xxx` tag 与数据库兼容 |

### Dirty Database 修复

Atlas 追踪每个迁移版本的执行状态。如果迁移执行中途失败，数据库进入 dirty 状态：

```bash
# 1. 查看当前状态
atlas schema inspect --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --format "{{ range .Realm.Schemas }}{{ .Name }}{{ end }}"

# 2. 手动修复数据库（执行未完成的 SQL 或回滚）

# 3. 标记版本为 clean
atlas schema apply --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --to "file://atlas/schema/mysql/user.hcl" --dev-url "docker://mysql/8/egoadmin_user"

# 4. 重新执行迁移
make run SERVICE=user
```

### 本地开发环境重置

```bash
# 停止中间件
make dev-down

# 清空数据卷
docker volume rm egoadmin_mysql_data

# 重新启动
make dev-up
make run SERVICE=user
```

## 验证命令

```bash
make migrate.new SERVICE=user NAME=example
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
go test ./internal/app/user/adapter/persistence/mysql/...
```

