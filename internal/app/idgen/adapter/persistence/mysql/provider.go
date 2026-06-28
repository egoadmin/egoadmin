package mysql

import (
	machinedomain "github.com/egoadmin/egoadmin/internal/app/idgen/domain/machine"
	segmentdomain "github.com/egoadmin/egoadmin/internal/app/idgen/domain/segment"
	"github.com/egoadmin/egoadmin/internal/component/idgen/store/gormstore"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSegmentRepository,
	NewMachineLeaseRepository,
	wire.Bind(new(segmentdomain.Repository), new(*SegmentRepository)),
	wire.Bind(new(machinedomain.Repository), new(*MachineLeaseRepository)),
)

func MigrationModels() []any {
	return []any{
		&gormstore.SegmentModel{},
		&machineLeaseModel{},
	}
}

func MigrationJoinTables() []platformmysql.MigrationJoinTable {
	return []platformmysql.MigrationJoinTable{}
}
