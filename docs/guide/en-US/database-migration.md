# Database & Migrations

EgoAdmin defines schema through GORM models and manages versioned migrations with Atlas.

## Boundaries

| Service | Database | Migration Directory |
|---------|----------|---------------------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

## GORM Model

### Model Definition Patterns

```go
// internal/app/user/adapter/persistence/mysql/user_model.go
package mysql

import "time"

type UserModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex;comment:Username"`
  Nickname  string    `gorm:"column:nickname;type:varchar(64);not null;default:'';comment:Nickname"`
  Email     string    `gorm:"column:email;type:varchar(128);default:'';index;comment:Email"`
  Phone     string    `gorm:"column:phone;type:varchar(20);default:'';index;comment:Phone"`
  Avatar    string    `gorm:"column:avatar;type:varchar(512);default:'';comment:Avatar URL"`
  Password  string    `gorm:"column:password;type:varchar(128);not null;comment:Password hash"`
  DeptID    uint64    `gorm:"column:dept_id;not null;default:0;index;comment:Department ID"`
  OwnerUID  uint64    `gorm:"column:owner_user_id;not null;default:0;index;comment:Creator user ID"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:Status 1=active 0=disabled"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string { return "user" }
```

### Role Model

```go
// internal/app/user/adapter/persistence/mysql/role_model.go
type RoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Name      string    `gorm:"column:name;type:varchar(64);not null;uniqueIndex;comment:Role name"`
  Code      string    `gorm:"column:code;type:varchar(64);not null;uniqueIndex;comment:Role code"`
  DataScope int32     `gorm:"column:data_scope;type:tinyint;not null;default:1;comment:Data scope"`
  Sort      int32     `gorm:"column:sort;not null;default:0;comment:Sort order"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:Status"`
  Remark    string    `gorm:"column:remark;type:varchar(255);default:'';comment:Remark"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RoleModel) TableName() string { return "role" }
```

### Department Model

```go
// internal/app/user/adapter/persistence/mysql/dept_model.go
type DeptModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  ParentID  uint64    `gorm:"column:parent_id;not null;default:0;index;comment:Parent dept ID"`
  Name      string    `gorm:"column:name;type:varchar(64);not null;comment:Department name"`
  Sort      int32     `gorm:"column:sort;not null;default:0;comment:Sort order"`
  Status    int32    `gorm:"column:status;type:tinyint;not null;default:1;comment:Status"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (DeptModel) TableName() string { return "dept" }
```

Model rules:

- Table name must be explicitly declared via `TableName()`.
- Every field must specify `column`, type, index, default, and comment.
- Service models live under `internal/app/<service>/adapter/persistence/mysql`.
- Join tables and Casbin tables are also exported through the service schema list.
- `owner_user_id` field is used by DataScope "self" permission filtering.

## Register Migration Models

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

### Join Table Definitions

Join tables are explicitly modeled rather than using GORM `many2many` tags:

```go
// User-role association
type UserRoleModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  UserID    uint64    `gorm:"column:user_id;not null;index:idx_user_role,unique;comment:User ID"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_user_role,unique;comment:Role ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserRoleModel) TableName() string { return "user_role" }

// Role-menu association (permission grants)
type RoleMenuModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_role_menu,unique;comment:Role ID"`
  MenuID    uint64    `gorm:"column:menu_id;not null;index:idx_role_menu,unique;comment:Menu ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RoleMenuModel) TableName() string { return "role_menu" }

// Role-department association (custom DataScope)
type RoleDeptModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  RoleID    uint64    `gorm:"column:role_id;not null;index:idx_role_dept,unique;comment:Role ID"`
  DeptID    uint64    `gorm:"column:dept_id;not null;index:idx_role_dept,unique;comment:Department ID"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (RoleDeptModel) TableName() string { return "role_dept" }
```

::: tip
Join tables must be registered in `MigrationJoinTables()` to be exported by Atlas. Composite unique indexes prevent duplicate associations.
:::

## Atlas Migration Workflow

### Full Workflow Example

```bash
# 1. Ensure toolchain is installed
make install

# 2. Generate proto and Wire code
make gen

# 3. Modify GORM model (e.g., add Profile field)
# Edit internal/app/user/adapter/persistence/mysql/user_model.go

# 4. Register new model in schema (if new table)
# Edit internal/app/user/schema/schema.go

# 5. Generate migration SQL
make migrate.new SERVICE=user NAME=add_user_profile

# 6. Inspect generated SQL
cat atlas/migrations/user/20260629120000_add_user_profile.sql

# 7. Validate migration syntax
make migrate.validate SERVICE=user

# 8. Update atlas.sum checksum
make migrate.hash SERVICE=user

# 9. Run tests
go test ./internal/app/user/...

# 10. Apply migration locally (auto-applied when using make run)
make dev-up
make run SERVICE=user
```

Output:

```text
atlas/migrations/user/
├── 20260629120000_add_user_profile.sql
└── atlas.sum
```

### Example Generated Migration

```sql
-- atlas/migrations/user/20260629120000_add_user_profile.sql
-- Add column "email" to table: "user"
ALTER TABLE `user` ADD COLUMN `email` varchar(128) NOT NULL DEFAULT '' COMMENT 'Email';
CREATE INDEX `idx_user_email` ON `user` (`email`);
-- Add column "phone" to table: "user"
ALTER TABLE `user` ADD COLUMN `phone` varchar(20) NOT NULL DEFAULT '' COMMENT 'Phone';
CREATE INDEX `idx_user_phone` ON `user` (`phone`);
```

### Schema Snapshots

Used for review, not deployment:

```bash
make migrate.schema SERVICE=user
make migrate.schema SERVICE=user DIALECT=postgres
```

Output:

```text
atlas/schema/mysql/user.hcl
atlas/schema/postgres/user.hcl
```

### Multi-Dialect Support

```bash
go run ./tools/atlasloader --service user --dialect mysql
go run ./tools/atlasloader --service user --dialect postgres
go run ./tools/atlasloader --service user --dialect sqlite
go run ./tools/atlasloader --service user --dialect sqlserver
```

Generate PostgreSQL migration:

```bash
make migrate.new SERVICE=user NAME=add_field DIALECT=postgres
```

Directory: `atlas/migrations/postgres/user`

## Generate Migrations

```bash
make install
make gen
make migrate.new SERVICE=user NAME=add_user_profile
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
```

## Runtime Migration Config

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

`AutoMigrate` should remain disabled for normal deployments:

```toml
[app.service]
autoMigrate = false
```

## Environment Variable Override for DSN

In production, override the database connection via environment variables:

```bash
# User service database
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"

# Gateway database
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local"

# IDGen database
EGOADMIN_APP_DBMIGRATION_URL="mysql://prod_user:prod_pass@db-host:3306/egoadmin_idgen?charset=utf8mb4&parseTime=True&loc=Local"
```

::: tip
Environment variable paths follow the config hierarchy: replace `.` with `_` and uppercase. `app.dbMigration.url` becomes `EGOADMIN_APP_DBMIGRATION_URL`.
:::

## Multi-Database Architecture

Each service owns its own database for data isolation:

```text
egoadmin_gateway   -- gateway service only, stores gateway config
egoadmin_user      -- user service only, stores users, roles, depts, permissions, Casbin policies
egoadmin_idgen     -- idgen service only, stores snowflake ID allocation, segment leases
```

Design principles:

- Services do not share databases; communicate via gRPC.
- Each service's migration directory manages only its own database schema.
- Casbin policy table `casbin_rule` lives in `egoadmin_user`.
- For cross-service data, use client calls rather than connecting directly to another service's database.

## Runtime Migration at Server Startup

When running `make run` or starting a container, runtime migration executes automatically:

```go
// internal/platform/database/mysql/migration.go
func RunMigration(cfg MigrationConfig) error {
  if !cfg.Enabled {
    return nil
  }

  client, err := migrate.NewClient(cfg.Driver, cfg.URL)
  if err != nil {
    return fmt.Errorf("failed to create migration client: %w", err)
  }
  defer client.Close()

  dir, err := migrate.NewLocalDir(cfg.Dir)
  if err != nil {
    return fmt.Errorf("failed to load migration directory: %w", err)
  }

  return client.Execute(context.Background(), dir, migrate.WithExecOrder(migrate.ExecOrderLinear))
}
```

Server startup sequence:

```text
1. Load config file + environment variable overrides
2. Initialize database connection
3. Execute Atlas runtime migration (if enabled = true)
4. Initialize components (AuthSession, IDGen, Redis, etc.)
5. Register gRPC services
6. Start HTTP/gRPC listeners
```

## Query Patterns

Repository queries use context for timeout and cancellation:

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

## Transaction Patterns

Single-service transactions are orchestrated in the application layer:

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

For cross-service writes, consider DTM.

## Common Migration Issues

| Problem | Cause | Solution |
|---------|-------|----------|
| Atlas dirty database | Previous migration failed | Fix database state, then re-apply |
| atlas.sum mismatch | Manually edited migration file | Verify SQL, then run `make migrate.hash SERVICE=user` |
| Wrong migration directory | SERVICE does not match database | Check `app.dbMigration.dir` |
| Table already exists | AutoMigrate or manual DDL caused drift | Drop local DB or write explicit migration |
| Unique index conflict | Data violates unique constraint | Clean data before adding unique index |
| Migration timeout | Large table ALTER TABLE holds lock | Use online DDL or run during low traffic |
| Model not registered | New model not in `MigrationModels()` | Check `schema.go` registration list |
| Column type mismatch | GORM tag does not match actual DB type | Verify `type:xxx` tag is database-compatible |

### Dirty Database Recovery

Atlas tracks each migration version's execution state. If a migration fails midway, the database enters dirty state:

```bash
# 1. Inspect current state
atlas schema inspect --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"

# 2. Manually fix the database (complete the failed SQL or rollback)

# 3. Re-apply schema
atlas schema apply --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --to "file://atlas/schema/mysql/user.hcl" --dev-url "docker://mysql/8/egoadmin_user"

# 4. Re-run migrations
make run SERVICE=user
```

### Local Development Reset

```bash
# Stop middleware
make dev-down

# Remove data volumes
docker volume rm egoadmin_mysql_data

# Restart
make dev-up
make run SERVICE=user
```

## Validation

```bash
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
go test ./internal/app/user/adapter/persistence/mysql/...
```
