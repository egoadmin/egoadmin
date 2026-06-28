package controller

import (
	"context"
	"math"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/egoadmin/elib/pkg/util/xtime"
	"github.com/jinzhu/copier"
)

// LogGrpc 日志grpc
type LogGRPC struct {
	logger auditlog.Loger
	log    *service.LogService
}

// NewLogGRPCController 实例化用户grpc
func NewLogGRPCController(log *service.LogService, logger auditlog.Loger) *LogGRPC {
	return &LogGRPC{
		logger: logger,
		log:    log,
	}
}

// GetLogList 获取系统日志列表
func (s *LogGRPC) GetLogList(ctx context.Context, in *userv1.GetLogListRequest) (out *userv1.GetLogListResponse, err error) {
	out = &userv1.GetLogListResponse{
		Logs: make([]*userv1.SysLog, 0),
	}

	opt := store.LogModelGetListOption{
		Pgopt: xorm.PaginateOption{
			Page:  int(in.GetPage()),
			Limit: int(in.GetLimit()),
			Sort:  in.GetSort(),
			Order: in.GetOrder(),
		},
		Username:  in.GetUsername(),
		Event:     in.GetEvent(),
		StartTime: xtime.Ts2Time(in.GetTimeRange().GetStart()),
		EndTime:   xtime.Ts2Time(in.GetTimeRange().GetEnd()),
	}

	logs, total, err := s.log.GetList(ctx, opt)
	if err != nil {
		return
	}
	if total > math.MaxInt32 {
		err = platformi18n.ErrorFailed(ctx, "LogCountExceeded", nil)
		return
	}
	//nolint:gosec // total is checked above and fits int32.
	out.Count = int32(total)
	err = copier.Copy(&out.Logs, &logs)

	return
}
