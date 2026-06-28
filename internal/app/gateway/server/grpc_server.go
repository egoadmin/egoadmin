package server

import (
	"context"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/gateway/controller"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	ecodev1 "github.com/egoadmin/elib/api/gen/go/ecode/v1"
	"github.com/egoadmin/elib/pkg/constant"
	"github.com/egoadmin/elib/pkg/metadata"
	emiddleware "github.com/egoadmin/elib/pkg/middleware"
	"github.com/egoadmin/elib/pkg/middleware/ecode"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/recovery"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/egoadmin/elib/pkg/transport/grpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egrpc"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/samber/lo"
	ggrpc "google.golang.org/grpc"
)

// 采用前缀匹配的方式，前缀将放通同前缀的所有接口。
// 如果仅需放通包中的某一两个方法需将地址写全，所以如非特殊需要请将需要放通的接口放同一个包对同一包进行统一放通。
var (
	// devOpen 开发时忽略的包,仅用于方便调试.调试结束请删除
	devOpen = []string{}
	// openPack 开放访问的包
	openPack = []string{
		"/grpc.health.v1.Health/",             // gRPC 健康检查
		"/user.v1.UserService/Login",          // 全局允许登录接口
		"/user.v1.UserService/GetLoginCrypto", // 获取登录加密参数
		"/user.v1.UserService/GetCaptcha",     // 获取验证码
	}
	// justLoginPack 仅登录包。需登录,无需鉴权的包
	justLoginPack = []string{
		"/user.v1.UserService/HeartBeatUser", // 心跳上报接口
		"/user.v1.CenterService",             // 个人中心
		"/user.v1.UserService/Logout",        // 退出登录
		"/user.v1.UserService/GetMenus",      // 获取菜单
	}
)

// GrpcServer grpc服务
type GrpcServer struct {
	*egrpc.Component
}

// NewGrpcServer 实例化grpc服务
func NewGrpcServer(opts controller.Options, _ config.EgoReady) *GrpcServer {
	// 从配置文件中加载gRPC服务器配置
	s := egrpc.Load("server.grpc").Build(
		// 设置一元拦截器
		egrpc.WithUnaryInterceptor(
			// 使用grpc.Middleware函数设置多个拦截器
			grpc.Middleware(
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
			),
		),
		// 设置流拦截器
		egrpc.WithStreamInterceptor(
			// 使用grpc.StreamMiddleware函数设置多个流拦截器
			grpc.StreamMiddleware(
				// 设置流恢复拦截器
				recovery.StreamRecovery(),
				// 设置认证会话验证流拦截器
				remoteAuthServerStream(opts),
				// 设置用户访问鉴权流拦截器
				perm.ServerStream(
					permCheckFunc(opts),
				),
				ecode.StreamEcode(), // 设置错误码拦截器
			),
		),
	)

	// 注册日志服务
	userv1.RegisterLogServiceServer(s.Server, opts.LogGRPC)
	// 注册用户服务
	userv1.RegisterUserServiceServer(s.Server, opts.UserGRPC)
	// 注册用户角色服务
	userv1.RegisterRoleServiceServer(s.Server, opts.RoleGRPC)
	// 注册用户组织服务
	userv1.RegisterDeptServiceServer(s.Server, opts.DeptGRPC)
	//  注册用户中心服务
	userv1.RegisterCenterServiceServer(s.Server, opts.CenterGRPC)

	// 返回GrpcServer结构体指针
	return &GrpcServer{
		Component: s,
	}
}

// jwtIgnoreFunc 忽略jwt鉴权处理函数
func jwtIgnoreFunc(opts controller.Options) func(ctx context.Context) bool {
	// 定义一个返回bool类型的函数，该函数接收一个context.Context类型的参数
	return func(ctx context.Context) bool {
		// 检测是否开放链接
		// 从context中获取完整的方法名
		fullMethod := grpc.FromContext(ctx)

		// 判断fullMethod是否以devOpen和openPack中的任意一个字符串为前缀
		return lo.ContainsBy(append(devOpen, openPack...), func(pack string) bool {
			return strings.HasPrefix(fullMethod, pack)
		})
	}
}

func remoteAuthServer(opts controller.Options) emiddleware.Middleware {
	return func(handler emiddleware.Handler) emiddleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			nextCtx, err := remoteAuthContext(ctx, opts)
			if err != nil {
				return nil, err
			}
			return handler(nextCtx, req)
		}
	}
}

func remoteAuthServerStream(opts controller.Options) emiddleware.StreamMiddleware {
	return func(handler emiddleware.StreamHandler) emiddleware.StreamHandler {
		return func(srv interface{}, stream ggrpc.ServerStream) error {
			wrappedStream := grpc_middleware.WrapServerStream(stream)
			nextCtx, err := remoteAuthContext(stream.Context(), opts)
			if err != nil {
				return err
			}
			wrappedStream.WrappedContext = nextCtx
			return handler(srv, wrappedStream)
		}
	}
}

func remoteAuthContext(ctx context.Context, opts controller.Options) (context.Context, error) {
	if jwtIgnoreFunc(opts)(ctx) {
		return ctx, nil
	}

	rawToken, err := extractBearerToken(ctx)
	if err != nil {
		return nil, err
	}
	auth, err := opts.UserClient.InternalAuth.ValidateAccessToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	return authsession.NewContext(ctx, auth), nil
}

// permCheckFunc 是一个函数，它返回一个函数，该函数用于检查用户是否有权限访问接口
func permCheckFunc(opts controller.Options) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		// 验证接口是否需要跳过鉴权
		fullMethod := grpc.FromContext(ctx)

		// 如果接口在devOpen、openPack或justLoginPack中，则跳过鉴权
		if lo.ContainsBy(append(devOpen, append(openPack, justLoginPack...)...), func(pack string) bool {
			return strings.HasPrefix(fullMethod, pack)
		}) {
			// 跳过鉴权
			return true, nil
		}

		// 读取用户信息
		auth, ok := authsession.FromContext(ctx)
		if !ok {
			return false, nil
		}

		service, method, ok := splitFullMethod(fullMethod)
		if !ok {
			return false, nil
		}

		// 鉴权
		ok, err := opts.UserClient.InternalAuth.CheckPermission(ctx, auth, service, method)
		if err != nil {
			elog.Error("err", elog.FieldErr(err))

			return false, err
		}

		if !ok {
			elog.Info("无权访问", elog.FieldValue(strings.ToUpper(service)+"/"+strings.ToUpper(method)))
			return false, nil
		}

		return true, nil
	}
}

func splitFullMethod(fullMethod string) (string, string, bool) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	arrs := strings.SplitN(trimmed, "/", 2)
	if len(arrs) != 2 || arrs[0] == "" || arrs[1] == "" {
		return "", "", false
	}
	return arrs[0], arrs[1], true
}

func extractBearerToken(ctx context.Context) (string, error) {
	return extractBearerTokenFromValue(ctx, metadata.ExtractIncoming(ctx).Get(constant.MDHeaderAuthorize))
}

func extractBearerTokenFromValue(ctx context.Context, value string) (string, error) {
	if value == "" {
		return "", ecodev1.ErrorUnauthenticated().WithMessage(platformi18n.Message(ctx, "AuthMissingToken"))
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", ecodev1.ErrorUnauthenticated().WithMessage(platformi18n.Message(ctx, "AuthMissingToken"))
	}
	return parts[1], nil
}
