package controller

import (
	"context"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/gotomicro/ego/core/elog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// InternalAuthGRPC 提供服务间鉴权接口。
type InternalAuthGRPC struct {
	auth   *authsession.Component
	casbin *perm.Casbin
	role   store.RoleInterface
}

// NewInternalAuthGRPCController 实例化服务间鉴权 grpc。
func NewInternalAuthGRPCController(auth *authsession.Component, casbin *perm.Casbin, role store.RoleInterface) *InternalAuthGRPC {
	return &InternalAuthGRPC{auth: auth, casbin: casbin, role: role}
}

// ValidateAccessToken 校验 access token。
func (s *InternalAuthGRPC) ValidateAccessToken(ctx context.Context, in *userv1.ValidateAccessTokenRequest) (*userv1.ValidateAccessTokenResponse, error) {
	auth, err := s.auth.ValidateAccessToken(ctx, in.GetAccessToken())
	if err != nil {
		return nil, authsession.ToEcodeContext(ctx, err)
	}

	return &userv1.ValidateAccessTokenResponse{
		Auth: authContextToRPC(auth),
	}, nil
}

// CheckPermission 校验接口权限。
func (s *InternalAuthGRPC) CheckPermission(ctx context.Context, in *userv1.CheckPermissionRequest) (*userv1.CheckPermissionResponse, error) {
	auth := authContextFromRPC(in.GetAuth())
	if auth == nil {
		return &userv1.CheckPermissionResponse{}, nil
	}

	sub := auth.Subject
	if sub == "" {
		sub = auth.Username
	}
	if sub == "" || in.GetService() == "" || in.GetMethod() == "" {
		return &userv1.CheckPermissionResponse{}, nil
	}

	obj, act := strings.ToUpper(in.GetService()), strings.ToUpper(in.GetMethod())
	allowed, err := s.casbin.Check(sub, obj, act)
	if err != nil {
		elog.Error("user internal permission check failed", elog.FieldErr(err))
		return nil, err
	}

	if !allowed {
		elog.Info("无权访问", elog.FieldValue("sub: "+sub+", obj: "+obj+", act: "+act))
	}

	return &userv1.CheckPermissionResponse{Allowed: allowed}, nil
}

// DeletePermissionPolicies 删除接口对应的权限策略。
func (s *InternalAuthGRPC) DeletePermissionPolicies(ctx context.Context, in *userv1.DeletePermissionPoliciesRequest) (*userv1.DeletePermissionPoliciesResponse, error) {
	policies := make([]store.RolePermissionPolicy, 0, len(in.GetPolicies()))
	for _, policy := range in.GetPolicies() {
		obj := strings.ToUpper(policy.GetService())
		act := strings.ToUpper(policy.GetMethod())
		if obj == "" || act == "" {
			continue
		}
		if _, err := s.casbin.Enforcer().RemoveFilteredPolicy(1, obj, act); err != nil {
			return nil, err
		}
		policies = append(policies, store.RolePermissionPolicy{
			Service: obj,
			Method:  act,
		})
	}
	if err := s.role.DeletePermissionPolicies(ctx, policies); err != nil {
		return nil, err
	}
	return &userv1.DeletePermissionPoliciesResponse{}, nil
}

func authContextToRPC(auth *authsession.AuthContext) *userv1.AuthContext {
	if auth == nil {
		return nil
	}
	return &userv1.AuthContext{
		UserId:            auth.UserID,
		Username:          auth.Username,
		UserType:          auth.UserType,
		DeptId:            auth.DeptID,
		Ua:                auth.UA,
		SessionId:         auth.SessionID,
		TokenId:           auth.TokenID,
		WorkspaceId:       auth.WorkspaceID,
		WorkspaceMemberId: auth.WorkspaceMemberID,
		IsBuiltinAdmin:    auth.IsBuiltinAdmin,
		Subject:           auth.Subject,
		IssuedAt:          timestamppb.New(auth.IssuedAt),
		ExpiresAt:         timestamppb.New(auth.ExpiresAt),
	}
}

func authContextFromRPC(auth *userv1.AuthContext) *authsession.AuthContext {
	if auth == nil {
		return nil
	}
	return &authsession.AuthContext{
		UserID:            auth.GetUserId(),
		Username:          auth.GetUsername(),
		UserType:          auth.GetUserType(),
		DeptID:            auth.GetDeptId(),
		UA:                auth.GetUa(),
		SessionID:         auth.GetSessionId(),
		TokenID:           auth.GetTokenId(),
		WorkspaceID:       auth.GetWorkspaceId(),
		WorkspaceMemberID: auth.GetWorkspaceMemberId(),
		IsBuiltinAdmin:    auth.GetIsBuiltinAdmin(),
		Subject:           auth.GetSubject(),
		IssuedAt:          auth.GetIssuedAt().AsTime(),
		ExpiresAt:         auth.GetExpiresAt().AsTime(),
	}
}
