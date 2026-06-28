package server

import (
	"context"
	"errors"
	"testing"

	"github.com/egoadmin/egoadmin/internal/app/gateway/controller"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/elib/pkg/constant"
	transportgrpc "github.com/egoadmin/elib/pkg/transport/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeInternalAuthService struct {
	validateCalls int
	validateToken string
	validateAuth  *authsession.AuthContext
	validateErr   error

	checkCalls   int
	checkAuth    *authsession.AuthContext
	checkSvc     string
	checkMethod  string
	checkAllowed bool
	checkErr     error
}

func (s *fakeInternalAuthService) ValidateAccessToken(ctx context.Context, token string) (*authsession.AuthContext, error) {
	s.validateCalls++
	s.validateToken = token
	return s.validateAuth, s.validateErr
}

func (s *fakeInternalAuthService) CheckPermission(ctx context.Context, auth *authsession.AuthContext, svc string, method string) (bool, error) {
	s.checkCalls++
	s.checkAuth = auth
	s.checkSvc = svc
	s.checkMethod = method
	return s.checkAllowed, s.checkErr
}

func (s *fakeInternalAuthService) DeletePermissionPolicies(context.Context, []userclient.PermissionPolicy) error {
	return nil
}

func TestRemoteAuthContextSkipsPublicMethods(t *testing.T) {
	fake := &fakeInternalAuthService{}
	opts := controller.Options{UserClient: &userclient.Client{InternalAuth: fake}}
	ctx := transportgrpc.NewContext(context.Background(), "/user.v1.UserService/GetCaptcha")

	got, err := remoteAuthContext(ctx, opts)
	if err != nil {
		t.Fatalf("remoteAuthContext() error = %v", err)
	}
	if got != ctx {
		t.Fatalf("remoteAuthContext() returned different context for public method")
	}
	if fake.validateCalls != 0 {
		t.Fatalf("ValidateAccessToken calls = %d, want 0", fake.validateCalls)
	}
}

func TestRemoteAuthContextUsesUserService(t *testing.T) {
	wantAuth := &authsession.AuthContext{UserID: 10, Username: "admin", Subject: "admin"}
	fake := &fakeInternalAuthService{validateAuth: wantAuth}
	opts := controller.Options{UserClient: &userclient.Client{InternalAuth: fake}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.MDHeaderAuthorize, "Bearer access-token",
	))
	ctx = transportgrpc.NewContext(ctx, "/user.v1.UserService/AddUser")

	got, err := remoteAuthContext(ctx, opts)
	if err != nil {
		t.Fatalf("remoteAuthContext() error = %v", err)
	}
	if fake.validateCalls != 1 {
		t.Fatalf("ValidateAccessToken calls = %d, want 1", fake.validateCalls)
	}
	if fake.validateToken != "access-token" {
		t.Fatalf("ValidateAccessToken token = %q, want access-token", fake.validateToken)
	}
	auth, ok := authsession.FromContext(got)
	if !ok {
		t.Fatal("auth context missing")
	}
	if auth.UserID != wantAuth.UserID || auth.Username != wantAuth.Username {
		t.Fatalf("auth context = %#v, want %#v", auth, wantAuth)
	}
}

func TestPermCheckFuncUsesUserService(t *testing.T) {
	auth := &authsession.AuthContext{UserID: 10, Username: "admin", Subject: "admin"}
	fake := &fakeInternalAuthService{checkAllowed: true}
	opts := controller.Options{UserClient: &userclient.Client{InternalAuth: fake}}
	ctx := authsession.NewContext(context.Background(), auth)
	ctx = transportgrpc.NewContext(ctx, "/user.v1.RoleService/AddRole")

	ok, err := permCheckFunc(opts)(ctx)
	if err != nil {
		t.Fatalf("permCheckFunc() error = %v", err)
	}
	if !ok {
		t.Fatal("permCheckFunc() allowed = false, want true")
	}
	if fake.checkCalls != 1 {
		t.Fatalf("CheckPermission calls = %d, want 1", fake.checkCalls)
	}
	if fake.checkAuth != auth {
		t.Fatalf("CheckPermission auth = %#v, want %#v", fake.checkAuth, auth)
	}
	if fake.checkSvc != "user.v1.RoleService" || fake.checkMethod != "AddRole" {
		t.Fatalf("CheckPermission target = %s/%s, want user.v1.RoleService/AddRole", fake.checkSvc, fake.checkMethod)
	}
}

func TestPermCheckFuncSkipsLoginOnlyMethods(t *testing.T) {
	fake := &fakeInternalAuthService{checkErr: errors.New("must not be called")}
	opts := controller.Options{UserClient: &userclient.Client{InternalAuth: fake}}
	ctx := transportgrpc.NewContext(context.Background(), "/user.v1.UserService/GetMenus")

	ok, err := permCheckFunc(opts)(ctx)
	if err != nil {
		t.Fatalf("permCheckFunc() error = %v", err)
	}
	if !ok {
		t.Fatal("permCheckFunc() allowed = false, want true")
	}
	if fake.checkCalls != 0 {
		t.Fatalf("CheckPermission calls = %d, want 0", fake.checkCalls)
	}
}

func TestExtractBearerTokenFromValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "valid bearer", value: "Bearer access-token", want: "access-token"},
		{name: "lowercase bearer", value: "bearer access-token", want: "access-token"},
		{name: "empty", value: "", wantErr: true},
		{name: "missing token", value: "Bearer ", wantErr: true},
		{name: "wrong scheme", value: "Basic access-token", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBearerTokenFromValue(context.Background(), tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("extractBearerTokenFromValue() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("extractBearerTokenFromValue() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("extractBearerTokenFromValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
