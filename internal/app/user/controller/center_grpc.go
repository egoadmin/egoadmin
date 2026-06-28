package controller

import (
	"context"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/jinzhu/copier"
)

// CenterGrpc 用户grpc
type CenterGRPC struct {
	logger auditlog.Loger
	user   *service.UserService
	dept   *service.DeptService
}

// NewCenterGRPCController 实例化用户grpc
func NewCenterGRPCController(user *service.UserService, dept *service.DeptService, logger auditlog.Loger) *CenterGRPC {
	return &CenterGRPC{
		logger: logger,
		user:   user,
		dept:   dept,
	}
}

// GetCenterInfo 获取用户资料
func (s *CenterGRPC) GetCenterInfo(ctx context.Context, in *userv1.GetCenterInfoRequest) (out *userv1.GetCenterInfoResponse, err error) {
	out = &userv1.GetCenterInfoResponse{
		User: &userv1.User{},
	}

	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}

	user, err := s.user.GetSelfUser(ctx, auth.UserID)
	if err != nil {
		return
	}

	if err = copier.Copy(&out.User, &user); err != nil {
		return
	}

	if user.DeptID == 0 {
		return
	}

	out.User.DeptName = strings.Join(userFindDeptNames(ctx, s.dept, user.DeptID), ">")

	return
}

// EditCenterInfo 修改个人资料
func (s *CenterGRPC) EditCenterInfo(ctx context.Context, in *userv1.EditCenterInfoRequest) (out *userv1.EditCenterInfoResponse, err error) {
	out = &userv1.EditCenterInfoResponse{}

	defer func() {
		if err == nil {
			action := "编辑个人基本信息"
			if in.GetPhone() != "" {
				action = "编辑个人手机号"
			}
			s.logger.Save(ctx, "用户-个人中心", "编辑", action, in)
		}
	}()
	if in.GetPhone() != "" && (in.GetPasswordCipher() == "" || in.GetKeyId() == "" || in.GetChallengeId() == "") {
		err = platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)

		return
	}
	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}
	// 检查姓名是否合法
	if in.GetName() != "" {
		// 系统内置用户不允许修改名称
		if (auth.Username == store.UserModelUsernameAdmin || auth.Username == store.UserModelUsernameRoot) && in.GetName() != auth.Username {
			err = platformi18n.ErrorFailed(ctx, "BuiltinUserNameChangeDenied", nil)

			return
		}
	}
	password := ""
	if in.GetPhone() != "" {
		payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
			KeyID:          in.GetKeyId(),
			ChallengeID:    in.GetChallengeId(),
			Username:       auth.Username,
			UA:             auth.UA,
			PasswordCipher: in.GetPasswordCipher(),
			Action:         logincrypto.ActionCenterEditInfo,
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
	err = s.user.EditCenterInfo(ctx, auth.UserID, in.GetName(), in.GetGender(), in.GetPhone(), password)

	return
}

// EditCenterPassword 修改密码
func (s *CenterGRPC) EditCenterPassword(ctx context.Context, in *userv1.EditCenterPasswordRequest) (out *userv1.EditCenterPasswordResponse, err error) {
	out = &userv1.EditCenterPasswordResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户-个人中心", "编辑", "编辑个人密码", in)
		}
	}()

	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}

	payload, er := s.user.DecryptLoginPayload(ctx, logincrypto.DecryptRequest{
		KeyID:          in.GetKeyId(),
		ChallengeID:    in.GetChallengeId(),
		Username:       auth.Username,
		UA:             auth.UA,
		PasswordCipher: in.GetPasswordCipher(),
		Action:         logincrypto.ActionCenterEditPassword,
	})
	if er != nil {
		err = er
		return
	}
	if payload.OldPassword == "" || payload.NewPassword == "" {
		err = platformi18n.ErrorFailed(ctx, "InvalidLoginParams", nil)
		return
	}
	err = s.user.UpdateUserPassword(ctx, auth.UserID, payload.OldPassword, payload.NewPassword)

	return
}

// EditCenterAvatar 修改头像
func (s *CenterGRPC) EditCenterAvatar(ctx context.Context, in *userv1.EditCenterAvatarRequest) (out *userv1.EditCenterAvatarResponse, err error) {
	out = &userv1.EditCenterAvatarResponse{}
	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户-个人中心", "编辑", "编辑个人基本信息", in)
		}
	}()

	auth, ok := authsession.FromContext(ctx)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "AuthMissingToken", nil)

		return
	}

	err = s.user.SaveAvatar(ctx, auth.UserID, in.GetReferenceId())

	return
}
