package controller

import (
	"context"
	"errors"
	"math"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/application"
	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/jinzhu/copier"
)

// RoleGrpc 角色grpc
type RoleGRPC struct {
	role        *service.RoleService
	roleUseCase *application.RoleUseCase
	logger      auditlog.Loger
}

// NewRoleGRPCController 实例化用户grpc
func NewRoleGRPCController(role *service.RoleService, roleUseCase *application.RoleUseCase, logger auditlog.Loger) *RoleGRPC {
	return &RoleGRPC{
		role:        role,
		roleUseCase: roleUseCase,
		logger:      logger,
	}
}

// AddRole 新增角色
func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (out *userv1.AddRoleResponse, err error) {
	out = &userv1.AddRoleResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "系统管理-角色管理", "新增", "新增角色", in)
		}
	}()

	role := &store.RoleModel{
		Name:      in.GetRole().GetName(),
		Typ:       in.GetRole().GetTyp(),
		DataPerm:  in.GetRole().GetDataPerm(),
		Uses:      in.GetRole().GetUses(),
		ViewMenus: in.GetRole().GetViewMenus(),
		Desc:      in.GetRole().GetDesc(),
		Policies:  roleStorePermissionPolicies(in.GetRole().GetPolicies()),
	}
	err = s.role.AddRole(ctx, role)
	if err = mapRoleError(ctx, err); err != nil {
		return
	}

	out.Id = role.ID

	return
}

// DeleteRole 删除角色
func (s *RoleGRPC) DeleteRole(ctx context.Context, in *userv1.DeleteRoleRequest) (out *userv1.DeleteRoleResponse, err error) {
	out = &userv1.DeleteRoleResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "系统管理-角色管理", "删除", "删除角色", in)
		}
	}()

	if err = mapRoleError(ctx, s.role.DeleteRole(ctx, in.GetId())); err != nil {
		return
	}

	return
}

// CheckDeleteRole 检查角色是否符合删除规则
func (s *RoleGRPC) CheckDeleteRole(ctx context.Context, in *userv1.CheckDeleteRoleRequest) (out *userv1.CheckDeleteRoleResponse, err error) {
	out = &userv1.CheckDeleteRoleResponse{}

	msg, err := s.role.CheckDeleteRole(ctx, in.GetId())
	if err != nil {
		return
	}
	if msg == "" {
		out.IsAllow = !out.IsAllow

		return
	}
	out.Msg = msg

	return
}

// UpdateRole 修改角色
func (s *RoleGRPC) UpdateRole(ctx context.Context, in *userv1.UpdateRoleRequest) (out *userv1.UpdateRoleResponse, err error) {
	out = &userv1.UpdateRoleResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "系统管理-角色管理", "编辑", "编辑角色", in)
		}
	}()

	err = mapRoleError(ctx, s.role.UpdateRole(ctx, in.GetId(), &store.RoleModel{
		Name:      in.GetRole().GetName(),
		Typ:       in.GetRole().GetTyp(),
		DataPerm:  in.GetRole().GetDataPerm(),
		Uses:      in.GetRole().GetUses(),
		ViewMenus: in.GetRole().GetViewMenus(),
		Desc:      in.GetRole().GetDesc(),
		Policies:  roleStorePermissionPolicies(in.GetRole().GetPolicies()),
	}))

	return
}

func roleStorePermissionPolicies(in []*userv1.RolePermissionPolicy) []store.RolePermissionPolicy {
	policies := make([]store.RolePermissionPolicy, 0, len(in))
	for _, policy := range in {
		policies = append(policies, store.RolePermissionPolicy{
			Service: policy.GetService(),
			Method:  policy.GetMethod(),
		})
	}
	return policies
}

// 预留:proto 权限策略转领域模型，供后续角色权限写入使用。
//
//nolint:unused // 预留:权限策略 proto→domain 转换
func rolePermissionPolicies(in []*userv1.RolePermissionPolicy) []roledomain.PermissionPolicy {
	policies := make([]roledomain.PermissionPolicy, 0, len(in))
	for _, policy := range in {
		policies = append(policies, roledomain.PermissionPolicy{
			Service: policy.GetService(),
			Method:  policy.GetMethod(),
		})
	}
	return policies
}

func mapRoleError(ctx context.Context, err error) error {
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

// GetRole 查询角色
func (s *RoleGRPC) GetRole(ctx context.Context, in *userv1.GetRoleRequest) (out *userv1.GetRoleResponse, err error) {
	out = &userv1.GetRoleResponse{
		Role: &userv1.Role{},
	}

	role, err := s.role.GetRole(ctx, in.GetId())
	if err != nil {
		return
	}

	if err = copier.Copy(&out.Role, &role); err != nil {
		return
	}

	return
}

// GetRoleList 获取角色列表
func (s *RoleGRPC) GetRoleList(ctx context.Context, in *userv1.GetRoleListRequest) (out *userv1.GetRoleListResponse, err error) {
	out = &userv1.GetRoleListResponse{}

	pgopt := xorm.PaginateOption{
		Page:  int(in.GetPage()),
		Limit: int(in.GetLimit()),
		Sort:  in.GetSort(),
		Order: in.GetOrder(),
	}

	roles, total, err := s.role.GetRoleList(ctx, in.GetName(), pgopt)
	if err != nil {
		return
	}
	if total > math.MaxInt32 {
		err = platformi18n.ErrorFailed(ctx, "RoleCountExceeded", nil)
		return
	}
	out.Roles = make([]*userv1.Role, 0, len(roles))
	//nolint:gosec // total is checked above and fits int32.
	out.Total = int32(total)

	if err = copier.Copy(&out.Roles, &roles); err != nil {
		return
	}

	return
}

// GetRoleAll 获取所有角色
func (s *RoleGRPC) GetRoleAll(ctx context.Context, in *userv1.GetRoleAllRequest) (out *userv1.GetRoleAllResponse, err error) {
	out = &userv1.GetRoleAllResponse{}

	roles, err := s.role.GetRoleAll(ctx)
	if err != nil {
		return
	}

	if err = copier.Copy(&out.Roles, &roles); err != nil {
		return
	}

	return
}
