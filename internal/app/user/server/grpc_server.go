package server

import (
	"fmt"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/controller"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/middleware/ecode"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/recovery"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/egoadmin/elib/pkg/transport/grpc"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egrpc"
	"github.com/samber/lo"
	"golang.org/x/net/context"
)

var (
	devOpen  = []string{}
	openPack = []string{
		"/grpc.health.v1.Health/",
		"/user.v1.UserService/Login",
		"/user.v1.UserService/GetLoginCrypto",
		"/user.v1.UserService/GetCaptcha",
		"/user.v1.InternalAuthService",
	}
	justLoginPack = []string{
		"/user.v1.UserService/HeartBeatUser",
		"/user.v1.CenterService",
		"/user.v1.UserService/Logout",
		"/user.v1.UserService/GetMenus",
	}
)

type GrpcServer struct {
	*egrpc.Component
}

func NewGrpcServer(opts controller.Options, _ config.EgoReady) *GrpcServer {
	s := egrpc.Load("server.grpc").Build(
		egrpc.WithUnaryInterceptor(
			grpc.Middleware(
				recovery.Recovery(),
				opts.Auth.Server(
					authsession.WithIgnore(jwtIgnoreFunc(opts)),
				),
				perm.Server(
					permCheckFunc(opts),
				),
				validate.Validator(validate.WithV10(opts.Validator)),
				ecode.Ecode(),
			),
		),
		egrpc.WithStreamInterceptor(
			grpc.StreamMiddleware(
				recovery.StreamRecovery(),
				opts.Auth.ServerStream(
					authsession.WithIgnore(jwtIgnoreFunc(opts)),
				),
				perm.ServerStream(
					permCheckFunc(opts),
				),
				ecode.StreamEcode(),
			),
		),
	)

	userv1.RegisterInternalAuthServiceServer(s.Server, opts.AuthGRPC)
	userv1.RegisterLogServiceServer(s.Server, opts.LogGRPC)
	userv1.RegisterUserServiceServer(s.Server, opts.UserGRPC)
	userv1.RegisterRoleServiceServer(s.Server, opts.RoleGRPC)
	userv1.RegisterDeptServiceServer(s.Server, opts.DeptGRPC)
	userv1.RegisterCenterServiceServer(s.Server, opts.CenterGRPC)

	return &GrpcServer{Component: s}
}

func jwtIgnoreFunc(opts controller.Options) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		fullMethod := grpc.FromContext(ctx)
		return lo.ContainsBy(append(devOpen, openPack...), func(pack string) bool {
			return strings.HasPrefix(fullMethod, pack)
		})
	}
}

func permCheckFunc(opts controller.Options) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		fullMethod := grpc.FromContext(ctx)

		if lo.ContainsBy(append(devOpen, append(openPack, justLoginPack...)...), func(pack string) bool {
			return strings.HasPrefix(fullMethod, pack)
		}) {
			return true, nil
		}

		auth, ok := authsession.FromContext(ctx)
		if !ok {
			return false, nil
		}

		arrs := strings.Split(strings.Replace(fullMethod, "/", "", 1), "/")
		if len(arrs) < 2 {
			return false, nil
		}

		sub, obj, act := auth.Subject, strings.ToUpper(arrs[0]), strings.ToUpper(arrs[1])
		if sub == "" {
			sub = auth.Username
		}

		ok, err := opts.Casbin.Check(sub, obj, act)
		if err != nil {
			elog.Error("err", elog.FieldErr(err))
			return false, err
		}

		if !ok {
			elog.Info("无权访问", elog.FieldValue(fmt.Sprintf("sub: %s, obj: %s, act: %s", sub, obj, act)))
			return false, nil
		}

		return true, nil
	}
}
