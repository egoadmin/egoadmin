package application

import (
	"context"
	"testing"
	"testing/fstest"

	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
)

type fakeAPIRepository struct {
	apis []*apidomain.API
	err  error
}

func (f fakeAPIRepository) CreateBatch(context.Context, []*apidomain.API) error {
	return nil
}

func (f fakeAPIRepository) Update(context.Context, uint64, *apidomain.API) error {
	return nil
}

func (f fakeAPIRepository) DeleteByIDs(context.Context, []uint64) error {
	return nil
}

func (f fakeAPIRepository) FindAll(context.Context) ([]*apidomain.API, error) {
	return f.apis, f.err
}

func (f fakeAPIRepository) FindAllBySign(context.Context, []string) ([]*apidomain.API, error) {
	return nil, nil
}

func TestPermissionUseCase_ValidateRoleAPIBoundary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	apiGetRoleList := &apidomain.API{
		ID:     1,
		Name:   "GETROLELIST",
		Path:   "USER.V1.ROLESERVICE",
		Method: "GETROLELIST",
	}
	apiDeleteUser := &apidomain.API{
		ID:     2,
		Name:   "DELETEUSER",
		Path:   "USER.V1.USERSERVICE",
		Method: "DELETEUSER",
	}

	tests := []struct {
		name       string
		viewMenus  string
		apiIDs     []uint64
		skip       bool
		wantErr    bool
		wantPolicy []permissiondomain.Policy
	}{
		{
			name:       "allowed api in selected menu",
			viewMenus:  "20101",
			apiIDs:     []uint64{apiGetRoleList.ID},
			wantPolicy: []permissiondomain.Policy{{Service: apiGetRoleList.Path, Method: apiGetRoleList.Method}},
		},
		{
			name:      "api outside selected menu is rejected",
			viewMenus: "20101",
			apiIDs:    []uint64{apiDeleteUser.ID},
			wantErr:   true,
		},
		{
			name:      "unknown api id is rejected",
			viewMenus: "20101",
			apiIDs:    []uint64{999},
			wantErr:   true,
		},
		{
			name:      "empty menu rejects api grants",
			viewMenus: "",
			apiIDs:    []uint64{apiGetRoleList.ID},
			wantErr:   true,
		},
		{
			name:       "skip config bypasses contract check",
			viewMenus:  "",
			apiIDs:     []uint64{apiDeleteUser.ID},
			skip:       true,
			wantPolicy: []permissiondomain.Policy{{Service: apiDeleteUser.Path, Method: apiDeleteUser.Method}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conf := &config.Config{}
			conf.SetSkipPermissionContractCheckForTest(tt.skip)

			uc := NewPermissionUseCase(PermissionOptions{
				Conf:          conf,
				APIRepository: fakeAPIRepository{apis: []*apidomain.API{apiGetRoleList, apiDeleteUser}},
				FrontendAssetsFS: fstest.MapFS{
					permissionContractPath: &fstest.MapFile{Data: []byte(`{
  "20101": {
    "name": "role list",
    "apis": ["USER.V1.ROLESERVICE/GETROLELIST"]
  }
}`)},
				},
			})

			gotPolicy, err := uc.ValidateRoleAPIBoundary(ctx, tt.viewMenus, tt.apiIDs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRoleAPIBoundary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(tt.wantPolicy) != len(gotPolicy) {
				t.Fatalf("ValidateRoleAPIBoundary() policies = %+v, want %+v", gotPolicy, tt.wantPolicy)
			}
			for i := range tt.wantPolicy {
				if gotPolicy[i] != tt.wantPolicy[i] {
					t.Fatalf("ValidateRoleAPIBoundary() policies = %+v, want %+v", gotPolicy, tt.wantPolicy)
				}
			}
		})
	}
}

func TestPermissionUseCase_ValidateRoleAPIBoundaryLocalizesError(t *testing.T) {
	t.Parallel()

	apiDeleteUser := &apidomain.API{
		ID:     2,
		Name:   "DELETEUSER",
		Path:   "USER.V1.USERSERVICE",
		Method: "DELETEUSER",
	}
	uc := NewPermissionUseCase(PermissionOptions{
		Conf:          &config.Config{},
		APIRepository: fakeAPIRepository{apis: []*apidomain.API{apiDeleteUser}},
		FrontendAssetsFS: fstest.MapFS{
			permissionContractPath: &fstest.MapFile{Data: []byte(`{
  "20101": {
    "name": "role list",
    "apis": ["USER.V1.ROLESERVICE/GETROLELIST"]
  }
}`)},
		},
	})

	ctx := platformi18n.WithAcceptLanguage(context.Background(), "en-US,en;q=0.9")
	_, err := uc.ValidateRoleAPIBoundary(ctx, "20101", []uint64{apiDeleteUser.ID})
	if err == nil {
		t.Fatal("ValidateRoleAPIBoundary() error = nil, want localized error")
	}
	got, ok := UserMessage(err)
	if !ok {
		t.Fatalf("ValidateRoleAPIBoundary() error type = %T, want UserMessageError", err)
	}
	want := `Security block: selected function module does not include API permission "DELETEUSER", so it cannot be assigned`
	if got != want {
		t.Fatalf("UserMessage() = %q, want %q", got, want)
	}
}
