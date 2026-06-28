package schema

import (
	idgenmysql "github.com/egoadmin/egoadmin/internal/app/idgen/adapter/persistence/mysql"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

func MigrationModels() []any {
	return idgenmysql.MigrationModels()
}

func MigrationJoinTables() []platformmysql.MigrationJoinTable {
	return idgenmysql.MigrationJoinTables()
}
