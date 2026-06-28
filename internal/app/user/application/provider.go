package application

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserUseCase,
	NewRoleUseCase,
	NewDeptUseCase,
	wire.Struct(new(UserOptions), "*"),
	wire.Struct(new(RoleOptions), "*"),
	wire.Struct(new(DeptOptions), "*"),
)
