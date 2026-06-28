package store

import (
	"context"

	"gorm.io/gorm"
)

// DeptInterface 组织管理
type DeptInterface interface {
	// Add 添加组织
	Add(ctx context.Context, dept *DeptModel) (err error)
	// Delete 删除组织
	Delete(ctx context.Context, id uint64) (err error)
	// DeleteByIds 删除多个组织
	DeleteByIds(ctx context.Context, ids []uint64) (err error)
	// Update 修改组织
	Update(ctx context.Context, id uint64, name string) (err error)
	// UpdatePriority 修改排序
	UpdatePriority(ctx context.Context, depts []DeptModel) (err error)
	// GetSelf 查询组织
	// 此接口只返回该组织自生信息
	GetSelf(ctx context.Context, id uint64) (dept *DeptModel, err error)
	// Get 查询组织
	// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
	Get(ctx context.Context, id uint64) (dept *DeptModel, err error)
	// GetByName 根据名称模糊获取组织
	// 名称为空返回所有组织
	GetByName(ctx context.Context, name string) (depts []*DeptModel, err error)
	// GetByID 根据ID码获取组织
	// 返回组织及其所有子组织
	GetByID(ctx context.Context, id uint64) (depts []*DeptModel, err error)
	// GetByIDs 根据IDs获取组织本身
	GetByIDs(ctx context.Context, ids []uint64) (depts []*DeptModel, err error)
	// GetByCode 根据code码获取组织
	// 返回组织及其所有子组织
	GetByCode(ctx context.Context, code string) (depts []*DeptModel, err error)
	// GetSubtreeIDs 根据组织ID返回该组织及所有子组织ID
	GetSubtreeIDs(ctx context.Context, id uint64) (ids []uint64, err error)
	// GetAncestorIDs 根据组织路径返回该组织及所有上级组织ID
	GetAncestorIDs(ctx context.Context, id uint64) (ids []uint64, err error)
	// GetAll 查询所有组织
	GetAll(ctx context.Context) (depts []*DeptModel, err error)
	// GetTopAll 获取所有一级组织
	// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
	GetTopAll(ctx context.Context) (depts []*DeptModel, err error)
	// GetChilds 查询子节点
	// 此接口会同时返回当前数据的所有下一级数据，以用于是否可展开的计算
	GetChilds(ctx context.Context, parentID uint64) (depts []*DeptModel, err error)
	// CountByOption 统计组织
	CountByOption(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (count int64, err error)
}
