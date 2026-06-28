package store

import (
	"context"
	"strconv"
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
	"gorm.io/gorm"
)

// Log 日志操作
type Log struct {
	cc *egorm.Component
}

// LogModel 日志模型
type LogModel struct {
	xorm.Model
	UserID     string `gorm:"type:varchar(50);not null;default:'';comment:用户id"`
	UserIDU64  uint64 `gorm:"column:user_id_u64;index:idx_sys_log_user_id_u64_created_id,priority:1;type:bigint(20) unsigned;not null;default:0;comment:用户id数值列"`
	Username   string `gorm:"type:varchar(255);not null;default:'';comment:用户名"`
	DeptID     string `gorm:"type:varchar(50);not null;default:'';comment:用户部门id"`
	DeptIDU64  uint64 `gorm:"column:dept_id_u64;index:idx_sys_log_dept_id_u64_created_id,priority:1;type:bigint(20) unsigned;not null;default:0;comment:用户部门id数值列"`
	DeptName   string `gorm:"type:varchar(1000);not null;default:'';comment:用户部门全称"`
	Typ        string `gorm:"type:varchar(255);not null;default:'';comment:操作类型"`
	ModuleName string `gorm:"type:varchar(255);not null;default:'';comment:模块名"`
	Title      string `gorm:"type:varchar(255);not null;default:'';comment:标题，如创建用户"`
	URL        string `gorm:"column:url;type:varchar(255);not null;default:'';comment:访问链接"`
	Method     string `gorm:"type:varchar(255);not null;default:'';comment:请求方法,如GET,POST等"`
	ClientIP   string `gorm:"column:client_ip;type:varchar(255);not null;default:'';comment:客户端ip"`
	Params     string `gorm:"type:text;not null;comment:请求参数"`
	Remark     string `gorm:"type:text;comment:备注"`
}

// TableName 表名.
func (LogModel) TableName() string {
	return "sys_log"
}

// SetID id设置接口.
func (m *LogModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

// BeforeCreate 创建执行前钩子函数.
func (m *LogModel) BeforeCreate(tx *gorm.DB) error {
	return mysql.SetID(m)
}

// UserIdToRPC id转RPC.
func (m *LogModel) UserIdToRPC() uint64 {
	if m.UserIDU64 != 0 {
		return m.UserIDU64
	}
	num, _ := strconv.ParseUint(m.UserID, 10, 64)

	return num
}

// DeptIdToRPC id转RPC.
func (m *LogModel) DeptIdToRPC() uint64 {
	if m.DeptIDU64 != 0 {
		return m.DeptIDU64
	}
	num, _ := strconv.ParseUint(m.DeptID, 10, 64)

	return num
}

// NewLog 实例化日志
func NewLog(db *egorm.Component) LogInterface {
	return &Log{
		cc: db,
	}
}

// Save 保存日志
func (m *Log) Save(ctx context.Context, alog *LogModel) (err error) {
	if alog.UserIDU64 == 0 && alog.UserID != "" {
		alog.UserIDU64, _ = strconv.ParseUint(alog.UserID, 10, 64)
	}
	if alog.DeptIDU64 == 0 && alog.DeptID != "" {
		alog.DeptIDU64, _ = strconv.ParseUint(alog.DeptID, 10, 64)
	}
	return mysql.DBWithContext(ctx, m.cc).Create(alog).Error
}

// LogModelGetListOption 查询日志参数
type LogModelGetListOption struct {
	Pgopt     xorm.PaginateOption
	Username  string
	Event     string
	StartTime time.Time
	EndTime   time.Time
}

// GetList 日志列表
func (m *Log) GetList(ctx context.Context, opt LogModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) (logs []*LogModel, total int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	// 筛选条件
	scopes = append(scopes,
		logScopeUsernameLike(opt.Username),
		logScopeTitleLike(opt.Event),
		logScopeCratedAtRange(opt.StartTime, opt.EndTime),
	)

	// 分页处理
	if opt.Pgopt.Sort == "" {
		opt.Pgopt.Sort = createAt
		opt.Pgopt.Order = desc
	}

	// 查询日志数量
	err = db.Scopes(scopes...).Model(&LogModel{}).Count(&total).Error
	if err != nil {
		return
	}
	scopes = append(scopes, xorm.WithScopePaginate(opt.Pgopt)...)
	scopes = append(scopes, scopeStableIDOrder())
	err = db.Scopes(scopes...).Find(&logs).Error

	return
}

// DeleteLogBeforeDate 删除某日期之前的日志
func (m *Log) DeleteLogBeforeDate(ctx context.Context, date time.Time) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Unscoped().Where("created_at < ?", date).Delete(&LogModel{}).Error

	return
}
