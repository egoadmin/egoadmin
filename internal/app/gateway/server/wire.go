//go:build wireinject
// +build wireinject

package server

import (
	gatewaypermission "github.com/egoadmin/egoadmin/internal/app/gateway/adapter/permission"
	gatewaymysql "github.com/egoadmin/egoadmin/internal/app/gateway/adapter/persistence/mysql"
	"github.com/egoadmin/egoadmin/internal/app/gateway/application"
	"github.com/egoadmin/egoadmin/internal/app/gateway/controller"
	"github.com/egoadmin/egoadmin/internal/app/gateway/internal/job"
	idgenclient "github.com/egoadmin/egoadmin/internal/client/idgenclient"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	cdncomponent "github.com/egoadmin/egoadmin/internal/component/cdn"
	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/egoadmin/egoadmin/internal/platform/cache/redis"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/discovery"
	idgenplatform "github.com/egoadmin/egoadmin/internal/platform/idgen"
	"github.com/egoadmin/egoadmin/internal/platform/objectstore"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/google/wire"
)

func NewApp() (*App, error) {
	panic(wire.Build(
		newEgo,
		newEgoReady,
		newConfig,
		newFrontendAssetsFS,
		newHealth,
		newSchemaReady,
		newValidate,
		wire.Struct(new(Options), "*"),
		application.ProviderSet,
		gatewaymysql.ProviderSet,
		gatewaypermission.ProviderSet,
		wire.Bind(new(application.PermissionPolicyCleaner), new(*gatewaypermission.PolicyCleaner)),
		mysql.CoreProviderSet,
		redis.NewRedisComponent,
		idgenplatform.ProviderSet,
		idcodec.ProviderSet,
		objectstore.ProviderSet,
		uploadcomponent.ProviderSet,
		cdncomponent.ProviderSet,
		job.ProviderSet,
		shutdown.ProviderSet,
		controller.ProviderSet,
		idgenclient.ProviderSet,
		userclient.ProviderSet,
		discovery.ProviderSet,
		wire.Bind(new(controller.RoleBoundaryValidator), new(*application.PermissionUseCase)),
		ProviderSet,
		newApp,
	))
}
