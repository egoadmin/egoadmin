package mysql

import (
	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAPIRepository,
	NewUploadRepository,
	wire.Bind(new(apidomain.Repository), new(*APIRepository)),
	wire.Bind(new(uploadcomponent.MetadataStore), new(*UploadRepository)),
)

func MigrationModels() []any {
	models := platformmysql.MigrationModels()
	models = append(models,
		&apiModel{},
		&fileObjectModel{},
		&fileReferenceModel{},
		&uploadSessionModel{},
	)
	return models
}

func MigrationJoinTables() []platformmysql.MigrationJoinTable {
	return []platformmysql.MigrationJoinTable{}
}
