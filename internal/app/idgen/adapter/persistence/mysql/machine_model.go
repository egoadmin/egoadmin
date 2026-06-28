package mysql

import "time"

type machineLeaseModel struct {
	Namespace     string    `gorm:"primaryKey;size:64;not null;default:'';comment:租约命名空间"`
	MachineID     int32     `gorm:"primaryKey;column:machine_id;not null;comment:机器号"`
	InstanceID    string    `gorm:"index:idx_idgen_machine_instance,priority:1;size:255;not null;default:'';comment:实例ID"`
	SessionID     string    `gorm:"size:64;not null;default:'';comment:租约会话ID"`
	TTLMillis     int64     `gorm:"not null;default:30000;comment:租约TTL毫秒"`
	RenewMillis   int64     `gorm:"not null;default:10000;comment:建议续租间隔毫秒"`
	ExpiresAt     time.Time `gorm:"index;not null;comment:过期时间"`
	LastRenewedAt time.Time `gorm:"not null;comment:最近续租时间"`
	CreatedAt     time.Time `gorm:"comment:创建时间"`
	UpdatedAt     time.Time `gorm:"comment:更新时间"`
}

func (machineLeaseModel) TableName() string {
	return "idgen_machine_lease"
}
