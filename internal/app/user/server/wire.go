//go:build wireinject
// +build wireinject

package server

import (
	usercache "github.com/egoadmin/egoadmin/internal/app/user/adapter/cache"
	userpermission "github.com/egoadmin/egoadmin/internal/app/user/adapter/permission"
	usermysql "github.com/egoadmin/egoadmin/internal/app/user/adapter/persistence/mysql"
	"github.com/egoadmin/egoadmin/internal/app/user/application"
	"github.com/egoadmin/egoadmin/internal/app/user/controller"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/job"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	idgenclient "github.com/egoadmin/egoadmin/internal/client/idgenclient"
	"github.com/egoadmin/egoadmin/internal/platform/discovery"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/google/wire"
)

func NewApp() (*App, error) {
	panic(wire.Build(
		newEgo,
		newEgoReady,
		newConfig,
		newLoger,
		newSchemaReady,
		newHealth,
		newCasbin,
		newValidate,
		newCaptcha,
		wire.Struct(new(Options), "*"),
		application.ProviderSet,
		usermysql.ProviderSet,
		usercache.ProviderSet,
		userpermission.ProviderSet,
		wire.Bind(new(application.AuthSnapshotCache), new(*usercache.AuthSnapshotCache)),
		wire.Bind(new(application.UserLocks), new(*usercache.UserLocks)),
		wire.Bind(new(application.RoleLocks), new(*usercache.UserLocks)),
		wire.Bind(new(application.DeptLocks), new(*usercache.UserLocks)),
		wire.Bind(new(application.RoleAssignments), new(*usermysql.UserRepository)),
		wire.Bind(new(application.DeptAssignments), new(*usermysql.UserRepository)),
		wire.Bind(new(application.RoleBinding), new(*userpermission.RoleBinding)),
		wire.Bind(new(application.RolePermissionBinding), new(*userpermission.RoleBinding)),
		controller.ProviderSet,
		idgenclient.ProviderSet,
		service.ProviderSet,
		discovery.ProviderSet,
		shutdown.ProviderSet,
		wire.Bind(new(job.UserOfflineService), new(*service.UserService)),
		job.ProviderSet,
		ProviderSet,
		newApp,
	))
}
