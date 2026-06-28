package controller

import (
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewLogGRPCController,
	NewUserGRPCController,
	NewRoleGRPCController,
	NewDeptGRPCController,
	NewCenterGRPCController,
)

type Options struct {
	Conf       *config.Config
	Validator  *validate.Validate
	UserClient *userclient.Client
	LogGRPC    *LogGRPC
	UserGRPC   *UserGRPC
	RoleGRPC   *RoleGRPC
	DeptGRPC   *DeptGRPC
	CenterGRPC *CenterGRPC
}
