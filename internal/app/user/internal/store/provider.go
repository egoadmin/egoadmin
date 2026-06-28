package store

import (
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewLog,
	NewDept,
	NewUser,
	NewRole,
	NewConfig,
	NewAuthCryptoKey,
)

func MigrationModels() []any {
	return []any{
		&LogModel{},
		&RoleModel{},
		&RolePermissionPolicy{},
		&UserModel{},
		&UserRole{},
		&DeptModel{},
		&ConfigModel{},
		&AuthCryptoKeyModel{},
		&gormadapter.CasbinRule{},
	}
}

func MigrationJoinTables() []mysql.MigrationJoinTable {
	return []mysql.MigrationJoinTable{
		{
			Model: &UserModel{},
			Field: "Roles",
			Table: &UserRole{},
		},
	}
}
