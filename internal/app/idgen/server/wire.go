//go:build wireinject
// +build wireinject

package server

import (
	idgenmysql "github.com/egoadmin/egoadmin/internal/app/idgen/adapter/persistence/mysql"
	"github.com/egoadmin/egoadmin/internal/app/idgen/application"
	"github.com/egoadmin/egoadmin/internal/app/idgen/controller"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/discovery"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/google/wire"
)

func NewApp() (*App, error) {
	panic(wire.Build(
		newEgo,
		newConfig,
		newSchemaReady,
		newHealth,
		wire.Struct(new(Options), "*"),
		application.ProviderSet,
		idgenmysql.ProviderSet,
		controller.ProviderSet,
		mysql.NoIDProviderSet,
		discovery.ProviderSet,
		shutdown.ProviderSet,
		ProviderSet,
		newApp,
	))
}
