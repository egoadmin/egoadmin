package controller

import (
	"context"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/gateway/application"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
)

type RoleBoundaryValidator interface {
	ValidateRoleAPIBoundary(context.Context, string, []uint64) ([]permissiondomain.Policy, error)
	APIIDsByPolicies(context.Context, []permissiondomain.Policy) ([]uint64, error)
}

type RoleGRPC struct {
	client    userclient.RoleService
	validator RoleBoundaryValidator
}

func NewRoleGRPCController(client *userclient.Client, validator RoleBoundaryValidator) *RoleGRPC {
	return &RoleGRPC{client: client.Role, validator: validator}
}

func (s *RoleGRPC) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error) {
	if err := s.prepareRolePolicies(ctx, in.GetRole()); err != nil {
		return nil, err
	}

	return s.client.AddRole(ctx, in)
}

func (s *RoleGRPC) DeleteRole(ctx context.Context, in *userv1.DeleteRoleRequest) (*userv1.DeleteRoleResponse, error) {
	return s.client.DeleteRole(ctx, in)
}

func (s *RoleGRPC) CheckDeleteRole(ctx context.Context, in *userv1.CheckDeleteRoleRequest) (*userv1.CheckDeleteRoleResponse, error) {
	return s.client.CheckDeleteRole(ctx, in)
}

func (s *RoleGRPC) UpdateRole(ctx context.Context, in *userv1.UpdateRoleRequest) (*userv1.UpdateRoleResponse, error) {
	if err := s.prepareRolePolicies(ctx, in.GetRole()); err != nil {
		return nil, err
	}

	return s.client.UpdateRole(ctx, in)
}

func (s *RoleGRPC) GetRole(ctx context.Context, in *userv1.GetRoleRequest) (*userv1.GetRoleResponse, error) {
	out, err := s.client.GetRole(ctx, in)
	if err != nil {
		return nil, err
	}
	if err = s.fillRoleAPIIDs(ctx, out.GetRole()); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RoleGRPC) GetRoleList(ctx context.Context, in *userv1.GetRoleListRequest) (*userv1.GetRoleListResponse, error) {
	return s.client.GetRoleList(ctx, in)
}

func (s *RoleGRPC) GetRoleAll(ctx context.Context, in *userv1.GetRoleAllRequest) (*userv1.GetRoleAllResponse, error) {
	return s.client.GetRoleAll(ctx, in)
}

func (s *RoleGRPC) prepareRolePolicies(ctx context.Context, role *userv1.Role) error {
	if role == nil {
		return nil
	}
	policies, err := s.validator.ValidateRoleAPIBoundary(ctx, role.GetViewMenus(), uniqueAPIIDs(role.GetApis()))
	if err != nil {
		return mapGatewayApplicationError(err)
	}
	role.Policies = rolePoliciesToRPC(policies)
	return nil
}

func (s *RoleGRPC) fillRoleAPIIDs(ctx context.Context, role *userv1.Role) error {
	if role == nil {
		return nil
	}
	apiIDs, err := s.validator.APIIDsByPolicies(ctx, rpcRolePolicies(role.GetPolicies()))
	if err != nil {
		return mapGatewayApplicationError(err)
	}
	role.Apis = apiIDs
	return nil
}

func uniqueAPIIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func rolePoliciesToRPC(policies []permissiondomain.Policy) []*userv1.RolePermissionPolicy {
	out := make([]*userv1.RolePermissionPolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, &userv1.RolePermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}
	return out
}

func rpcRolePolicies(policies []*userv1.RolePermissionPolicy) []permissiondomain.Policy {
	out := make([]permissiondomain.Policy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, permissiondomain.Policy{
			Service: policy.GetService(),
			Method:  policy.GetMethod(),
		})
	}
	return out
}

func mapGatewayApplicationError(err error) error {
	if err == nil {
		return nil
	}
	if msg, ok := application.UserMessage(err); ok {
		return ecode.ErrorFailed().WithMessage(msg)
	}
	return err
}
