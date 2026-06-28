package mysql

import (
	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewUserRepository,
	NewRoleRepository,
	NewDeptRepository,
	wire.Bind(new(userdomain.Repository), new(*UserRepository)),
	wire.Bind(new(roledomain.Repository), new(*RoleRepository)),
	wire.Bind(new(deptdomain.Repository), new(*DeptRepository)),
)
