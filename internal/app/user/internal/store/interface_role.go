package store

import (
	"context"

	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

// RoleInterface 角色管理
type RoleInterface interface {
	// Add 新增角色
	Add(ctx context.Context, role *RoleModel) (err error)
	// Delete 删除角色
	Delete(ctx context.Context, id uint64) (err error)
	// Update 修改角色
	Update(ctx context.Context, id uint64, role *RoleModel) (err error)
	// Get 查询角色
	Get(ctx context.Context, id uint64, scopes ...func(*gorm.DB) *gorm.DB) (role *RoleModel, err error)
	// GetList 查询角色列表
	GetList(ctx context.Context, name string, opt xorm.PaginateOption, scopes ...func(*gorm.DB) *gorm.DB) (roles []*RoleModel, total int64, err error)
	// GetAll 获取所有角色
	GetAll(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (roles []*RoleModel, err error)
	// CountByOption 统计角色
	CountByOption(ctx context.Context, scope func(*gorm.DB) *gorm.DB) (count int64, err error)
	// DeletePermissionPolicies 删除指定接口对应的角色权限策略
	DeletePermissionPolicies(ctx context.Context, policies []RolePermissionPolicy) (err error)
}
