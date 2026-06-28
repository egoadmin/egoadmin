package server

import (
	"github.com/egoadmin/xgin"
	"github.com/egoadmin/xgin/pkg/util"
	"github.com/gotomicro/ego/server/egin"
)

// NewHttpServer creates a health-only HTTP server for idgen.
// It does not register auth, permission, validation, or business HTTP routes.
func NewHttpServer() *egin.Component {
	server := xgin.Load("server.http").Build()
	util.DisableGinResponseWrapping()
	return server
}
