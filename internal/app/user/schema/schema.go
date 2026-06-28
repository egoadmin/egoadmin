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
