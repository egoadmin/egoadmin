package server

import (
	idgenv1 "github.com/egoadmin/egoadmin/api/gen/go/idgen/v1"
	"github.com/egoadmin/egoadmin/internal/app/idgen/controller"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/middleware/ecode"
	"github.com/egoadmin/elib/pkg/middleware/recovery"
	"github.com/egoadmin/elib/pkg/transport/grpc"
	"github.com/gotomicro/ego/server/egrpc"
)

type GrpcServer struct {
	*egrpc.Component
}

func NewGrpcServer(opts controller.Options, _ config.EgoReady) *GrpcServer {
	s := egrpc.Load("server.grpc").Build(
		egrpc.WithUnaryInterceptor(
			grpc.Middleware(
				recovery.Recovery(),
				ecode.Ecode(),
			),
		),
		egrpc.WithStreamInterceptor(
			grpc.StreamMiddleware(
				recovery.StreamRecovery(),
				ecode.StreamEcode(),
			),
		),
	)

	idgenv1.RegisterSegmentServiceServer(s.Server, opts.Segment)
	idgenv1.RegisterMachineLeaseServiceServer(s.Server, opts.MachineLease)

	return &GrpcServer{Component: s}
}
