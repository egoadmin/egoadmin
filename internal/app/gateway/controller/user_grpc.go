package controller

import (
	"context"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/gateway/application"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
)

type UserGRPC struct {
	client userclient.UserService
	api    *application.APIUseCase
}

func NewUserGRPCController(client *userclient.Client, api *application.APIUseCase) *UserGRPC {
	return &UserGRPC{client: client.User, api: api}
}

func (s *UserGRPC) Login(ctx context.Context, in *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	return s.client.Login(ctx, in)
}

func (s *UserGRPC) GetLoginCrypto(ctx context.Context, in *userv1.GetLoginCryptoRequest) (*userv1.GetLoginCryptoResponse, error) {
	return s.client.GetLoginCrypto(ctx, in)
}

func (s *UserGRPC) GetMenus(ctx context.Context, in *userv1.GetMenusRequest) (*userv1.GetMenusResponse, error) {
	return s.client.GetMenus(ctx, in)
}

func (s *UserGRPC) Logout(ctx context.Context, in *userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
	return s.client.Logout(ctx, in)
}

func (s *UserGRPC) GetCaptcha(ctx context.Context, in *userv1.GetCaptchaRequest) (*userv1.GetCaptchaResponse, error) {
	return s.client.GetCaptcha(ctx, in)
}

func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (*userv1.AddUserResponse, error) {
	return s.client.AddUser(ctx, in)
}

func (s *UserGRPC) DeleteUser(ctx context.Context, in *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	return s.client.DeleteUser(ctx, in)
}

func (s *UserGRPC) UpdateUser(ctx context.Context, in *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	return s.client.UpdateUser(ctx, in)
}

func (s *UserGRPC) GetUser(ctx context.Context, in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	return s.client.GetUser(ctx, in)
}

func (s *UserGRPC) UGetDept(ctx context.Context, in *userv1.UGetDeptRequest) (*userv1.UGetDeptResponse, error) {
	return s.client.UGetDept(ctx, in)
}

func (s *UserGRPC) GetUserList(ctx context.Context, in *userv1.GetUserListRequest) (*userv1.GetUserListResponse, error) {
	return s.client.GetUserList(ctx, in)
}

func (s *UserGRPC) GetAPIs(ctx context.Context, in *userv1.GetAPIsRequest) (*userv1.GetAPIsResponse, error) {
	apis, err := s.api.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	out := &userv1.GetAPIsResponse{
		Apis: make([]*userv1.API, 0, len(apis)),
	}
	for _, api := range apis {
		out.Apis = append(out.Apis, &userv1.API{
			Id:       api.ID,
			FullPath: api.FullPath,
		})
	}
	return out, nil
}

func (s *UserGRPC) ResetUserPassword(ctx context.Context, in *userv1.ResetUserPasswordRequest) (*userv1.ResetUserPasswordResponse, error) {
	return s.client.ResetUserPassword(ctx, in)
}

func (s *UserGRPC) HeartBeatUser(ctx context.Context, in *userv1.HeartBeatUserRequest) (*userv1.HeartBeatUserResponse, error) {
	return s.client.HeartBeatUser(ctx, in)
}

func (s *UserGRPC) GetOnlineUserList(ctx context.Context, in *userv1.GetOnlineUserListRequest) (*userv1.GetOnlineUserListResponse, error) {
	return s.client.GetOnlineUserList(ctx, in)
}

func (s *UserGRPC) OfflineUser(ctx context.Context, in *userv1.OfflineUserRequest) (*userv1.OfflineUserResponse, error) {
	return s.client.OfflineUser(ctx, in)
}
