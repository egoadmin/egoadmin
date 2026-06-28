package application

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewSegmentUseCase,
	NewMachineLeaseUseCase,
)
