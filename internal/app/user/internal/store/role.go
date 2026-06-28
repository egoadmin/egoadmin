package store

import (
	"context"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
	"gorm.io/gorm"
)

const (
	// RoleModelBuiltIn 内置角色.
	RoleModelBuiltIn int32 = iota + 1
	// RoleModelNonBuiltIn 普通角色.
	RoleModelNonBuiltIn
)

const (
	// RoleModelTypePlatform 平台角色
	RoleModelTypePlatform int32 = iota + 1
)

const (
	// RoleModelDataPermAll 1全部数据权限
	RoleModelDataPermAll int32 = iota + 1
	// RoleModelDataPermUserDeptAndSubDept 2用户所属组织及子组织数据权限
	RoleModelDataPermUserDeptAndSubDept
	// RoleModelDataPermUserDeptSelf 3用户所属组织自身数据权限
	RoleModelDataPermUserDeptSelf
	// RoleModelUserSelf 4仅用户自身数据权限
	RoleModelUserSelf
)

// RoleModel 角色模型
type RoleModel struct {
	xorm.Model
	Name        string                 `gorm:"index;type:varchar(255);not null;default:'';comment:分组名称,角色"`
	Typ         int32                  `gorm:"int(10);not null;default:1;comment:类型: 1平台角色"`
	BuiltIn     int32                  `gorm:"int(10);not null;default:2;comment:是否内置角色,1内置角色,2普通角色"`
	DataPerm    int32                  `gorm:"int(10);not null;default:1;comment:数据权限: 1全部数据权限,2用户所属组织及子组织数据权限,3用户所属组织自身数据权限,4仅用户自身数据权限"` // 数据权限: 1全部数据权限,2用户所属组织及子组织数据权限,3用户所属组织自身数据权限,4仅用户自身数据权限
	OwnerUserID uint64                 `gorm:"index:idx_role_owner_user_id;type:bigint(20) unsigned;not null;default:0;comment:角色创建者用户id"`
	OwnerDeptID uint64                 `gorm:"index:idx_role_owner_dept_id;type:bigint(20) unsigned;not null;default:0;comment:角色归属组织id"`
	Uses        string                 `gorm:"type:text;not null;comment:角色可用功能"` // 功能权限,采用,分隔
	ViewMenus   string                 `gorm:"type:text;not null;comment:页面菜单"`   // 前端视图菜单,采用,分隔的字符串
	Desc        string                 `gorm:"type:varchar(255);not null;default:'';comment:描述"`
	Policies    []RolePermissionPolicy `gorm:"foreignKey:RoleModelID;references:ID"` // 角色接口权限策略
}

// RolePermissionPolicy 角色接口权限策略.
type RolePermissionPolicy struct {
	RoleModelID uint64 `gorm:"primaryKey;type:bigint(20) unsigned;not null;comment:角色ID"`
	Service     string `gorm:"primaryKey;type:varchar(255);not null;default:'';comment:gRPC 服务名,casbin obj"`
	Method      string `gorm:"primaryKey;type:varchar(255);not null;default:'';comment:gRPC 方法名,casbin act"`
}

// TableName 表名.
func (RolePermissionPolicy) TableName() string {
	return "role_permission_policy"
}

// TableName 表名.
func (RoleModel) TableName() string {
	return "role"
}

// SetID id设置接口.
func (m *RoleModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

// BeforeCreate 创建执行前钩子函数.
func (m *RoleModel) BeforeCreate(tx *gorm.DB) error {
	return mysql.SetID(m)
}

// PoliciesToRPC 转换角色接口权限策略为RPC格式.
func (m *RoleModel) PoliciesToRPC() (policies []*userv1.RolePermissionPolicy) {
	policies = make([]*userv1.RolePermissionPolicy, 0, len(m.Policies))
	for _, policy := range m.Policies {
		policies = append(policies, &userv1.RolePermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}

	return
}

// NewRole 实例化角色管理
func NewRole(db *egorm.Component, id xorm.IDSetter) RoleInterface {
	return &Role{
		cc: db,
	}
}

// Role 角色管理
type Role struct {
	cc *egorm.Component
}

// Add 添加角色.
// 同时会保存角色的接口菜单.需将接口菜单传递过来.
func (m *Role) Add(ctx context.Context, role *RoleModel) (err error) {
	role.Policies = normalizePermissionPolicies(role.Policies)
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Create(&role).Error

	return
}

// Delete 删除角色.
func (m *Role) Delete(ctx context.Context, id uint64) (err error) {
	var role RoleModel
	role.ID = id

	err = mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
		if err := tx.Where(map[string]any{"role_model_id": id}).Delete(&RolePermissionPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&role).Error; err != nil {
			return err
		}

		return nil
	})

	return
}

// Update 修改角色.
// 需同时将角色的接口菜单传递过来.
func (m *Role) Update(ctx context.Context, id uint64, role *RoleModel) (err error) {
	role.ID = id
	role.Policies = normalizePermissionPolicies(role.Policies)
	return mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
		if err := tx.Model(&role).Select("Name", "Typ", "Desc", "Uses", "ViewMenus", "DataPerm", "OwnerUserID", "OwnerDeptID").Updates(&role).Error; err != nil {
			return err
		}
		if err := tx.Where(map[string]any{"role_model_id": id}).Delete(&RolePermissionPolicy{}).Error; err != nil {
			return err
		}
		for i := range role.Policies {
			role.Policies[i].RoleModelID = id
		}
		if len(role.Policies) == 0 {
			return nil
		}
		return tx.CreateInBatches(role.Policies, 100).Error
	})
}

// Get 查询角色
func (m *Role) Get(ctx context.Context, id uint64, scopes ...func(*gorm.DB) *gorm.DB) (role *RoleModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	})
	err = db.Model(&RoleModel{}).Preload("Policies").Scopes(scopes...).First(&role).Error

	return
}

// GetList 查询角色列表
func (m *Role) GetList(ctx context.Context, name string, opt xorm.PaginateOption, scopes ...func(*gorm.DB) *gorm.DB) (roles []*RoleModel, total int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	// 筛选条件
	scopes = append(scopes,
		roleScopeNameLike(name), // 名称
		func(db *gorm.DB) *gorm.DB {
			return db.Where("built_in=?", RoleModelNonBuiltIn) // 非内置角色
		},
	)

	// 分页处理
	if opt.Sort == "" {
		opt.Sort = createAt
		opt.Order = desc
	}

	// 查询角色数量
	err = db.Scopes(scopes...).Model(&RoleModel{}).Count(&total).Error
	if err != nil {
		return
	}
	scopes = append(scopes, xorm.WithScopePaginate(opt)...)
	scopes = append(scopes, scopeStableIDOrder())
	err = db.Scopes(scopes...).Find(&roles).Error

	return
}

// GetAll 获取所有角色
func (m *Role) GetAll(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (roles []*RoleModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&RoleModel{}).Scopes(scopes...).Find(&roles).Error

	return
}

// RoleCountWithName 通过角色名称统计角色
func RoleCountWithName(name string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if name == "" {
			return db
		}

		return db.Where("name = ?", name)
	}
}

// CountByOption 统计角色
func (m *Role) CountByOption(ctx context.Context, scope func(*gorm.DB) *gorm.DB) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&RoleModel{}).Scopes(scope).Count(&count).Error

	return
}

// DeletePermissionPolicies 删除指定接口对应的角色权限策略.
func (m *Role) DeletePermissionPolicies(ctx context.Context, policies []RolePermissionPolicy) (err error) {
	normalized := normalizePermissionPolicies(policies)
	if len(normalized) == 0 {
		return nil
	}

	db := mysql.DBWithContext(ctx, m.cc)
	for _, policy := range normalized {
		if err = db.Where(map[string]any{
			"service": policy.Service,
			"method":  policy.Method,
		}).Delete(&RolePermissionPolicy{}).Error; err != nil {
			return
		}
	}
	return nil
}

func normalizePermissionPolicies(policies []RolePermissionPolicy) []RolePermissionPolicy {
	normalized := make([]RolePermissionPolicy, 0, len(policies))
	seen := make(map[string]struct{}, len(policies))
	for _, policy := range policies {
		service := strings.ToUpper(strings.TrimSpace(policy.Service))
		method := strings.ToUpper(strings.TrimSpace(policy.Method))
		if service == "" || method == "" {
			continue
		}
		key := service + "/" + method
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, RolePermissionPolicy{
			RoleModelID: policy.RoleModelID,
			Service:     service,
			Method:      method,
		})
	}
	return normalized
}
