package controller

import (
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewInternalAuthGRPCController,
	NewLogGRPCController,
	NewUserGRPCController,
	NewRoleGRPCController,
	NewDeptGRPCController,
	NewCenterGRPCController,
)

type Options struct {
	Conf       *config.Config
	Casbin     *perm.Casbin
	Auth       *authsession.Component
	Validator  *validate.Validate
	LogGRPC    *LogGRPC
	AuthGRPC   *InternalAuthGRPC
	UserGRPC   *UserGRPC
	RoleGRPC   *RoleGRPC
	DeptGRPC   *DeptGRPC
	CenterGRPC *CenterGRPC
}
