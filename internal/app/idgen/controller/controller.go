package controller

import (
	"github.com/egoadmin/egoadmin/internal/app/idgen/application"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSegmentGRPC,
	NewMachineLeaseGRPC,
)

type Options struct {
	Segment      *SegmentGRPC
	MachineLease *MachineLeaseGRPC
}

type SegmentGRPC struct {
	usecase *application.SegmentUseCase
}

func NewSegmentGRPC(usecase *application.SegmentUseCase) *SegmentGRPC {
	return &SegmentGRPC{usecase: usecase}
}

type MachineLeaseGRPC struct {
	usecase *application.MachineLeaseUseCase
}

func NewMachineLeaseGRPC(usecase *application.MachineLeaseUseCase) *MachineLeaseGRPC {
	return &MachineLeaseGRPC{usecase: usecase}
}
