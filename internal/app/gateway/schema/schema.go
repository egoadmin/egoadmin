package schema

import (
	gatewaymysql "github.com/egoadmin/egoadmin/internal/app/gateway/adapter/persistence/mysql"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

func MigrationModels() []any {
	return gatewaymysql.MigrationModels()
}

func MigrationJoinTables() []platformmysql.MigrationJoinTable {
	return gatewaymysql.MigrationJoinTables()
}
