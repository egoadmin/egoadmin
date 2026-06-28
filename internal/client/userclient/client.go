package userclient

import (
	"context"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/discovery"
	"github.com/google/wire"
	"github.com/gotomicro/ego/client/egrpc"
	"github.com/gotomicro/ego/core/eerrors"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ProviderSet = wire.NewSet(
	NewClient,
	NewInternalAuthService,
)

type LogService interface {
	GetLogList(context.Context, *userv1.GetLogListRequest) (*userv1.GetLogListResponse, error)
}

type UserService interface {
	Login(context.Context, *userv1.LoginRequest) (*userv1.LoginResponse, error)
	GetLoginCrypto(context.Context, *userv1.GetLoginCryptoRequest) (*userv1.GetLoginCryptoResponse, error)
	GetMenus(context.Context, *userv1.GetMenusRequest) (*userv1.GetMenusResponse, error)
	Logout(context.Context, *userv1.LogoutRequest) (*userv1.LogoutResponse, error)
	GetCaptcha(context.Context, *userv1.GetCaptchaRequest) (*userv1.GetCaptchaResponse, error)
	AddUser(context.Context, *userv1.AddUserRequest) (*userv1.AddUserResponse, error)
	DeleteUser(context.Context, *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error)
	UpdateUser(context.Context, *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error)
	GetUser(context.Context, *userv1.GetUserRequest) (*userv1.GetUserResponse, error)
	UGetDept(context.Context, *userv1.UGetDeptRequest) (*userv1.UGetDeptResponse, error)
	GetUserList(context.Context, *userv1.GetUserListRequest) (*userv1.GetUserListResponse, error)
	GetAPIs(context.Context, *userv1.GetAPIsRequest) (*userv1.GetAPIsResponse, error)
	ResetUserPassword(context.Context, *userv1.ResetUserPasswordRequest) (*userv1.ResetUserPasswordResponse, error)
	HeartBeatUser(context.Context, *userv1.HeartBeatUserRequest) (*userv1.HeartBeatUserResponse, error)
	GetOnlineUserList(context.Context, *userv1.GetOnlineUserListRequest) (*userv1.GetOnlineUserListResponse, error)
	OfflineUser(context.Context, *userv1.OfflineUserRequest) (*userv1.OfflineUserResponse, error)
}

type RoleService interface {
	AddRole(context.Context, *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error)
	DeleteRole(context.Context, *userv1.DeleteRoleRequest) (*userv1.DeleteRoleResponse, error)
	CheckDeleteRole(context.Context, *userv1.CheckDeleteRoleRequest) (*userv1.CheckDeleteRoleResponse, error)
	UpdateRole(context.Context, *userv1.UpdateRoleRequest) (*userv1.UpdateRoleResponse, error)
	GetRole(context.Context, *userv1.GetRoleRequest) (*userv1.GetRoleResponse, error)
	GetRoleList(context.Context, *userv1.GetRoleListRequest) (*userv1.GetRoleListResponse, error)
	GetRoleAll(context.Context, *userv1.GetRoleAllRequest) (*userv1.GetRoleAllResponse, error)
}

type DeptService interface {
	AddDept(context.Context, *userv1.AddDeptRequest) (*userv1.AddDeptResponse, error)
	DeleteDeptCascade(context.Context, *userv1.DeleteDeptCascadeRequest) (*userv1.DeleteDeptCascadeResponse, error)
	UpdateDept(context.Context, *userv1.UpdateDeptRequest) (*userv1.UpdateDeptResponse, error)
	UpdatePriorityDept(context.Context, *userv1.UpdatePriorityDeptRequest) (*userv1.UpdatePriorityDeptResponse, error)
	GetDeptByName(context.Context, *userv1.GetDeptByNameRequest) (*userv1.GetDeptByNameResponse, error)
	GetDept(context.Context, *userv1.GetDeptRequest) (*userv1.GetDeptResponse, error)
	GetDeptTop(context.Context, *userv1.GetDeptTopRequest) (*userv1.GetDeptTopResponse, error)
	GetDeptChild(context.Context, *userv1.GetDeptChildRequest) (*userv1.GetDeptChildResponse, error)
	CheckDeptDelete(context.Context, *userv1.CheckDeptDeleteRequest) (*userv1.CheckDeptDeleteResponse, error)
}

type CenterService interface {
	GetCenterInfo(context.Context, *userv1.GetCenterInfoRequest) (*userv1.GetCenterInfoResponse, error)
	EditCenterInfo(context.Context, *userv1.EditCenterInfoRequest) (*userv1.EditCenterInfoResponse, error)
	EditCenterPassword(context.Context, *userv1.EditCenterPasswordRequest) (*userv1.EditCenterPasswordResponse, error)
	EditCenterAvatar(context.Context, *userv1.EditCenterAvatarRequest) (*userv1.EditCenterAvatarResponse, error)
}

type InternalAuthService interface {
	ValidateAccessToken(context.Context, string) (*authsession.AuthContext, error)
	CheckPermission(context.Context, *authsession.AuthContext, string, string) (bool, error)
	DeletePermissionPolicies(context.Context, []PermissionPolicy) error
}

type PermissionPolicy struct {
	Service string
	Method  string
}

type Client struct {
	conn         *egrpc.Component
	Log          LogService
	User         UserService
	Role         RoleService
	Dept         DeptService
	Center       CenterService
	InternalAuth InternalAuthService
}

func NewClient(_ discovery.Ready) *Client {
	conn := egrpc.Load("client.grpc.user").Build()
	return &Client{
		conn:         conn,
		Log:          &logClient{client: userv1.NewLogServiceClient(conn.ClientConn)},
		User:         &userClient{client: userv1.NewUserServiceClient(conn.ClientConn)},
		Role:         &roleClient{client: userv1.NewRoleServiceClient(conn.ClientConn)},
		Dept:         &deptClient{client: userv1.NewDeptServiceClient(conn.ClientConn)},
		Center:       &centerClient{client: userv1.NewCenterServiceClient(conn.ClientConn)},
		InternalAuth: &internalAuthClient{client: userv1.NewInternalAuthServiceClient(conn.ClientConn)},
	}
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil || c.conn.ClientConn == nil {
		return nil
	}
	return c.conn.Close()
}

func NewInternalAuthService(client *Client) InternalAuthService {
	return client.InternalAuth
}

var forwardedMetadataKeys = map[string]struct{}{
	"authorization":     {},
	"accept-language":   {},
	"x-forwarded-for":   {},
	"x-forwarded-host":  {},
	"x-request-id":      {},
	"x-correlation-id":  {},
	"x-b3-traceid":      {},
	"x-b3-spanid":       {},
	"x-b3-parentspanid": {},
	"x-b3-sampled":      {},
	"x-b3-flags":        {},
	"traceparent":       {},
	"tracestate":        {},
	"grpc-trace-bin":    {},
	"uber-trace-id":     {},
	"jaeger-debug-id":   {},
	"jaeger-baggage":    {},
	"baggage":           {},
}

func outgoingContext(ctx context.Context) context.Context {
	incoming, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	merged := outgoing.Copy()
	for key, values := range incoming {
		key = strings.ToLower(key)
		if _, ok := forwardedMetadataKeys[key]; !ok {
			continue
		}
		merged[key] = values
	}
	return metadata.NewOutgoingContext(ctx, merged)
}

func normalizeResponse[T any](out T, err error) (T, error) {
	return out, normalizeEgoError(err)
}

func normalizeEgoError(err error) error {
	if err == nil {
		return nil
	}
	egoErr := eerrors.FromError(err)
	if egoErr.GetReason() == "" {
		return err
	}
	return egoErr
}

type logClient struct {
	client userv1.LogServiceClient
}

func (c *logClient) GetLogList(ctx context.Context, in *userv1.GetLogListRequest) (*userv1.GetLogListResponse, error) {
	return normalizeResponse(c.client.GetLogList(outgoingContext(ctx), in))
}

type userClient struct {
	client userv1.UserServiceClient
}

func (c *userClient) Login(ctx context.Context, in *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	return normalizeResponse(c.client.Login(outgoingContext(ctx), in))
}

func (c *userClient) GetLoginCrypto(ctx context.Context, in *userv1.GetLoginCryptoRequest) (*userv1.GetLoginCryptoResponse, error) {
	return normalizeResponse(c.client.GetLoginCrypto(outgoingContext(ctx), in))
}

func (c *userClient) GetMenus(ctx context.Context, in *userv1.GetMenusRequest) (*userv1.GetMenusResponse, error) {
	return normalizeResponse(c.client.GetMenus(outgoingContext(ctx), in))
}

func (c *userClient) Logout(ctx context.Context, in *userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
	return normalizeResponse(c.client.Logout(outgoingContext(ctx), in))
}

func (c *userClient) GetCaptcha(ctx context.Context, in *userv1.GetCaptchaRequest) (*userv1.GetCaptchaResponse, error) {
	return normalizeResponse(c.client.GetCaptcha(outgoingContext(ctx), in))
}

func (c *userClient) AddUser(ctx context.Context, in *userv1.AddUserRequest) (*userv1.AddUserResponse, error) {
	return normalizeResponse(c.client.AddUser(outgoingContext(ctx), in))
}

func (c *userClient) DeleteUser(ctx context.Context, in *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	return normalizeResponse(c.client.DeleteUser(outgoingContext(ctx), in))
}

func (c *userClient) UpdateUser(ctx context.Context, in *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	return normalizeResponse(c.client.UpdateUser(outgoingContext(ctx), in))
}

func (c *userClient) GetUser(ctx context.Context, in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	return normalizeResponse(c.client.GetUser(outgoingContext(ctx), in))
}

func (c *userClient) UGetDept(ctx context.Context, in *userv1.UGetDeptRequest) (*userv1.UGetDeptResponse, error) {
	return normalizeResponse(c.client.UGetDept(outgoingContext(ctx), in))
}

func (c *userClient) GetUserList(ctx context.Context, in *userv1.GetUserListRequest) (*userv1.GetUserListResponse, error) {
	return normalizeResponse(c.client.GetUserList(outgoingContext(ctx), in))
}

func (c *userClient) GetAPIs(ctx context.Context, in *userv1.GetAPIsRequest) (*userv1.GetAPIsResponse, error) {
	return normalizeResponse(c.client.GetAPIs(outgoingContext(ctx), in))
}

func (c *userClient) ResetUserPassword(ctx context.Context, in *userv1.ResetUserPasswordRequest) (*userv1.ResetUserPasswordResponse, error) {
	return normalizeResponse(c.client.ResetUserPassword(outgoingContext(ctx), in))
}

func (c *userClient) HeartBeatUser(ctx context.Context, in *userv1.HeartBeatUserRequest) (*userv1.HeartBeatUserResponse, error) {
	return normalizeResponse(c.client.HeartBeatUser(outgoingContext(ctx), in))
}

func (c *userClient) GetOnlineUserList(ctx context.Context, in *userv1.GetOnlineUserListRequest) (*userv1.GetOnlineUserListResponse, error) {
	return normalizeResponse(c.client.GetOnlineUserList(outgoingContext(ctx), in))
}

func (c *userClient) OfflineUser(ctx context.Context, in *userv1.OfflineUserRequest) (*userv1.OfflineUserResponse, error) {
	return normalizeResponse(c.client.OfflineUser(outgoingContext(ctx), in))
}

type roleClient struct {
	client userv1.RoleServiceClient
}

func (c *roleClient) AddRole(ctx context.Context, in *userv1.AddRoleRequest) (*userv1.AddRoleResponse, error) {
	return normalizeResponse(c.client.AddRole(outgoingContext(ctx), in))
}

func (c *roleClient) DeleteRole(ctx context.Context, in *userv1.DeleteRoleRequest) (*userv1.DeleteRoleResponse, error) {
	return normalizeResponse(c.client.DeleteRole(outgoingContext(ctx), in))
}

func (c *roleClient) CheckDeleteRole(ctx context.Context, in *userv1.CheckDeleteRoleRequest) (*userv1.CheckDeleteRoleResponse, error) {
	return normalizeResponse(c.client.CheckDeleteRole(outgoingContext(ctx), in))
}

func (c *roleClient) UpdateRole(ctx context.Context, in *userv1.UpdateRoleRequest) (*userv1.UpdateRoleResponse, error) {
	return normalizeResponse(c.client.UpdateRole(outgoingContext(ctx), in))
}

func (c *roleClient) GetRole(ctx context.Context, in *userv1.GetRoleRequest) (*userv1.GetRoleResponse, error) {
	return normalizeResponse(c.client.GetRole(outgoingContext(ctx), in))
}

func (c *roleClient) GetRoleList(ctx context.Context, in *userv1.GetRoleListRequest) (*userv1.GetRoleListResponse, error) {
	return normalizeResponse(c.client.GetRoleList(outgoingContext(ctx), in))
}

func (c *roleClient) GetRoleAll(ctx context.Context, in *userv1.GetRoleAllRequest) (*userv1.GetRoleAllResponse, error) {
	return normalizeResponse(c.client.GetRoleAll(outgoingContext(ctx), in))
}

type deptClient struct {
	client userv1.DeptServiceClient
}

func (c *deptClient) AddDept(ctx context.Context, in *userv1.AddDeptRequest) (*userv1.AddDeptResponse, error) {
	return normalizeResponse(c.client.AddDept(outgoingContext(ctx), in))
}

func (c *deptClient) DeleteDeptCascade(ctx context.Context, in *userv1.DeleteDeptCascadeRequest) (*userv1.DeleteDeptCascadeResponse, error) {
	return normalizeResponse(c.client.DeleteDeptCascade(outgoingContext(ctx), in))
}

func (c *deptClient) UpdateDept(ctx context.Context, in *userv1.UpdateDeptRequest) (*userv1.UpdateDeptResponse, error) {
	return normalizeResponse(c.client.UpdateDept(outgoingContext(ctx), in))
}

func (c *deptClient) UpdatePriorityDept(ctx context.Context, in *userv1.UpdatePriorityDeptRequest) (*userv1.UpdatePriorityDeptResponse, error) {
	return normalizeResponse(c.client.UpdatePriorityDept(outgoingContext(ctx), in))
}

func (c *deptClient) GetDeptByName(ctx context.Context, in *userv1.GetDeptByNameRequest) (*userv1.GetDeptByNameResponse, error) {
	return normalizeResponse(c.client.GetDeptByName(outgoingContext(ctx), in))
}

func (c *deptClient) GetDept(ctx context.Context, in *userv1.GetDeptRequest) (*userv1.GetDeptResponse, error) {
	return normalizeResponse(c.client.GetDept(outgoingContext(ctx), in))
}

func (c *deptClient) GetDeptTop(ctx context.Context, in *userv1.GetDeptTopRequest) (*userv1.GetDeptTopResponse, error) {
	return normalizeResponse(c.client.GetDeptTop(outgoingContext(ctx), in))
}

func (c *deptClient) GetDeptChild(ctx context.Context, in *userv1.GetDeptChildRequest) (*userv1.GetDeptChildResponse, error) {
	return normalizeResponse(c.client.GetDeptChild(outgoingContext(ctx), in))
}

func (c *deptClient) CheckDeptDelete(ctx context.Context, in *userv1.CheckDeptDeleteRequest) (*userv1.CheckDeptDeleteResponse, error) {
	return normalizeResponse(c.client.CheckDeptDelete(outgoingContext(ctx), in))
}

type centerClient struct {
	client userv1.CenterServiceClient
}

func (c *centerClient) GetCenterInfo(ctx context.Context, in *userv1.GetCenterInfoRequest) (*userv1.GetCenterInfoResponse, error) {
	return normalizeResponse(c.client.GetCenterInfo(outgoingContext(ctx), in))
}

func (c *centerClient) EditCenterInfo(ctx context.Context, in *userv1.EditCenterInfoRequest) (*userv1.EditCenterInfoResponse, error) {
	return normalizeResponse(c.client.EditCenterInfo(outgoingContext(ctx), in))
}

func (c *centerClient) EditCenterPassword(ctx context.Context, in *userv1.EditCenterPasswordRequest) (*userv1.EditCenterPasswordResponse, error) {
	return normalizeResponse(c.client.EditCenterPassword(outgoingContext(ctx), in))
}

func (c *centerClient) EditCenterAvatar(ctx context.Context, in *userv1.EditCenterAvatarRequest) (*userv1.EditCenterAvatarResponse, error) {
	return normalizeResponse(c.client.EditCenterAvatar(outgoingContext(ctx), in))
}

type internalAuthClient struct {
	client userv1.InternalAuthServiceClient
}

func (c *internalAuthClient) ValidateAccessToken(ctx context.Context, accessToken string) (*authsession.AuthContext, error) {
	out, err := c.client.ValidateAccessToken(outgoingContext(ctx), &userv1.ValidateAccessTokenRequest{
		AccessToken: accessToken,
	})
	if err != nil {
		return nil, normalizeEgoError(err)
	}
	return authContextFromRPC(out.GetAuth()), nil
}

func (c *internalAuthClient) CheckPermission(ctx context.Context, auth *authsession.AuthContext, service string, method string) (bool, error) {
	out, err := c.client.CheckPermission(outgoingContext(ctx), &userv1.CheckPermissionRequest{
		Auth:    authContextToRPC(auth),
		Service: service,
		Method:  method,
	})
	if err != nil {
		return false, normalizeEgoError(err)
	}
	return out.GetAllowed(), nil
}

func (c *internalAuthClient) DeletePermissionPolicies(ctx context.Context, policies []PermissionPolicy) error {
	in := &userv1.DeletePermissionPoliciesRequest{
		Policies: make([]*userv1.PermissionPolicy, 0, len(policies)),
	}
	for _, policy := range policies {
		in.Policies = append(in.Policies, &userv1.PermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}
	_, err := c.client.DeletePermissionPolicies(outgoingContext(ctx), in)
	return normalizeEgoError(err)
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
