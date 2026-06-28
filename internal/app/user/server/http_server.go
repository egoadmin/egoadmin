package server

import (
	"github.com/egoadmin/xgin"
	"github.com/egoadmin/xgin/pkg/util"
	"github.com/gotomicro/ego/server/egin"
)

// NewHttpServer creates a health-only HTTP server for user.
// Business APIs stay gRPC-only; health endpoints intentionally have no auth middleware.
func NewHttpServer() *egin.Component {
	server := xgin.Load("server.http").Build()
	util.DisableGinResponseWrapping()
	return server
}
