package controller

import (
	"context"
	"errors"
	"math"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/application"
	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/metadata"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/egoadmin/elib/pkg/util/xtime"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
)

// UserGrpc 用户grpc
type UserGRPC struct {
	logger      auditlog.Loger
	user        *service.UserService
	userUseCase *application.UserUseCase
	dept        *service.DeptService
}

// NewUserGRPCController 实例化用户grpc
func NewUserGRPCController(user *service.UserService, userUseCase *application.UserUseCase, dept *service.DeptService, logger auditlog.Loger) *UserGRPC {
	return &UserGRPC{
		logger:      logger,
		user:        user,
		userUseCase: userUseCase,
		dept:        dept,
	}
}

// Login 登录
func (s *UserGRPC) Login(ctx context.Context, in *userv1.LoginRequest) (out *userv1.LoginResponse, err error) {
	out = &userv1.LoginResponse{}
	if (in.GetUsername() == "" || in.GetPasswordCipher() == "" || in.GetKeyId() == "" || in.GetChallengeId() == "") && in.GetToken() == "" {
		err = platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)

		return
	}
	if in.GetToken() == "" && s.user.Conf.User().UseCaptcha && !s.user.Captcha.VerifyString(in.GetCaptchaId(), in.GetCaptchaCode()) {
		err = platformi18n.ErrorFailed(ctx, "InvalidCaptcha", nil)

		return
	}
	password := ""
	if in.GetToken() == "" {
		payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
			KeyID:          in.GetKeyId(),
			ChallengeID:    in.GetChallengeId(),
			Username:       in.GetUsername(),
			UA:             in.GetUa(),
			PasswordCipher: in.GetPasswordCipher(),
			Action:         logincrypto.ActionLogin,
		})
		if er != nil {
			err = er
			return
		}
		password = payload.Password
		if password == "" {
			err = platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)
			return
		}
	}
	resp, err := s.userUseCase.Login(ctx, application.LoginCommand{
		Username:     in.GetUsername(),
		Password:     password,
		UA:           in.GetUa(),
		RefreshToken: in.GetToken(),
		IP:           metadata.ExtractIncoming(ctx).Get("x-forwarded-for"),
	})
	err = mapUserDomainError(ctx, err)
	if err != nil {
		return
	}
	// 非续期登录
	if in.GetToken() == "" {
		s.logger.Save(authsession.NewContext(ctx, resp.Auth), "用户-登录退出", "登录", "登录", in)
	}

	out = &userv1.LoginResponse{
		UserTyp:      resp.UserType,
		Menus:        resp.Menus,
		ExpiredAt:    xtime.Time2Ts(resp.ExpiresAt),
		Token:        resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}

	return
}

// GetLoginCrypto 获取登录加密参数
func (s *UserGRPC) GetLoginCrypto(ctx context.Context, in *userv1.GetLoginCryptoRequest) (out *userv1.GetLoginCryptoResponse, err error) {
	out = &userv1.GetLoginCryptoResponse{}
	challenge, err := s.user.GetLoginCrypto(ctx, in.GetUsername(), in.GetUa(), in.GetAction())
	if err != nil {
		return
	}
	out.KeyId = challenge.KeyID
	out.PublicKey = challenge.PublicKey
	out.ChallengeId = challenge.ChallengeID
	out.Nonce = challenge.Nonce
	out.Algorithm = challenge.Algorithm
	out.ExpiresAt = challenge.ExpiresAt
	return
}

// GetMenus 获取菜单
func (s *UserGRPC) GetMenus(ctx context.Context, in *userv1.GetMenusRequest) (out *userv1.GetMenusResponse, err error) {
	out = &userv1.GetMenusResponse{}
	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}
	out.Menus, err = s.user.GetMenus(ctx, auth.UserID)

	return
}

// Logout 退出登录
func (s *UserGRPC) Logout(ctx context.Context, in *userv1.LogoutRequest) (out *userv1.LogoutResponse, err error) {
	out = &userv1.LogoutResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户-登录退出", "登出", "登出", in)
		}
	}()

	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}

	// 更新登录状态
	err = s.user.User.UpdateBaseWithoutHook(ctx, auth.UserID, &store.UserModel{
		UserOnline: store.UserModelOffline,
	})
	if err != nil {
		return
	}
	// 清除用户token
	err = s.user.Auth.Logout(ctx, auth)

	return
}

// GetCaptcha 获取验证码
func (s *UserGRPC) GetCaptcha(ctx context.Context, in *userv1.GetCaptchaRequest) (out *userv1.GetCaptchaResponse, err error) {
	out = &userv1.GetCaptchaResponse{}
	out.Id, out.CaptchaImg, err = s.user.Captcha.GetVerifyImgString()

	return
}

// AddUser 新增后台用户
func (s *UserGRPC) AddUser(ctx context.Context, in *userv1.AddUserRequest) (out *userv1.AddUserResponse, err error) {
	out = &userv1.AddUserResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-用户", "新增", "新增用户", in)
		}
	}()
	if in.GetUser().GetPassword() != "" {
		err = mapUserDomainError(ctx, application.ErrSubmittedPassword)
		return
	}

	gender, ok := store.UserModelGenderToInt8(in.GetUser().GetGender())
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
		return
	}
	if _, err = userdomain.DefaultPasswordFromPhone(in.GetUser().GetPhone()); err != nil {
		err = mapUserDomainError(ctx, err)
		return
	}
	user := &store.UserModel{
		Username:   in.GetUser().GetUsername(),
		Name:       in.GetUser().GetName(),
		Phone:      lo.ToPtr(in.GetUser().GetPhone()),
		Gender:     gender,
		UserStatus: in.GetUser().GetUserStatus(),
		DeptID:     in.GetUser().GetDeptId(),
		Remark:     in.GetUser().GetRemark(),
		Roles:      userRolesFromIDs(in.GetRoleIds()),
	}
	err = mapUserDomainError(ctx, s.user.AddUser(ctx, user))
	if err != nil {
		return
	}
	out.Id = user.ID

	return
}

// DeleteUser 删除管理员
func (s *UserGRPC) DeleteUser(ctx context.Context, in *userv1.DeleteUserRequest) (out *userv1.DeleteUserResponse, err error) {
	out = &userv1.DeleteUserResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-用户", "删除", "删除用户", in)
		}
	}()

	err = s.user.DeleteUser(ctx, in.GetIds())

	return
}

// UpdateUser 修改用户
func (s *UserGRPC) UpdateUser(ctx context.Context, in *userv1.UpdateUserRequest) (out *userv1.UpdateUserResponse, err error) {
	out = &userv1.UpdateUserResponse{}
	// utName := "用户"
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-用户", "编辑", "编辑用户", in)
		}
	}()

	phone := in.GetPhone()
	gender, ok := store.UserModelGenderToInt8(in.GetGender())
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
		return
	}

	err = mapUserDomainError(ctx, s.user.UpdateUser(ctx, in.GetId(), &store.UserModel{
		Name:       in.GetName(),
		Phone:      lo.ToPtr(phone),
		Gender:     gender,
		UserStatus: in.GetUserStatus(),
		DeptID:     in.GetDeptId(),
		Roles:      userRolesFromIDs(in.GetRoleIds()),
	}))

	return
}

// GetUser 查询用户
func (s *UserGRPC) GetUser(ctx context.Context, in *userv1.GetUserRequest) (out *userv1.GetUserResponse, err error) {
	out = &userv1.GetUserResponse{
		User: &userv1.User{},
	}

	user, err := s.user.GetUser(ctx, in.GetId())
	if err != nil {
		return
	}

	err = copier.Copy(&out.User, &user)

	return
}

// UGetDept 根据组织名称获取组织
//
// 组织名称为空时返回所有组织
// ID不为空时精确查找
func (s *UserGRPC) UGetDept(ctx context.Context, in *userv1.UGetDeptRequest) (out *userv1.UGetDeptResponse, err error) {
	out = &userv1.UGetDeptResponse{Depts: []*userv1.Dept{}}
	fdepts, err := s.dept.GetDeptByName(ctx, in.GetName(), in.GetId())
	if err != nil {
		return
	}
	err = copier.Copy(&out.Depts, &fdepts)

	return
}

// GetUserList 查询用户列表
func (s *UserGRPC) GetUserList(ctx context.Context, in *userv1.GetUserListRequest) (out *userv1.GetUserListResponse, err error) {
	out = &userv1.GetUserListResponse{
		Users: make([]*userv1.User, 0),
	}

	opt := store.UserModelGetListOption{
		Pgopt: xorm.PaginateOption{
			Page:  int(in.GetPage()),
			Limit: int(in.GetLimit()),
			Sort:  in.GetSort(),
			Order: in.GetOrder(),
		},
		UserStatus: in.GetUserStatus(),
		RoleID:     in.GetRoleId(),
		Phone:      in.GetPhone(),
	}
	if in.GetDeptId() != 0 {
		opt.Deptids = []uint64{in.GetDeptId()}
	}

	users, total, err := s.user.GetUserListNameOrUserName(ctx, opt, in.GetName())
	if err != nil {
		return
	}

	if total > math.MaxInt32 {
		err = platformi18n.ErrorFailed(ctx, "UserCountExceeded", nil)
		return
	}
	//nolint:gosec // total is checked above and fits int32.
	out.Total = int32(total)

	if err = copier.Copy(&out.Users, &users); err != nil {
		return
	}
	lo.ForEach(out.Users, func(item *userv1.User, index int) {
		if item.DeptId == 0 {
			return
		}
		out.Users[index].DeptName = strings.Join(userFindDeptNames(ctx, s.dept, item.DeptId), ">")
	})

	return
}

// GetAPIs 获取系统接口
func (s *UserGRPC) GetAPIs(ctx context.Context, in *userv1.GetAPIsRequest) (out *userv1.GetAPIsResponse, err error) {
	out = &userv1.GetAPIsResponse{
		Apis: make([]*userv1.API, 0),
	}

	return
}

// ResetUserPassword 重置密码
func (s *UserGRPC) ResetUserPassword(ctx context.Context, in *userv1.ResetUserPasswordRequest) (out *userv1.ResetUserPasswordResponse, err error) {
	out = &userv1.ResetUserPasswordResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-用户", "编辑", "重置密码", in)
		}
	}()

	err = mapUserDomainError(ctx, s.user.ResetUserPassword(ctx, in.GetId()))

	return
}

func mapUserDomainError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrSubmittedPassword):
		return platformi18n.ErrorFailed(ctx, "SubmittedPasswordUnsupported", nil)
	case errors.Is(err, userdomain.ErrBuiltinUsername):
		return platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil)
	case errors.Is(err, userdomain.ErrUsernameExists):
		return platformi18n.ErrorFailed(ctx, "UsernameExists", nil)
	case errors.Is(err, userdomain.ErrPhoneExists):
		return platformi18n.ErrorFailed(ctx, "PhoneExists", nil)
	case errors.Is(err, application.ErrLoginUARequired):
		return platformi18n.ErrorFailed(ctx, "LoginInfoAbnormal", nil)
	case errors.Is(err, userdomain.ErrLoginDenied):
		return platformi18n.ErrorFailed(ctx, "LoginFailed", nil)
	case authsession.IsSessionError(err):
		return authsession.ToEcodeContext(ctx, err)
	case errors.Is(err, userdomain.ErrBuiltinPasswordReset):
		return platformi18n.ErrorFailed(ctx, "BuiltinPasswordResetDenied", nil)
	case errors.Is(err, userdomain.ErrInvalidDefaultPasswd):
		return platformi18n.ErrorFailed(ctx, "DefaultPasswordPhoneTooShort", nil)
	default:
		return err
	}
}

func userRolesFromIDs(ids []uint64) []store.RoleModel {
	roles := make([]store.RoleModel, 0, len(ids))
	for _, id := range ids {
		roles = append(roles, store.RoleModel{Model: xorm.Model{ID: id}})
	}
	return roles
}

// HeartBeatUser 用户心跳
//
// 心跳用于刷新在线状态；心跳超时默认只影响在线列表，不撤销登录态
func (s *UserGRPC) HeartBeatUser(ctx context.Context, in *userv1.HeartBeatUserRequest) (out *userv1.HeartBeatUserResponse, err error) {
	out = &userv1.HeartBeatUserResponse{}
	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}

	err = s.userUseCase.MarkUserOnline(ctx, auth.UserID)

	return
}

// GetOnlineUserList 查询在线用户列表
func (s *UserGRPC) GetOnlineUserList(ctx context.Context, in *userv1.GetOnlineUserListRequest) (out *userv1.GetOnlineUserListResponse, err error) {
	out = &userv1.GetOnlineUserListResponse{
		Users: make([]*userv1.User, 0),
	}

	opt := store.UserModelGetListOption{
		Pgopt: xorm.PaginateOption{
			Page:  int(in.GetPage()),
			Limit: int(in.GetLimit()),
			Sort:  in.GetSort(),
			Order: in.GetOrder(),
		},
		UserOnline:  store.UserModelOnline,
		LastLoginIP: in.GetLastLoginIp(),
	}

	users, total, err := s.user.GetUserListUserName(ctx, opt, in.GetName())
	if err != nil {
		return
	}

	if total > math.MaxInt32 {
		err = platformi18n.ErrorFailed(ctx, "UserCountExceeded", nil)
		return
	}
	//nolint:gosec // total is checked above and fits int32.
	out.Total = int32(total)

	if err = copier.Copy(&out.Users, &users); err != nil {
		return
	}

	return
}

// OfflineUser 下线用户
func (s *UserGRPC) OfflineUser(ctx context.Context, in *userv1.OfflineUserRequest) (out *userv1.OfflineUserResponse, err error) {
	out = &userv1.OfflineUserResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-在线用户", "强退", "强退用户", in)
		}
	}()

	uids := in.GetIds()
	if len(uids) == 0 {
		return
	}
	err = s.user.ForceOfflineUsers(ctx, uids)

	return
}
