package store

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// LogInterface ...
type LogInterface interface {
	// Save 保存日志
	Save(ctx context.Context, alog *LogModel) error
	// GetList 日志列表
	GetList(ctx context.Context, opt LogModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) (logs []*LogModel, total int64, err error)
	// DeleteLogBeforeDate 删除某日期之前的日志
	DeleteLogBeforeDate(ctx context.Context, date time.Time) (err error)
}
