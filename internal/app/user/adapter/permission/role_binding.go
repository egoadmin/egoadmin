package permission

import (
	"context"
	"strconv"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	"github.com/egoadmin/elib/pkg/middleware/perm"
)

// RoleBinding syncs user role bindings to Casbin.
type RoleBinding struct {
	casbin *perm.Casbin
}

var _ application.RoleBinding = (*RoleBinding)(nil)
var _ application.RolePermissionBinding = (*RoleBinding)(nil)

// NewRoleBinding creates a Casbin-backed role binding adapter.
func NewRoleBinding(casbin *perm.Casbin) *RoleBinding {
	return &RoleBinding{casbin: casbin}
}

func (b *RoleBinding) ReplaceUserRoles(ctx context.Context, username string, roleIDs []uint64) error {
	if b == nil || b.casbin == nil {
		return nil
	}
	if err := b.casbin.DeleteRolesForUser(username); err != nil {
		return err
	}
	return b.casbin.AddRolesForUser(username, roleNames(roleIDs))
}

func (b *RoleBinding) ReplaceRolePermissions(ctx context.Context, roleID uint64, policies []roledomain.PermissionPolicy) error {
	if b == nil || b.casbin == nil {
		return nil
	}
	roleName := roledomain.CasbinName(roleID)
	rules := rolePermissionRules(roleName, policies)
	if len(rules) == 0 {
		return b.casbin.DeletePermissionsForUser(roleName)
	}
	return b.casbin.UpdatePermissionsForUser(roleName, rules)
}

func (b *RoleBinding) DeleteRole(ctx context.Context, roleID uint64) error {
	if b == nil || b.casbin == nil {
		return nil
	}
	return b.casbin.DeleteRole(roledomain.CasbinName(roleID))
}

func roleNames(roleIDs []uint64) []string {
	names := make([]string, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		names = append(names, strconv.FormatUint(roleID, 10))
	}
	return names
}

func rolePermissionRules(roleName string, policies []roledomain.PermissionPolicy) [][]string {
	normalized := roledomain.NormalizePolicies(policies)
	rules := make([][]string, 0, len(normalized))
	for _, policy := range normalized {
		rules = append(rules, []string{
			roleName,
			policy.Service,
			policy.Method,
		})
	}
	return rules
}
