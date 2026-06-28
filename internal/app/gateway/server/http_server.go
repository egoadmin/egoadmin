package server

import (
	"net/http"

	egoadmin "github.com/egoadmin/egoadmin"
	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/gateway/controller"
	"github.com/egoadmin/elib/pkg/middleware/ecode"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/recovery"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/egoadmin/xgin"
	"github.com/egoadmin/xgin/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/server/egin"
)

const openAPIYAMLPath = "/openapi.yaml"

// NewHttpServer 实例化http服务
func NewHttpServer(opts controller.Options) *egin.Component {
	server := xgin.Load("server.http").Build()
	util.DisableGinResponseWrapping()
	util.WithMiddleware(
		// 设置恢复拦截器
		recovery.Recovery(),
		// 设置认证会话验证拦截器
		remoteAuthServer(opts),
		// 设置用户访问鉴权拦截器
		perm.Server(
			permCheckFunc(opts),
		),
		// 设置参数验证拦截器
		validate.Validator(validate.WithV10(opts.Validator)),
		ecode.Ecode(), // 设置错误码拦截器
	)

	registerHttp(server, opts)
	registerOpenAPIDoc(server)

	return server
}

func registerHttp(r *egin.Component, opts controller.Options) {
	// 注册日志服务
	userv1.RegisterLogServiceHTTPServer(r, opts.LogGRPC)
	// 注册用户服务
	userv1.RegisterUserServiceHTTPServer(r, opts.UserGRPC)
	// 注册用户角色服务
	userv1.RegisterRoleServiceHTTPServer(r, opts.RoleGRPC)
	// 注册用户组织服务
	userv1.RegisterDeptServiceHTTPServer(r, opts.DeptGRPC)
	//  注册用户中心服务
	userv1.RegisterCenterServiceHTTPServer(r, opts.CenterGRPC)
}

func registerOpenAPIDoc(r *egin.Component) {
	r.GET(openAPIYAMLPath, func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=3600")
		ctx.Data(http.StatusOK, "application/yaml; charset=utf-8", egoadmin.OpenAPIYAML)
	})
}
