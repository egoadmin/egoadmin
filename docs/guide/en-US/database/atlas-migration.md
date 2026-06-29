# Atlas Migration Management

EgoAdmin uses Atlas for database schema migrations. Each service owns its own database boundary with independent migration files.

## Overview

EgoAdmin's migration architecture follows the "one service, one database" principle. Each service (gateway, user, idgen) has its own database instance and Atlas migration directory. Services cannot directly access each other's databases.

| Service | Database | Migration Directory |
|---------|----------|---------------------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

Core design principles:

- GORM Models are the schema source. Atlas migrations are the deployment source.
- Migration directories must map one-to-one with service databases. No cross-service access.
- Production relies on Atlas versioned migrations, not GORM AutoMigrate.
- AutoMigrate is only a fallback for local development.

::: warning
Cross-service writes should go through DTM distributed transactions. Do not bypass database boundaries.
:::

## Core Usage

### Create a New Migration

Use `make migrate.new` to generate a versioned migration file for a given service:

```bash
make migrate.new SERVICE=user NAME=add_avatar_field
```

This command:

1. Runs `tools/atlasloader` to export GORM Models into an Atlas-readable schema.
2. Calls `atlas migrate diff` to compare the current schema against existing migrations and generate a new SQL file.
3. Automatically updates the `atlas.sum` hash file.

Example output:

```text
atlas/migrations/user/
├── 20260622213810_user_initial.sql
├── 20260625180857_user_data_scope.sql
├── 20260629120000_add_avatar_field.sql   <-- newly generated
└── atlas.sum
```

::: tip
Migration filenames include a timestamp prefix. Atlas applies them in chronological order. Do not manually edit timestamps.
:::

### Validate Migrations

Verify SQL syntax and directory integrity before committing:

```bash
# Validate migration SQL syntax
make migrate.validate SERVICE=user

# Recalculate hash (required after manual SQL edits)
make migrate.hash SERVICE=user
```

::: danger
After manually editing `.sql` files, you must run `migrate.hash`. Otherwise Atlas will fail the hash check on apply.
:::

### Apply Migrations

Apply migrations to the target database:

```bash
make migrate.apply SERVICE=user ATLAS_URL="mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"
```

The `ATLAS_URL` parameter is required, formatted as an Atlas-compatible database URL.

::: warning
Always confirm the URL points to the correct environment before running apply. Back up production databases before irreversible migrations.
:::

### Generate Schema Snapshot

Audit and review the current final schema state (not for deployment):

```bash
# MySQL dialect (default)
make migrate.schema SERVICE=user

# PostgreSQL dialect
make migrate.schema SERVICE=user DIALECT=postgres
```

Output goes to `atlas/schema/<dialect>/<service>.hcl`.

### Multi-Dialect Support

EgoAdmin supports MySQL, PostgreSQL, SQLite, and SQL Server:

```bash
# Generate PostgreSQL migration
make migrate.new SERVICE=user NAME=add_field DIALECT=postgres

# Generate SQLite migration
make migrate.new SERVICE=user NAME=add_field DIALECT=sqlite
```

Non-MySQL migration directories use the structure `atlas/migrations/<dialect>/<service>/`.

## Configuration Examples

### Service Config File

Configure runtime migration parameters in `configs/<service>/config.toml`:

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

Configuration reference:

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable runtime migration. Set to `false` to skip |
| `driver` | string | Migration driver. Currently only `atlas` is supported |
| `url` | string | Database URL. Supports `$ENV_VAR` expansion |
| `dir` | string | Migration directory. Format: `file://<path>` |
| `bin` | string | Atlas binary path. Defaults to `atlas` |

### Environment Variable Overrides

Override config file values with `EGOADMIN_*` prefixed environment variables, useful for containerized deployments:

```bash
# Enable migration
export EGOADMIN_APP_DBMIGRATION_ENABLED=true

# Override database URL
export EGOADMIN_APP_DBMIGRATION_URL="mysql://egoadmin:secret@db-host:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

### Skipping Migration

Some scenarios (testing, local dev with manual schema) require skipping migration:

```bash
# Skip Atlas migration via environment variable
export EGOADMIN_ATLAS_MIGRATED=true
```

::: info
`EGOADMIN_ATLAS_MIGRATED=true` skips the `ApplyAtlas` call entirely. It takes priority over the `enabled` field in config.
:::

### GORM AutoMigrate Config

```toml
[app.service]
autoMigrate = false
```

::: warning
`autoMigrate` must remain `false` in production. AutoMigrate is only for rapid local prototyping. Its DDL output is not version-controlled.
:::

## Real-World Examples

### Full Migration Workflow

The following demonstrates adding an `avatar` field to the user service end-to-end:

**Step 1: Update the GORM Model**

```go
// internal/app/user/internal/store/user_model.go
type UserModel struct {
    ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
    Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex;comment:用户名"`
    Avatar    string    `gorm:"column:avatar;type:varchar(512);not null;default:'';comment:头像URL"` // new field
    Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
    CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string {
    return "user"
}
```

**Step 2: Ensure the Model Is Registered**

```go
// internal/app/user/internal/store/schema.go
func MigrationModels() []any {
    return []any{
        &UserModel{},
        &RoleModel{},
        &DeptModel{},
        &AuditLogModel{},
        &casbinadapter.CasbinRule{},
    }
}
```

::: tip
If a new model is not registered in `MigrationModels()`, Atlas cannot detect the table. The migration will not create it.
:::

**Step 3: Generate and Validate**

```bash
# Generate code (including Wire injection, etc.)
make gen

# Generate migration
make migrate.new SERVICE=user NAME=add_avatar_field

# Validate
make migrate.validate SERVICE=user

# Generate hash
make migrate.hash SERVICE=user
```

**Step 4: Apply**

```bash
# Local development
make migrate.apply SERVICE=user ATLAS_URL="mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"

# Verify
go test ./internal/app/user/adapter/persistence/mysql/...
```

### Runtime Migration Internals

Migrations are triggered at service startup via the Wire dependency injection chain. Here is the user service's `newSchemaReady` function:

```go
// internal/app/user/server/server.go
type schemaReady struct{}

func newSchemaReady(conf *config.Config, db mysql.MysqlInterface) (schemaReady, error) {
    // Step 1: Execute Atlas versioned migration
    if err := migration.ApplyAtlas(context.Background(), conf.DBMigration(), "file://atlas/migrations/user"); err != nil {
        return schemaReady{}, err
    }

    // Step 2: Only run GORM AutoMigrate when autoMigrate=true
    if conf.App().AutoMigrate {
        if err := db.Migrate(context.Background(), schema.MigrationModels(), schema.MigrationJoinTables()); err != nil {
            return schemaReady{}, err
        }
    }

    return schemaReady{}, nil
}
```

The `schemaReady` struct acts as a readiness marker. Downstream components (such as Casbin initialization) depend on it:

```go
func newCasbin(cc *egorm.Component, _ schemaReady) (*perm.Casbin, error) {
    // When schemaReady is consumed, database tables are guaranteed to exist
    // ...
}
```

::: info
`ApplyAtlas` internally invokes `atlas migrate apply --url <url> --dir <dir>`. It checks `EGOADMIN_ATLAS_MIGRATED` and the `enabled` config field at startup, skipping entirely if either condition is met.
:::

### Schema Registration Structure

Each service's schema package aggregates all models that need migration:

```go
// internal/app/user/schema/schema.go
package schema

import (
    "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
    "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

func MigrationModels() []any {
    return store.MigrationModels()
}

func MigrationJoinTables() []mysql.MigrationJoinTable {
    return store.MigrationJoinTables()
}
```

Model registration hierarchy:

```text
schema/schema.go          <- service entry point, referenced by server.go
  └── store/schema.go     <- actual model registration with all Models and JoinTables
        ├── UserModel
        ├── RoleModel
        ├── DeptModel
        └── casbinadapter.CasbinRule
```

### Migration Directory Layout

```
atlas/
├── atlas.hcl                    # Atlas global config
├── migrations/
│   ├── gateway/
│   │   ├── 20260622213810_gateway_initial.sql
│   │   └── atlas.sum
│   ├── user/
│   │   ├── 20260622213810_user_initial.sql
│   │   ├── 20260625180857_user_data_scope.sql
│   │   └── atlas.sum
│   └── idgen/
│       ├── 20260624214500_idgen_initial.sql
│       └── atlas.sum
└── schema/
    ├── mysql/
    │   ├── user.hcl
    │   └── idgen.hcl
    └── postgres/
        └── user.hcl
```

Key files:

- `*.sql` - Versioned migration files containing UP-direction DDL statements.
- `atlas.sum` - Directory hash verification file that prevents migration file tampering.
- `atlas.hcl` - Atlas global config defining environments, variables, and dev URLs.

## How It Works

### ApplyAtlas Execution Flow

```go
// internal/platform/database/mysql/migration/apply.go
func ApplyAtlas(ctx context.Context, conf config.DBMigrationConf, defaultDir string) error {
    // 1. Check environment variable skip flag
    if skipped, _ := strconv.ParseBool(os.Getenv("EGOADMIN_ATLAS_MIGRATED")); skipped {
        return nil
    }
    // 2. Check if migration is enabled in config
    if !conf.Enabled {
        return nil
    }
    // 3. Validate driver type
    if conf.Driver != "" && !strings.EqualFold(conf.Driver, "atlas") {
        return fmt.Errorf("unsupported db migration driver %q", conf.Driver)
    }
    // 4. Resolve URL (supports env var expansion)
    url := os.ExpandEnv(conf.URL)
    if url == "" {
        url = os.Getenv("ATLAS_URL")
    }
    // 5. Execute atlas migrate apply command
    cmd := exec.CommandContext(ctx, bin, "migrate", "apply", "--url", url, "--dir", dir)
    return cmd.Run()
}
```

Execution chain:

```text
Service startup
  -> Wire constructs newSchemaReady()
    -> migration.ApplyAtlas()
      -> exec: atlas migrate apply --url <url> --dir <dir>
        -> Atlas verifies directory integrity against atlas.sum
        -> Applies pending .sql files in timestamp order
        -> Records applied versions in database table schema_atlas.schema_migration
```

### URL Resolution Priority

The database URL in `ApplyAtlas` is resolved in this order:

1. Config file `app.dbMigration.url` (supports `$ENV_VAR` expansion)
2. Environment variable `ATLAS_URL`
3. If both are empty, the function returns an error

::: info
For containerized deployments, inject the database address via `EGOADMIN_APP_DBMIGRATION_URL`. Do not hardcode connection strings in images.
:::

## Common Issues

### Dirty Database State

**Symptom**: Atlas reports a `dirty database` error and refuses to apply further migrations.

**Cause**: A previous migration failed midway, leaving the database in an inconsistent state.

**Fix**:

```bash
# 1. Check current migration version
atlas migrate status --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --dir "file://atlas/migrations/user"

# 2. Manually fix the database (rollback or complete the SQL)

# 3. Mark the version as clean
atlas migrate set --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --dir "file://atlas/migrations/user" <version>
```

### atlas.sum Hash Mismatch

**Symptom**: `atlas migrate apply` fails with a hash verification error.

**Cause**: A `.sql` file was manually edited without recalculating the hash.

**Fix**:

```bash
make migrate.hash SERVICE=user
```

::: danger
After manually editing migration SQL, you must run `migrate.hash`. Failing to do so will break all subsequent apply operations.
:::

### Migration Directory Mismatch

**Symptom**: Migration operates on the wrong service's database.

**Cause**: The `SERVICE` parameter and `app.dbMigration.dir` config point to different migration directories.

**Fix**:

- Check the `dir` field in `configs/<service>/config.toml`.
- Verify the mapping between the `SERVICE` parameter and the database name.
- The default `dir` is `file://atlas/migrations/<service>`. You usually do not need to set it manually.

### New Table Not Created

**Symptom**: Migration succeeds but the database does not contain the new table.

**Cause**: The GORM Model is not registered in `MigrationModels()` or `MigrationJoinTables()`.

**Fix**:

```go
// Make sure all models needing migration are in MigrationModels()
func MigrationModels() []any {
    return []any{
        &UserModel{},
        &NewModel{},      // <-- add the new model here
    }
}
```

### Production Mistake

**Symptom**: Wrong migration executed against production database.

**Prevention**:

- Always validate migrations in a test environment first.
- Inject `ATLAS_URL` via environment variables. Never hardcode production addresses.
- Back up the database before high-risk operations.
- Add `make migrate.validate` as a gate in your CI/CD pipeline.

### Migration Skipped at Startup

**Symptom**: Service starts without errors but database tables do not exist.

**Cause**: `enabled = false` or `EGOADMIN_ATLAS_MIGRATED=true` is set.

**Fix**:

```bash
# Check config
grep -r "dbMigration" configs/<service>/

# Check environment variables
echo $EGOADMIN_ATLAS_MIGRATED
echo $EGOADMIN_APP_DBMIGRATION_ENABLED
```

## Reference Links

- [Atlas Official Documentation](https://atlasgo.io/getting-started)
- [Atlas migrate apply](https://atlasgo.io/versioned/diff)
- [Atlas GORM Integration](https://atlasgo.io/orms/gorm)
- [EgoAdmin Database & Migrations](./database-migration.md)
- [EgoAdmin Architecture Overview](/guide/en-US/architecture)
