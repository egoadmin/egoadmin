package gormstore

import "time"

const (
	DefaultTableName = "idgen_segment"

	StatusEnabled  = 1
	StatusDisabled = 2
)

// SegmentModel is the GORM schema source for Atlas migrations.
type SegmentModel struct {
	Namespace   string     `gorm:"primaryKey;size:64;not null;default:'';comment:命名空间"`
	Name        string     `gorm:"primaryKey;size:128;not null;default:'';comment:业务名称"`
	NextID      int64      `gorm:"not null;default:1;comment:下一个可分配ID"`
	Step        int64      `gorm:"not null;default:10000;comment:基础步长"`
	MinStep     int64      `gorm:"not null;default:10000;comment:最小步长"`
	MaxStep     int64      `gorm:"not null;default:100000000;comment:最大步长"`
	Status      int32      `gorm:"not null;default:1;comment:状态,1启用,2禁用"`
	LastStep    int64      `gorm:"not null;default:0;comment:上次实际领取步长"`
	Description string     `gorm:"size:255;not null;default:'';comment:描述"`
	LastFetchAt *time.Time `gorm:"comment:上次领取号段时间"`
	CreatedAt   time.Time  `gorm:"comment:创建时间"`
	UpdatedAt   time.Time  `gorm:"comment:更新时间"`
}

func (SegmentModel) TableName() string {
	return DefaultTableName
}
