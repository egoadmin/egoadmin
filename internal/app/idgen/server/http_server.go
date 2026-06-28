package server

import (
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/xgin"
	"github.com/egoadmin/xgin/pkg/util"
	"github.com/gotomicro/ego/server/egin"
)

// NewHttpServer creates a health-only HTTP server for idgen.
// It does not register auth, permission, validation, or business HTTP routes.
// 依赖 config.EgoReady 以保证在 ego 配置加载（econf 就绪）之后构造。
func NewHttpServer(_ config.EgoReady) *egin.Component {
	server := xgin.Load("server.http").Build()
	util.DisableGinResponseWrapping()
	return server
}
