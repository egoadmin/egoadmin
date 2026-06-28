package idgen

import (
	"github.com/egoadmin/egoadmin/internal/client/idgenclient"
	compidgen "github.com/egoadmin/egoadmin/internal/component/idgen"
	"github.com/egoadmin/egoadmin/internal/component/idgen/machine/grpcallocator"
	"github.com/egoadmin/egoadmin/internal/component/idgen/store/grpcstore"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

var ProviderSet = wire.NewSet(
	NewSegmentStore,
	NewMachineAllocator,
	NewMachineLeaseManager,
	NewComponent,
	NewIDGetter,
)

func NewSegmentStore(client idgenclient.SegmentService) compidgen.SegmentStore {
	return grpcstore.New(client)
}

func NewMachineAllocator(client idgenclient.MachineLeaseService) compidgen.MachineAllocator {
	return grpcallocator.New(client)
}

func NewMachineLeaseManager(allocator compidgen.MachineAllocator) compidgen.MachineLeaseManager {
	conf := compidgen.DefaultMachineConfig()
	key := "component.idgen.machine"
	if err := econf.UnmarshalKey(key, conf); err != nil {
		elog.Panic("parse idgen machine config error", elog.FieldErr(err), elog.FieldKey(key))
	}
	manager, err := compidgen.NewMachineLeaseManager(key, conf, allocator, elog.EgoLogger.With(elog.FieldComponent(compidgen.PackageName), elog.FieldComponentName(key)))
	if err != nil {
		elog.Panic("build idgen machine manager failed", elog.FieldErr(err), elog.FieldKey(key))
	}
	return manager
}

func NewComponent(store compidgen.SegmentStore, manager compidgen.MachineLeaseManager) *compidgen.Component {
	return compidgen.Load("component.idgen.default").Build(
		compidgen.WithSegmentStore(store),
		compidgen.WithMachineLeaseManager(manager),
	)
}

func NewIDGetter(component *compidgen.Component) xflake.Geter {
	generator, err := component.GeneratorDefault()
	if err != nil {
		elog.Panic("build idgen default getter failed", elog.FieldErr(err))
	}
	return compidgen.NewIDGetter(generator)
}
