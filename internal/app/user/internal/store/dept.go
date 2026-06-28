package store

import (
	"context"
	"strconv"
	"strings"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
	"gorm.io/gorm"
)

const (
	// DeptModelParentTop 顶级父类id
	DeptModelParentTop = 0
)

const (
	// DeptModelStatusValid 有效.
	DeptModelStatusValid int32 = iota + 1
	// DeptModelStatusInvalid 无效.
	DeptModelStatusInvalid
)

const (
	// DeptModelMaxLevel 组织最大层级
	DeptModelMaxLevel int32 = 5
)

// DeptModel 系统组织.
type DeptModel struct {
	xorm.Model
	Code     string      `gorm:"uniqueIndex;type:varchar(500);not null;default:'';comment:部门编号[节点路径(节点id拼接)]"` // 部门编号[节点路径(节点id拼接)]
	ParentID uint64      `gorm:"index;type:bigint(20);not null;default:0;comment:上级组织ID"`
	DeptName string      `gorm:"index;type:varchar(255);not null;default:'';comment:组织名称"`
	Leader   string      `gorm:"type:varchar(255);not null;default:'';comment:组织负责人"`
	Phone    string      `gorm:"type:varchar(255);not null;default:'';comment:联系电话"`
	Email    string      `gorm:"type:varchar(255);not null;default:'';comment:邮箱"`
	Remark   string      `gorm:"type:varchar(255);not null;default:'';comment:备注"`
	Priority int32       `gorm:"type:int(10);not null;default:0;comment:排序,当前层级的排序"`
	Status   int32       `gorm:"type:int(10);not null;default:1;comment:状态,1正常,2禁用"` // 状态,1正常,2禁用
	Level    int32       `gorm:"type:int(10);not null;default:1;comment:组织层级"`       // 组织层级
	Childs   []DeptModel `gorm:"foreignkey:ParentID"`
}

// TableName 表名.
func (DeptModel) TableName() string {
	return "dept"
}

// SetID id设置接口.
func (m *DeptModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

// BeforeCreate 创建执行前钩子函数.
func (m *DeptModel) BeforeCreate(tx *gorm.DB) error {
	return mysql.SetID(m)
}

// HasChildToRPC 计算节点是否含有子节点
func (m *DeptModel) HasChildToRPC() (hasChild bool) {
	return len(m.Childs) != 0
}

// Dept 组织管理
type Dept struct {
	cc *egorm.Component
}

// NewDept 实例化组织管理
func NewDept(db *egorm.Component, id xorm.IDSetter) DeptInterface {
	return &Dept{
		cc: db,
	}
}

// Add 添加组织
func (m *Dept) Add(ctx context.Context, dept *DeptModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Create(&dept).Error

	return
}

// Delete 删除组织
func (m *Dept) Delete(ctx context.Context, id uint64) (err error) {
	var dept DeptModel
	dept.ID = id

	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Unscoped().Delete(&dept).Error

	return
}

// DeleteByIds 删除多个组织
func (m *Dept) DeleteByIds(ctx context.Context, ids []uint64) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Unscoped().Where("id IN (?)", ids).Delete(&DeptModel{}).Error

	return
}

// Update 修改组织
func (m *Dept) Update(ctx context.Context, id uint64, name string) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.
		Model(&DeptModel{Model: xorm.Model{ID: id}}).
		UpdateColumn("dept_name", name).Error

	return
}

// UpdatePriority 修改排序
func (m *Dept) UpdatePriority(ctx context.Context, depts []DeptModel) (err error) {
	err = mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
		var er error
		for _, v := range depts {
			if er = tx.Model(&DeptModel{}).Select("priority").Where("id = ?", v.ID).Update("priority", v.Priority).Error; er != nil {
				return er
			}
		}

		return nil
	})

	return
}

// GetSelf 查询组织
// 此接口只返回该组织自生信息
func (m *Dept) GetSelf(ctx context.Context, id uint64) (dept *DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).First(&dept, id).Error
	return
}

// Get 查询组织
// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
func (m *Dept) Get(ctx context.Context, id uint64) (dept *DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).Preload("Childs").First(&dept, id).Error

	return
}

// GetByName 根据名称模糊获取组织
// 名称为空返回所有组织
func (m *Dept) GetByName(ctx context.Context, name string) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Scopes(func(d *gorm.DB) *gorm.DB {
		if name == "" {
			return d
		}
		return d.Where("dept_name LIKE CONCAT('%',?,'%')", name)
	}).Find(&depts).Error

	return
}

// GetByID 根据ID码获取组织
// 返回组织及其所有子组织
func (m *Dept) GetByID(ctx context.Context, id uint64) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Table("dept as dt, (?) as dp", db.Model(&DeptModel{Model: xorm.Model{ID: id}}).Select("`code`")).Where("dt.`code` LIKE CONCAT(dp.`code`,'%')").Find(&depts).Error

	return
}

// GetByIDs 根据IDs获取组织本身
func (m *Dept) GetByIDs(ctx context.Context, ids []uint64) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("id IN (?)", ids).Find(&depts).Error

	return
}

// GetByCode 根据code码获取组织
// 返回组织及其所有子组织
func (m *Dept) GetByCode(ctx context.Context, code string) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("code LIKE ?", code+"'%'").Find(&depts).Error

	return
}

// GetSubtreeIDs 根据组织ID返回该组织及所有子组织ID
func (m *Dept) GetSubtreeIDs(ctx context.Context, id uint64) (ids []uint64, err error) {
	dept, err := m.GetSelf(ctx, id)
	if err != nil {
		return nil, err
	}
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).
		Where("code LIKE ?", dept.Code+"%").
		Pluck("id", &ids).Error
	return ids, err
}

// GetAncestorIDs 根据组织路径返回该组织及所有上级组织ID
func (m *Dept) GetAncestorIDs(ctx context.Context, id uint64) (ids []uint64, err error) {
	dept, err := m.GetSelf(ctx, id)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(dept.Code, "-")
	ids = make([]uint64, 0, len(parts))
	for _, part := range parts {
		var ancestorID uint64
		if ancestorID, err = strconv.ParseUint(part, 10, 64); err != nil {
			return nil, err
		}
		ids = append(ids, ancestorID)
	}
	return ids, nil
}

// GetAll 查询所有组织
func (m *Dept) GetAll(ctx context.Context) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).Find(&depts).Error

	return
}

// GetTopAll 获取所有一级组织
// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
func (m *Dept) GetTopAll(ctx context.Context) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).Preload("Childs").Order("priority ASC").Where("parent_id = ?", DeptModelParentTop).Find(&depts).Error

	return
}

// GetChilds 查询子节点
// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
func (m *Dept) GetChilds(ctx context.Context, parentID uint64) (depts []*DeptModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).Preload("Childs").Order("priority ASC").Where("parent_id = ?", parentID).Find(&depts).Error

	return
}

// CountDeptWithParentID 通过父组织统计子组织
func CountDeptWithParentID(parentID uint64) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where("parent_id = ?", parentID)
	}
}

// CountDeptWithDeptName 通过组织名称统计组织
func CountDeptWithDeptName(name string) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}

		return tx.Where("dept_name = ?", name)
	}
}

// CountByOption 统计组织
func (m *Dept) CountByOption(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&DeptModel{}).Scopes(scopes...).Count(&count).Error

	return
}
