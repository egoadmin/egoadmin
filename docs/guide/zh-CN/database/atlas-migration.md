# Atlas 迁移管理

EgoAdmin 使用 Atlas 管理数据库 schema 迁移，每个服务拥有独立的数据库边界和迁移文件。

## 概览

EgoAdmin 的数据库迁移架构遵循"一服务一库"原则。每个服务（gateway、user、idgen）各自拥有独立的数据库实例和 Atlas 迁移目录，服务之间不能直接访问对方的数据库。

| 服务 | 数据库 | 迁移目录 |
|------|--------|----------|
| gateway | `egoadmin_gateway` | `atlas/migrations/gateway` |
| user | `egoadmin_user` | `atlas/migrations/user` |
| idgen | `egoadmin_idgen` | `atlas/migrations/idgen` |

核心设计原则：

- GORM Model 是 schema 源，Atlas migration 是部署源。
- 迁移目录必须与服务数据库一一对应，不可交叉使用。
- 生产环境依赖 Atlas 版本化迁移，不依赖 GORM AutoMigrate。
- AutoMigrate 仅作为本地开发的兜底能力。

::: warning
跨服务写入应通过 DTM 分布式事务协调，不要试图绕过数据库边界。
:::

## 核心用法

### 创建新迁移

使用 `make migrate.new` 为指定服务生成版本化迁移文件：

```bash
make migrate.new SERVICE=user NAME=add_avatar_field
```

执行后会：

1. 运行 `tools/atlasloader` 将 GORM Model 导出为 Atlas 可读的 schema。
2. 调用 `atlas migrate diff` 对比当前 schema 与已有迁移，生成新的 SQL 文件。
3. 自动更新 `atlas.sum` 哈希文件。

输出示例：

```text
atlas/migrations/user/
├── 20260622213810_user_initial.sql
├── 20260625180857_user_data_scope.sql
├── 20260629120000_add_avatar_field.sql   <-- 新生成
└── atlas.sum
```

::: tip
迁移文件名包含时间戳前缀，Atlas 按时间顺序依次应用。不要手动修改时间戳。
:::

### 校验迁移

在提交迁移前，验证 SQL 语法和目录完整性：

```bash
# 校验迁移 SQL 语法
make migrate.validate SERVICE=user

# 重新计算哈希值（手动编辑 SQL 后必须执行）
make migrate.hash SERVICE=user
```

::: danger
手动编辑 `.sql` 文件后如果不执行 `migrate.hash`，Atlas 会在 apply 时报哈希校验失败。
:::

### 应用迁移

将迁移应用到目标数据库：

```bash
make migrate.apply SERVICE=user ATLAS_URL="mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"
```

`ATLAS_URL` 参数是必填项，格式为 Atlas 兼容的数据库连接 URL。

::: warning
执行 apply 前请确认 URL 指向正确的环境。对生产库执行不可逆迁移前务必备份。
:::

### 生成 schema 快照

用于审计和 review 当前最终 schema 状态（不用于部署）：

```bash
# MySQL 方言（默认）
make migrate.schema SERVICE=user

# PostgreSQL 方言
make migrate.schema SERVICE=user DIALECT=postgres
```

输出到 `atlas/schema/<dialect>/<service>.hcl`。

### 多方言支持

EgoAdmin 支持 MySQL、PostgreSQL、SQLite 和 SQL Server 四种方言：

```bash
# 生成 PostgreSQL 迁移
make migrate.new SERVICE=user NAME=add_field DIALECT=postgres

# 生成 SQLite 迁移
make migrate.new SERVICE=user NAME=add_field DIALECT=sqlite
```

非 MySQL 方言的迁移目录结构为 `atlas/migrations/<dialect>/<service>/`。

## 配置示例

### 服务配置文件

在 `configs/<service>/config.toml` 中配置运行时迁移参数：

```toml
[app.dbMigration]
enabled = true
driver = "atlas"
url = "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
dir = "file://atlas/migrations/user"
bin = "atlas"
```

各配置项说明：

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `enabled` | bool | 是否启用运行时迁移。设为 `false` 跳过 |
| `driver` | string | 迁移驱动，当前仅支持 `atlas` |
| `url` | string | 数据库连接 URL，支持 `$ENV_VAR` 展开 |
| `dir` | string | 迁移文件目录，格式 `file://<path>` |
| `bin` | string | Atlas 可执行文件路径，默认 `atlas` |

### 环境变量覆盖

通过 `EGOADMIN_*` 前缀的环境变量覆盖配置文件，适用于容器化部署场景：

```bash
# 启用迁移
export EGOADMIN_APP_DBMIGRATION_ENABLED=true

# 覆盖数据库连接 URL
export EGOADMIN_APP_DBMIGRATION_URL="mysql://egoadmin:secret@db-host:3306/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local"
```

### 跳过迁移

某些场景（如测试、本地开发已手动建表）下可跳过迁移：

```bash
# 通过环境变量跳过 Atlas 迁移
export EGOADMIN_ATLAS_MIGRATED=true
```

::: info
`EGOADMIN_ATLAS_MIGRATED=true` 会跳过 `ApplyAtlas` 调用，优先级高于配置文件中的 `enabled` 字段。
:::

### GORM AutoMigrate 配置

```toml
[app.service]
autoMigrate = false
```

::: warning
生产部署中 `autoMigrate` 必须保持 `false`。AutoMigrate 仅用于本地开发快速原型验证，生成的 DDL 不具备版本控制能力。
:::

## 实战示例

### 完整迁移流程

以下演示为 user 服务添加 `avatar` 字段的完整流程：

**第一步：编写 GORM Model**

```go
// internal/app/user/internal/store/user_model.go
type UserModel struct {
    ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
    Username  string    `gorm:"column:username;type:varchar(64);not null;uniqueIndex;comment:用户名"`
    Avatar    string    `gorm:"column:avatar;type:varchar(512);not null;default:'';comment:头像URL"` // 新增字段
    Status    int32     `gorm:"column:status;type:tinyint;not null;default:1;comment:状态"`
    CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserModel) TableName() string {
    return "user"
}
```

**第二步：确保模型已注册到迁移清单**

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
如果新模型没有注册到 `MigrationModels()`，Atlas 无法感知该表，迁移不会创建对应的数据表。
:::

**第三步：生成并校验迁移**

```bash
# 生成代码（包括 Wire 注入等）
make gen

# 生成迁移
make migrate.new SERVICE=user NAME=add_avatar_field

# 校验
make migrate.validate SERVICE=user

# 生成哈希
make migrate.hash SERVICE=user
```

**第四步：应用迁移**

```bash
# 本地开发
make migrate.apply SERVICE=user ATLAS_URL="mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user"

# 验证
go test ./internal/app/user/adapter/persistence/mysql/...
```

### 运行时迁移原理

服务启动时通过 Wire 依赖注入链触发迁移。以下是 user 服务的 `newSchemaReady` 函数：

```go
// internal/app/user/server/server.go
type schemaReady struct{}

func newSchemaReady(conf *config.Config, db mysql.MysqlInterface) (schemaReady, error) {
    // 第一步：执行 Atlas 版本化迁移
    if err := migration.ApplyAtlas(context.Background(), conf.DBMigration(), "file://atlas/migrations/user"); err != nil {
        return schemaReady{}, err
    }

    // 第二步：仅在 autoMigrate=true 时执行 GORM AutoMigrate
    if conf.App().AutoMigrate {
        if err := db.Migrate(context.Background(), schema.MigrationModels(), schema.MigrationJoinTables()); err != nil {
            return schemaReady{}, err
        }
    }

    return schemaReady{}, nil
}
```

`schemaReady` 结构体作为就绪标志，被下游组件（如 Casbin 初始化）依赖：

```go
func newCasbin(cc *egorm.Component, _ schemaReady) (*perm.Casbin, error) {
    // schemaReady 被消费时，数据库表已就绪
    // ...
}
```

::: info
`ApplyAtlas` 内部调用 `atlas migrate apply --url <url> --dir <dir>` 命令行。它会在启动时检查 `EGOADMIN_ATLAS_MIGRATED` 环境变量和 `enabled` 配置，两者都不满足时直接跳过。
:::

### Schema 注册结构

各服务的 schema 包负责聚合所有需要迁移的模型：

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

迁移模型的分层关系：

```text
schema/schema.go          <- 服务入口，被 server.go 引用
  └── store/schema.go     <- 实际模型注册，包含所有 Model 和 JoinTable
        ├── UserModel
        ├── RoleModel
        ├── DeptModel
        └── casbinadapter.CasbinRule
```

### 迁移目录结构

```
atlas/
├── atlas.hcl                    # Atlas 全局配置
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

关键文件说明：

- `*.sql`：版本化迁移文件，包含 UP 方向的 DDL 语句。
- `atlas.sum`：目录哈希校验文件，防止迁移文件被篡改。
- `atlas.hcl`：Atlas 全局配置，定义环境、变量和 dev URL。

## 工作原理

### ApplyAtlas 执行流程

```go
// internal/platform/database/mysql/migration/apply.go
func ApplyAtlas(ctx context.Context, conf config.DBMigrationConf, defaultDir string) error {
    // 1. 检查环境变量跳过标记
    if skipped, _ := strconv.ParseBool(os.Getenv("EGOADMIN_ATLAS_MIGRATED")); skipped {
        return nil
    }
    // 2. 检查配置是否启用
    if !conf.Enabled {
        return nil
    }
    // 3. 校验驱动类型
    if conf.Driver != "" && !strings.EqualFold(conf.Driver, "atlas") {
        return fmt.Errorf("unsupported db migration driver %q", conf.Driver)
    }
    // 4. 解析 URL（支持环境变量展开）
    url := os.ExpandEnv(conf.URL)
    if url == "" {
        url = os.Getenv("ATLAS_URL")
    }
    // 5. 执行 atlas migrate apply 命令
    cmd := exec.CommandContext(ctx, bin, "migrate", "apply", "--url", url, "--dir", dir)
    return cmd.Run()
}
```

执行链路：

```text
服务启动
  -> Wire 构造 newSchemaReady()
    -> migration.ApplyAtlas()
      -> exec: atlas migrate apply --url <url> --dir <dir>
        -> Atlas 比对 atlas.sum 校验目录完整性
        -> 按时间戳顺序依次执行未应用的 .sql 文件
        -> 记录已应用版本到数据库 schema_atlas.schema_migration 表
```

### URL 解析优先级

`ApplyAtlas` 的数据库 URL 按以下优先级解析：

1. 配置文件 `app.dbMigration.url`（支持 `$ENV_VAR` 展开）
2. 环境变量 `ATLAS_URL`
3. 两者都为空时报错退出

::: info
容器化部署时推荐通过 `EGOADMIN_APP_DBMIGRATION_URL` 注入数据库地址，避免在镜像中硬编码连接信息。
:::

## 常见问题

### Dirty database 状态

**现象**：Atlas 报 `dirty database` 错误，无法继续 apply。

**原因**：上一次迁移执行中途失败，数据库处于不一致状态。

**处理**：

```bash
# 1. 连接数据库查看当前迁移版本
atlas migrate status --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --dir "file://atlas/migrations/user"

# 2. 手动修复数据库（回滚或补全 SQL）

# 3. 标记版本为 clean
atlas migrate set --url "mysql://egoadmin:egoadmin@127.0.0.1:3307/egoadmin_user" --dir "file://atlas/migrations/user" <version>
```

### atlas.sum 哈希不匹配

**现象**：`atlas migrate apply` 报哈希校验失败。

**原因**：手动编辑了 `.sql` 文件但没有更新哈希。

**处理**：

```bash
make migrate.hash SERVICE=user
```

::: danger
手动编辑迁移 SQL 后必须执行 `migrate.hash`。不执行会导致所有后续 apply 操作失败。
:::

### 迁移目录与服务不匹配

**现象**：迁移操作了错误服务的数据库。

**原因**：`SERVICE` 参数与 `app.dbMigration.dir` 配置指向不同的迁移目录。

**处理**：

- 检查 `configs/<service>/config.toml` 中的 `dir` 字段。
- 确认 `SERVICE` 参数和数据库名称对应关系。
- `dir` 默认值为 `file://atlas/migrations/<service>`，无需手动配置。

### 新表未创建

**现象**：迁移执行成功但数据库中没有新表。

**原因**：GORM Model 没有注册到 `MigrationModels()` 或 `MigrationJoinTables()`。

**处理**：

```go
// 确保在 MigrationModels() 中包含所有需要迁移的模型
func MigrationModels() []any {
    return []any{
        &UserModel{},
        &NewModel{},      // <-- 添加新模型
    }
}
```

### 生产环境误操作

**现象**：对生产库执行了错误的迁移。

**预防措施**：

- 始终先在测试环境验证迁移。
- `ATLAS_URL` 使用环境变量注入，不要硬编码生产地址。
- 高风险操作前备份数据库。
- CI/CD 流程中加入 `make migrate.validate` 卡点。

### 服务启动跳过迁移

**现象**：服务启动后数据库表不存在，但没有报错。

**原因**：`enabled = false` 或设置了 `EGOADMIN_ATLAS_MIGRATED=true`。

**处理**：

```bash
# 检查配置
grep -r "dbMigration" configs/<service>/

# 检查环境变量
echo $EGOADMIN_ATLAS_MIGRATED
echo $EGOADMIN_APP_DBMIGRATION_ENABLED
```

## 参考链接

- [Atlas 官方文档](https://atlasgo.io/getting-started)
- [Atlas migrate apply](https://atlasgo.io/versioned/diff)
- [Atlas GORM 集成](https://atlasgo.io/orms/gorm)
- [EgoAdmin 数据库与迁移](./database-migration.md)
- [EgoAdmin 架构概览](/guide/zh-CN/architecture)
