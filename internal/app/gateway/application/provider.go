package application

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAPIUseCase,
	NewPermissionUseCase,
	wire.Struct(new(APIOptions), "*"),
	wire.Struct(new(PermissionOptions), "*"),
)
