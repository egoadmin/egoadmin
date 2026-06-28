package mysql

import (
	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type roleAggregateModel struct {
	xorm.Model
	Name        string
	Typ         int32
	BuiltIn     int32
	DataPerm    int32
	OwnerUserID uint64
	OwnerDeptID uint64
	Uses        string
	ViewMenus   string
	Desc        string
	Policies    []rolePermissionPolicyModel `gorm:"foreignKey:RoleModelID;references:ID"`
}

func (roleAggregateModel) TableName() string {
	return "role"
}

func (m *roleAggregateModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *roleAggregateModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

type rolePermissionPolicyModel struct {
	RoleModelID uint64
	Service     string
	Method      string
}

func (rolePermissionPolicyModel) TableName() string {
	return "role_permission_policy"
}

func roleModelFromDomain(r *roledomain.Role) *roleAggregateModel {
	if r == nil {
		return nil
	}
	return &roleAggregateModel{
		Model:       xorm.Model{ID: r.ID},
		Name:        r.Name,
		Typ:         r.Type,
		BuiltIn:     r.BuiltIn,
		DataPerm:    r.DataPerm,
		OwnerUserID: r.OwnerUserID,
		OwnerDeptID: r.OwnerDeptID,
		Uses:        r.Uses,
		ViewMenus:   r.ViewMenus,
		Desc:        r.Desc,
		Policies:    rolePolicyModelsFromDomain(r.ID, r.Policies),
	}
}

func (m *roleAggregateModel) toDomain() *roledomain.Role {
	if m == nil {
		return nil
	}
	return &roledomain.Role{
		ID:          m.ID,
		Name:        m.Name,
		Type:        m.Typ,
		BuiltIn:     m.BuiltIn,
		DataPerm:    m.DataPerm,
		OwnerUserID: m.OwnerUserID,
		OwnerDeptID: m.OwnerDeptID,
		Uses:        m.Uses,
		ViewMenus:   m.ViewMenus,
		Desc:        m.Desc,
		Policies:    rolePoliciesToDomain(m.Policies),
	}
}

func rolePolicyModelsFromDomain(roleID uint64, policies []roledomain.PermissionPolicy) []rolePermissionPolicyModel {
	normalized := roledomain.NormalizePolicies(policies)
	models := make([]rolePermissionPolicyModel, 0, len(normalized))
	for _, policy := range normalized {
		models = append(models, rolePermissionPolicyModel{
			RoleModelID: roleID,
			Service:     policy.Service,
			Method:      policy.Method,
		})
	}
	return models
}

func rolePoliciesToDomain(policies []rolePermissionPolicyModel) []roledomain.PermissionPolicy {
	out := make([]roledomain.PermissionPolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, roledomain.PermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}
	return roledomain.NormalizePolicies(out)
}
