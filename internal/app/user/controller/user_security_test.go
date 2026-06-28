package controller

import (
	"context"
	"strings"
	"testing"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
	"github.com/gotomicro/ego/core/eerrors"
)

func TestUserGRPC_AddUserRejectsSubmittedPassword(t *testing.T) {
	ctrl := &UserGRPC{}

	_, err := ctrl.AddUser(context.Background(), &userv1.AddUserRequest{
		User: &userv1.User{
			Username: "alice",
			Password: "legacy-password",
			Name:     "Alice",
			Phone:    "13800138000",
			Gender:   1,
		},
	})
	if err == nil {
		t.Fatal("expected AddUser to reject submitted password")
	}
	if !strings.Contains(err.Error(), "不支持提交密码") {
		t.Fatalf("AddUser() error = %q, want submitted password rejection", err.Error())
	}
}

func TestUserGRPC_AddUserRejectsShortPhoneForDefaultPassword(t *testing.T) {
	ctrl := &UserGRPC{}

	_, err := ctrl.AddUser(context.Background(), &userv1.AddUserRequest{
		User: &userv1.User{
			Username: "alice",
			Name:     "Alice",
			Phone:    "12345",
			Gender:   1,
		},
	})
	if err == nil {
		t.Fatal("expected AddUser to reject a short phone")
	}
	if !strings.Contains(err.Error(), "手机号长度不足") {
		t.Fatalf("AddUser() error = %q, want short phone rejection", err.Error())
	}
}

func TestMapUserDomainErrorMapsAuthSessionErrors(t *testing.T) {
	err := mapUserDomainError(context.Background(), authsession.ErrRefreshExpired)
	egoErr := eerrors.FromError(err)
	wantReason := eerrors.FromError(ecode.ErrorLoginExpired()).GetReason()
	if egoErr.GetReason() != wantReason {
		t.Fatalf("reason = %q, want %q", egoErr.GetReason(), wantReason)
	}
	if egoErr.GetMessage() != "登录已过期" {
		t.Fatalf("message = %q, want 登录已过期", egoErr.GetMessage())
	}
}
