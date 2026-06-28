package service

import (
	"context"
	"time"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
)

// LogService 日志服务
type LogService struct {
	Options
}

// NewLogService 日志服务
func NewLogService(options Options) *LogService {
	return &LogService{
		Options: options,
	}
}

// GetList 查询日志列表
func (s *LogService) GetList(ctx context.Context, opt store.LogModelGetListOption) (logs []*store.LogModel, total int64, err error) {
	scope, err := s.DataScope(ctx)
	if err != nil {
		return nil, 0, err
	}
	return s.Log.GetList(ctx, opt, scope.LogScope())
}

// CleanLog 清理两年前的日志
func (s *LogService) CleanLog(ctx context.Context) (err error) {
	beforeDate := time.Now().AddDate(-2, 0, -1)
	err = s.Log.DeleteLogBeforeDate(ctx, beforeDate)

	return
}
