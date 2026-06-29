# Database & Migrations

EgoAdmin defines schema through GORM models and manages versioned migrations with Atlas.

## Boundaries

| Service | Database | Migration Directory |
|---------|----------|---------------------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

## GORM Model

```go
type UserModel struct {
  ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
  Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex"`
  DeptID    uint64    `gorm:"column:dept_id;not null;default:0;index"`
  Status    int32     `gorm:"column:status;type:tinyint;not null;default:1"`
  CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
  UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string { return "user" }
```

## Register Migration Models

```go
func MigrationModels() []any {
  return []any{
    &mysql.UserModel{},
    &mysql.RoleModel{},
    &mysql.DeptModel{},
  }
}
```

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

## Validation

```bash
make migrate.validate SERVICE=user
make migrate.hash SERVICE=user
go test ./internal/app/user/adapter/persistence/mysql/...
```
