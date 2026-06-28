package service

import (
	"context"
	"strings"
	"testing"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWidestDataScope(t *testing.T) {
	tests := []struct {
		name  string
		roles []store.RoleModel
		want  DataScopeLevel
	}{
		{name: "no roles defaults self", want: DataScopeSelf},
		{
			name: "single dept self",
			roles: []store.RoleModel{
				{DataPerm: store.RoleModelDataPermUserDeptSelf},
			},
			want: DataScopeDeptSelf,
		},
		{
			name: "multi role uses widest",
			roles: []store.RoleModel{
				{DataPerm: store.RoleModelUserSelf},
				{DataPerm: store.RoleModelDataPermUserDeptAndSubDept},
			},
			want: DataScopeDeptAndSub,
		},
		{
			name: "all wins",
			roles: []store.RoleModel{
				{DataPerm: store.RoleModelUserSelf},
				{DataPerm: store.RoleModelDataPermAll},
			},
			want: DataScopeAll,
		},
		{
			name: "invalid ignored conservatively",
			roles: []store.RoleModel{
				{DataPerm: 99},
			},
			want: DataScopeSelf,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := widestDataScope(tt.roles); got != tt.want {
				t.Fatalf("widestDataScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataScopeAllowsUser(t *testing.T) {
	scope := DataScope{
		UserID:  10,
		DeptID:  20,
		Level:   DataScopeDeptAndSub,
		DeptIDs: []uint64{20, 21},
	}

	if !scope.AllowsUser(&store.UserModel{Model: xorm.Model{ID: 11}, DeptID: 21}) {
		t.Fatal("expected user in child dept to be allowed")
	}
	if scope.AllowsUser(&store.UserModel{Model: xorm.Model{ID: 12}, DeptID: 30}) {
		t.Fatal("expected user outside dept scope to be denied")
	}

	selfScope := DataScope{UserID: 10, DeptID: 20, Level: DataScopeSelf, DeptIDs: []uint64{20}}
	if !selfScope.AllowsUser(&store.UserModel{Model: xorm.Model{ID: 10}, DeptID: 30}) {
		t.Fatal("expected self user to be allowed regardless dept")
	}
	if selfScope.AllowsUser(&store.UserModel{Model: xorm.Model{ID: 11}, DeptID: 20}) {
		t.Fatal("expected other user in same dept to be denied for self scope")
	}
}

func TestDataScopeAllowsRole(t *testing.T) {
	scope := DataScope{
		UserID:  10,
		DeptID:  20,
		Level:   DataScopeDeptSelf,
		DeptIDs: []uint64{20},
	}

	tests := []struct {
		name string
		role *store.RoleModel
		want bool
	}{
		{
			name: "owned by user",
			role: &store.RoleModel{
				BuiltIn:     store.RoleModelNonBuiltIn,
				DataPerm:    store.RoleModelDataPermUserDeptSelf,
				OwnerUserID: 10,
			},
			want: true,
		},
		{
			name: "owned by visible dept",
			role: &store.RoleModel{
				BuiltIn:     store.RoleModelNonBuiltIn,
				DataPerm:    store.RoleModelDataPermUserDeptSelf,
				OwnerDeptID: 20,
			},
			want: true,
		},
		{
			name: "wider data permission denied",
			role: &store.RoleModel{
				BuiltIn:     store.RoleModelNonBuiltIn,
				DataPerm:    store.RoleModelDataPermAll,
				OwnerUserID: 10,
			},
			want: false,
		},
		{
			name: "builtin denied",
			role: &store.RoleModel{
				BuiltIn:  store.RoleModelBuiltIn,
				DataPerm: store.RoleModelDataPermUserDeptSelf,
			},
			want: false,
		},
		{
			name: "global role denied",
			role: &store.RoleModel{
				BuiltIn:  store.RoleModelNonBuiltIn,
				DataPerm: store.RoleModelDataPermUserDeptSelf,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scope.AllowsRole(tt.role); got != tt.want {
				t.Fatalf("AllowsRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataScopeAllowsRoleMutable(t *testing.T) {
	scope := DataScope{
		UserID:  10,
		DeptID:  20,
		Level:   DataScopeDeptSelf,
		DeptIDs: []uint64{20},
	}

	if scope.AllowsRoleMutable(&store.RoleModel{
		BuiltIn:  store.RoleModelNonBuiltIn,
		DataPerm: store.RoleModelDataPermUserDeptSelf,
	}) {
		t.Fatal("expected global role to be visible but immutable")
	}
	if !scope.AllowsRoleMutable(&store.RoleModel{
		BuiltIn:     store.RoleModelNonBuiltIn,
		DataPerm:    store.RoleModelDataPermUserDeptSelf,
		OwnerDeptID: 20,
	}) {
		t.Fatal("expected role owned by visible dept to be mutable")
	}
}

func TestDataScopeAllowsDeptMutableID(t *testing.T) {
	scope := DataScope{UserID: 10, DeptID: 20, Level: DataScopeSelf, DeptIDs: []uint64{20}}
	if scope.AllowsDeptMutableID(20) {
		t.Fatal("expected self scope to deny dept mutation")
	}
	scope.Level = DataScopeDeptSelf
	if !scope.AllowsDeptMutableID(20) {
		t.Fatal("expected dept-self scope to allow own dept mutation")
	}
}

func TestDataScopeAdminBypass(t *testing.T) {
	scope := DataScope{IsAdmin: true, Level: DataScopeAll}

	if !scope.AllowsUser(&store.UserModel{Model: xorm.Model{ID: 99}, DeptID: 999}) {
		t.Fatal("expected admin to bypass user scope")
	}
	if !scope.AllowsDeptID(0) {
		t.Fatal("expected admin to bypass dept scope")
	}
	if !scope.AllowsRole(&store.RoleModel{BuiltIn: store.RoleModelBuiltIn, DataPerm: store.RoleModelDataPermAll}) {
		t.Fatal("expected admin to bypass role scope")
	}
}

func TestDataScopeLogScopeUsesNumericColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("open dry-run sqlite: %v", err)
	}

	deptScope := DataScope{
		UserID:  10,
		DeptID:  20,
		Level:   DataScopeDeptAndSub,
		DeptIDs: []uint64{20, 21},
	}
	stmt := db.Model(&store.LogModel{}).Scopes(deptScope.LogScope()).Find(&[]store.LogModel{}).Statement
	if !strings.Contains(stmt.SQL.String(), "dept_id_u64") {
		t.Fatalf("LogScope SQL = %q, want dept_id_u64 filter", stmt.SQL.String())
	}

	selfScope := DataScope{UserID: 10, DeptID: 20, Level: DataScopeSelf, DeptIDs: []uint64{20}}
	stmt = db.Model(&store.LogModel{}).Scopes(selfScope.LogScope()).Find(&[]store.LogModel{}).Statement
	if !strings.Contains(stmt.SQL.String(), "user_id_u64") {
		t.Fatalf("LogScope SQL = %q, want user_id_u64 filter", stmt.SQL.String())
	}
}

func TestDataScopeRoleScopeUsesOwnerFilters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("open dry-run sqlite: %v", err)
	}

	scope := DataScope{
		UserID:  10,
		DeptID:  20,
		Level:   DataScopeDeptSelf,
		DeptIDs: []uint64{20, 21},
	}
	stmt := db.Model(&store.RoleModel{}).Scopes(scope.RoleScope()).Find(&[]store.RoleModel{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "owner_user_id") {
		t.Fatalf("RoleScope SQL = %q, want owner_user_id filter", sql)
	}
	if !strings.Contains(sql, "owner_dept_id") {
		t.Fatalf("RoleScope SQL = %q, want owner_dept_id filter", sql)
	}
}

func TestLoadDataScopeUsesSavedUserDeptID(t *testing.T) {
	userStore := &dataScopeUserStore{
		user: &store.UserModel{
			Model:  xorm.Model{ID: 1},
			DeptID: 7,
			Roles: []store.RoleModel{
				{DataPerm: store.RoleModelDataPermUserDeptSelf},
			},
		},
	}
	scope, err := loadDataScope(context.Background(), &authsession.AuthContext{UserID: 1, DeptID: 99}, userStore, &dataScopeDeptStore{})
	if err != nil {
		t.Fatalf("loadDataScope() error = %v", err)
	}
	if scope.DeptID != 7 {
		t.Fatalf("loadDataScope() DeptID = %d, want 7", scope.DeptID)
	}
	if len(scope.DeptIDs) != 1 || scope.DeptIDs[0] != 7 {
		t.Fatalf("loadDataScope() DeptIDs = %v, want [7]", scope.DeptIDs)
	}
}

type dataScopeUserStore struct {
	user *store.UserModel
}

func (s *dataScopeUserStore) Add(context.Context, *store.UserModel) error                { return nil }
func (s *dataScopeUserStore) BatchAdd(context.Context, []*store.UserModel) error         { return nil }
func (s *dataScopeUserStore) Delete(context.Context, []uint64) error                     { return nil }
func (s *dataScopeUserStore) Update(context.Context, uint64, *store.UserModel) error     { return nil }
func (s *dataScopeUserStore) UpdateBase(context.Context, uint64, *store.UserModel) error { return nil }
func (s *dataScopeUserStore) UpdateBaseWithoutHook(context.Context, uint64, *store.UserModel) error {
	return nil
}
func (s *dataScopeUserStore) UpdateBaseWithoutHookAndTx(context.Context, uint64, *store.UserModel) error {
	return nil
}
func (s *dataScopeUserStore) UpdatePass(context.Context, uint64, string) error { return nil }
func (s *dataScopeUserStore) Get(context.Context, uint64) (*store.UserModel, error) {
	return s.user, nil
}
func (s *dataScopeUserStore) GetAuthSnapshot(context.Context, uint64) (*store.UserAuthSnapshot, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetByUsername(context.Context, string) (*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetByPhone(context.Context, string) (*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetList(context.Context, store.UserModelGetListOption, ...func(*gorm.DB) *gorm.DB) ([]*store.UserModel, int64, error) {
	return nil, 0, nil
}
func (s *dataScopeUserStore) GetByIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetByDeptIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) CountByDeptIds(context.Context, []uint64) (int64, error) { return 0, nil }
func (s *dataScopeUserStore) GetByUsernames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetByNames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetByPhones(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s *dataScopeUserStore) GetHeartbeatExpiredUids(context.Context, int64) ([]uint64, error) {
	return nil, nil
}
func (s *dataScopeUserStore) BatchOffline(context.Context, []uint64) error { return nil }
func (s *dataScopeUserStore) CountOnline(context.Context) (int64, error)   { return 0, nil }
func (s *dataScopeUserStore) CountByOption(context.Context, func(*gorm.DB) *gorm.DB) (int64, error) {
	return 0, nil
}
func (s *dataScopeUserStore) CountByRole(context.Context, uint64) (int64, error) { return 0, nil }
func (s *dataScopeUserStore) GetByRoleID(context.Context, uint64) ([]*store.UserModel, error) {
	return nil, nil
}

type dataScopeDeptStore struct{}

func (dataScopeDeptStore) Add(context.Context, *store.DeptModel) error  { return nil }
func (dataScopeDeptStore) Delete(context.Context, uint64) error         { return nil }
func (dataScopeDeptStore) DeleteByIds(context.Context, []uint64) error  { return nil }
func (dataScopeDeptStore) Update(context.Context, uint64, string) error { return nil }
func (dataScopeDeptStore) UpdatePriority(context.Context, []store.DeptModel) error {
	return nil
}
func (dataScopeDeptStore) GetSelf(context.Context, uint64) (*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) Get(context.Context, uint64) (*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetByName(context.Context, string) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetByID(context.Context, uint64) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetByIDs(context.Context, []uint64) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetByCode(context.Context, string) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetSubtreeIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetAncestorIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetAll(context.Context) ([]*store.DeptModel, error) { return nil, nil }
func (dataScopeDeptStore) GetTopAll(context.Context) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) GetChilds(context.Context, uint64) ([]*store.DeptModel, error) {
	return nil, nil
}
func (dataScopeDeptStore) CountByOption(context.Context, ...func(*gorm.DB) *gorm.DB) (int64, error) {
	return 0, nil
}
