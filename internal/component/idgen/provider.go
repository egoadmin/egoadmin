package idgen

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewIDGenComponent,
)

func NewIDGenComponent(store SegmentStore, manager MachineLeaseManager) *Component {
	return Load(defaultComponentName).Build(
		WithSegmentStore(store),
		WithMachineLeaseManager(manager),
	)
}
