package service

import (
	"context"
	"errors"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xorm"
)

// RoleService 角色服务
type RoleService struct {
	Options
}

// NewRoleService 角色服务
func NewRoleService(options Options) *RoleService {
	return &RoleService{
		Options: options,
	}
}

// AddRole 新增角色
func (s *RoleService) AddRole(ctx context.Context, role *store.RoleModel) (err error) {
	if s.RoleUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "RoleUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
		return err
	}
	if !scope.IsAdmin {
		role.OwnerUserID = scope.UserID
		role.OwnerDeptID = scope.DeptID
	}
	result, err := s.RoleUseCase.CreateRole(ctx, roleCommandFromStore(role))
	if err != nil {
		return mapRoleDomainError(ctx, err)
	}
	role.ID = result.ID

	return
}

// DeleteRole 删除角色
func (s *RoleService) DeleteRole(ctx context.Context, id uint64) (err error) {
	if s.RoleUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "RoleUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	role, err := s.Role.Get(ctx, id)
	if err != nil {
		return err
	}
	if err = scope.EnforceRoleMutable(ctx, role); err != nil {
		return err
	}
	affectedUserIDs, err := s.userIDsByRole(ctx, id)
	if err != nil {
		return err
	}
	if err = mapRoleDomainError(ctx, s.RoleUseCase.DeleteRole(ctx, id)); err != nil {
		return err
	}
	return deleteDataScopeCache(ctx, s.DataScopeCache(), affectedUserIDs...)
}

// CheckDeleteRole 检查角色是否符合删除规则
func (s *RoleService) CheckDeleteRole(ctx context.Context, id uint64) (msg string, err error) {
	if s.RoleUseCase == nil {
		return "", platformi18n.ErrorFailed(ctx, "RoleUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return "", err
	}
	role, err := s.Role.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if err = scope.EnforceRole(ctx, role); err != nil {
		return "", err
	}
	return s.RoleUseCase.CheckDeleteRole(ctx, id)
}

// UpdateRole 修改角色
func (s *RoleService) UpdateRole(ctx context.Context, roleID uint64, role *store.RoleModel) (err error) {
	if s.RoleUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "RoleUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	savedRole, err := s.Role.Get(ctx, roleID)
	if err != nil {
		return err
	}
	if err = scope.EnforceRoleMutable(ctx, savedRole); err != nil {
		return err
	}
	if err = scope.EnforceAssignableDataPerm(ctx, role.DataPerm); err != nil {
		return err
	}
	role.OwnerUserID = savedRole.OwnerUserID
	role.OwnerDeptID = savedRole.OwnerDeptID
	affectedUserIDs, err := s.userIDsByRole(ctx, roleID)
	if err != nil {
		return err
	}
	if err = mapRoleDomainError(ctx, s.RoleUseCase.UpdateRole(ctx, roleID, roleCommandFromStore(role))); err != nil {
		return err
	}
	return deleteDataScopeCache(ctx, s.DataScopeCache(), affectedUserIDs...)
}

// GetRole 查询角色
func (s *RoleService) GetRole(ctx context.Context, id uint64) (role *store.RoleModel, err error) {
	scope, err := s.DataScope(ctx)
	if err != nil {
		return nil, err
	}
	role, err = s.Role.Get(ctx, id, scope.RoleScope())

	return
}

// GetRoleList 获取角色列表
func (s *RoleService) GetRoleList(ctx context.Context, name string, pgopt xorm.PaginateOption) (roles []*store.RoleModel, total int64, err error) {
	scope, err := s.DataScope(ctx)
	if err != nil {
		return nil, 0, err
	}
	roles, total, err = s.Role.GetList(ctx, name, pgopt, scope.RoleScope())

	return
}

// GetRoleAll 获取所有角色
func (s *RoleService) GetRoleAll(ctx context.Context) (roles []*store.RoleModel, err error) {
	scope, err := s.DataScope(ctx)
	if err != nil {
		return nil, err
	}
	roles, err = s.Role.GetAll(ctx, scope.RoleScope())

	return
}

func roleCommandFromStore(role *store.RoleModel) application.SaveRoleCommand {
	if role == nil {
		return application.SaveRoleCommand{}
	}
	return application.SaveRoleCommand{
		Name:        role.Name,
		Type:        role.Typ,
		DataPerm:    role.DataPerm,
		OwnerUserID: role.OwnerUserID,
		OwnerDeptID: role.OwnerDeptID,
		Uses:        role.Uses,
		ViewMenus:   role.ViewMenus,
		Desc:        role.Desc,
		Policies:    rolePoliciesFromStore(role.Policies),
	}
}

func rolePoliciesFromStore(policies []store.RolePermissionPolicy) []roledomain.PermissionPolicy {
	out := make([]roledomain.PermissionPolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, roledomain.PermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}
	return out
}

func mapRoleDomainError(ctx context.Context, err error) error {
	var inUse roledomain.InUseError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, roledomain.ErrNameExists):
		return platformi18n.ErrorFailed(ctx, "RoleNameExists", nil)
	case errors.As(err, &inUse):
		return platformi18n.ErrorFailed(ctx, "RoleInUseCount", map[string]any{"Count": inUse.Count})
	case errors.Is(err, roledomain.ErrInUse):
		return platformi18n.ErrorFailed(ctx, "RoleInUse", nil)
	default:
		return err
	}
}

func (s *RoleService) userIDsByRole(ctx context.Context, roleID uint64) ([]uint64, error) {
	users, err := s.User.GetByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(users))
	for _, user := range users {
		if user != nil && user.ID != 0 {
			ids = append(ids, user.ID)
		}
	}
	return ids, nil
}
