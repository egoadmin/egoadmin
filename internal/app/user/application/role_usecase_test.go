package application

import (
	"context"
	"errors"
	"testing"

	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
)

func TestRoleUseCase_CreateRole(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{nextID: 100}
	tx := &transactionRunner{}
	permissions := &rolePermissionBinding{}
	locks := &roleLocks{}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          tx,
		RoleLocks:      locks,
		Permissions:    permissions,
	})

	result, err := uc.CreateRole(context.Background(), SaveRoleCommand{
		Name: "manager",
		Type: roledomain.TypePlatform,
		Policies: []roledomain.PermissionPolicy{
			{Service: " user.v1.User ", Method: " addRole "},
			{Service: "USER.V1.USER", Method: "ADDROLE"},
			{Service: "", Method: "ignored"},
		},
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if result.ID != 100 {
		t.Fatalf("CreateRole() ID = %d, want 100", result.ID)
	}
	if !tx.called {
		t.Fatal("CreateRole() did not run in transaction")
	}
	if locks.roleCreateCalls != 1 {
		t.Fatalf("role create locks = %d, want 1", locks.roleCreateCalls)
	}
	created := repo.roles[result.ID]
	if created == nil {
		t.Fatal("CreateRole() did not persist role")
	}
	if created.BuiltIn != roledomain.NonBuiltIn {
		t.Fatalf("BuiltIn = %d, want NonBuiltIn", created.BuiltIn)
	}
	wantPolicies := []roledomain.PermissionPolicy{{Service: "USER.V1.USER", Method: "ADDROLE"}}
	if !equalPolicies(created.Policies, wantPolicies) {
		t.Fatalf("created policies = %+v, want %+v", created.Policies, wantPolicies)
	}
	if permissions.replacedRoleID != result.ID || !equalPolicies(permissions.replacedPolicies, wantPolicies) {
		t.Fatalf("permission sync = id:%d policies:%+v, want id:%d policies:%+v",
			permissions.replacedRoleID, permissions.replacedPolicies, result.ID, wantPolicies)
	}
}

func TestRoleUseCase_CreateRoleRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{
		roles: map[uint64]*roledomain.Role{
			1: {ID: 1, Name: "manager"},
		},
	}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          &transactionRunner{},
	})

	_, err := uc.CreateRole(context.Background(), SaveRoleCommand{Name: "manager"})
	if !errors.Is(err, roledomain.ErrNameExists) {
		t.Fatalf("CreateRole() error = %v, want ErrNameExists", err)
	}
}

func TestRoleUseCase_UpdateRole(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{
		roles: map[uint64]*roledomain.Role{
			7: {ID: 7, Name: "manager", Policies: []roledomain.PermissionPolicy{{Service: "OLD", Method: "READ"}}},
		},
	}
	tx := &transactionRunner{}
	permissions := &rolePermissionBinding{}
	locks := &roleLocks{}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          tx,
		RoleLocks:      locks,
		Permissions:    permissions,
	})

	err := uc.UpdateRole(context.Background(), 7, SaveRoleCommand{
		Name:     "manager",
		Type:     roledomain.TypePlatform,
		DataPerm: 2,
		Policies: []roledomain.PermissionPolicy{
			{Service: "user.v1.User", Method: "updateRole"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if !tx.called {
		t.Fatal("UpdateRole() did not run in transaction")
	}
	if locks.roleUpdateCalls != 1 {
		t.Fatalf("role update locks = %d, want 1", locks.roleUpdateCalls)
	}
	updated := repo.roles[7]
	wantPolicies := []roledomain.PermissionPolicy{{Service: "USER.V1.USER", Method: "UPDATEROLE"}}
	if updated.DataPerm != 2 || !equalPolicies(updated.Policies, wantPolicies) {
		t.Fatalf("updated role = %+v, want DataPerm 2 policies %+v", updated, wantPolicies)
	}
	if permissions.replacedRoleID != 7 || !equalPolicies(permissions.replacedPolicies, wantPolicies) {
		t.Fatalf("permission sync = id:%d policies:%+v, want id:7 policies:%+v",
			permissions.replacedRoleID, permissions.replacedPolicies, wantPolicies)
	}
}

func TestRoleUseCase_UpdateRoleRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{
		roles: map[uint64]*roledomain.Role{
			7: {ID: 7, Name: "manager"},
			8: {ID: 8, Name: "auditor"},
		},
	}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          &transactionRunner{},
	})

	err := uc.UpdateRole(context.Background(), 7, SaveRoleCommand{Name: "auditor"})
	if !errors.Is(err, roledomain.ErrNameExists) {
		t.Fatalf("UpdateRole() error = %v, want ErrNameExists", err)
	}
}

func TestRoleUseCase_DeleteRoleRejectsAssignedRole(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{
		roles: map[uint64]*roledomain.Role{
			7: {ID: 7, Name: "manager"},
		},
	}
	permissions := &rolePermissionBinding{}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          &transactionRunner{},
		Assignments:    &roleAssignments{count: 3},
		Permissions:    permissions,
	})

	err := uc.DeleteRole(context.Background(), 7)
	if !errors.Is(err, roledomain.ErrInUse) {
		t.Fatalf("DeleteRole() error = %v, want ErrInUse", err)
	}
	var inUse roledomain.InUseError
	if !errors.As(err, &inUse) || inUse.Count != 3 {
		t.Fatalf("DeleteRole() in-use error = %#v, want count 3", err)
	}
	if _, ok := repo.roles[7]; !ok {
		t.Fatal("DeleteRole() deleted assigned role")
	}
	if permissions.deletedRoleID != 0 {
		t.Fatalf("deleted role permissions for %d, want none", permissions.deletedRoleID)
	}
}

func TestRoleUseCase_DeleteRole(t *testing.T) {
	t.Parallel()

	repo := &roleRepo{
		roles: map[uint64]*roledomain.Role{
			7: {ID: 7, Name: "manager"},
		},
	}
	tx := &transactionRunner{}
	permissions := &rolePermissionBinding{}
	locks := &roleLocks{}
	uc := NewRoleUseCase(RoleOptions{
		RoleRepository: repo,
		Mysql:          tx,
		RoleLocks:      locks,
		Assignments:    &roleAssignments{},
		Permissions:    permissions,
	})

	if err := uc.DeleteRole(context.Background(), 7); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
	if !tx.called {
		t.Fatal("DeleteRole() did not run in transaction")
	}
	if locks.userCreateCalls != 1 {
		t.Fatalf("user create locks = %d, want 1", locks.userCreateCalls)
	}
	if _, ok := repo.roles[7]; ok {
		t.Fatal("DeleteRole() did not delete role")
	}
	if permissions.deletedRoleID != 7 {
		t.Fatalf("deleted role permissions for %d, want 7", permissions.deletedRoleID)
	}
}

func TestRoleUseCase_CheckDeleteRole(t *testing.T) {
	t.Parallel()

	uc := NewRoleUseCase(RoleOptions{
		Assignments: &roleAssignments{count: 2},
		RoleLocks:   &roleLocks{},
	})

	msg, err := uc.CheckDeleteRole(context.Background(), 7)
	if err != nil {
		t.Fatalf("CheckDeleteRole() error = %v", err)
	}
	want := "有2个具备该角色的账号,请先编辑账号所属角色后再进行删除"
	if msg != want {
		t.Fatalf("CheckDeleteRole() msg = %q, want %q", msg, want)
	}
}

type roleRepo struct {
	roles  map[uint64]*roledomain.Role
	nextID uint64
}

func (r *roleRepo) Create(_ context.Context, role *roledomain.Role) error {
	if r.nextID == 0 {
		r.nextID = 1
	}
	role.ID = r.nextID
	r.nextID++
	if r.roles == nil {
		r.roles = map[uint64]*roledomain.Role{}
	}
	r.roles[role.ID] = cloneRole(role)
	return nil
}

func (r *roleRepo) Update(_ context.Context, id uint64, role *roledomain.Role) error {
	if _, ok := r.roles[id]; !ok {
		return roledomain.ErrNotFound
	}
	updated := cloneRole(role)
	updated.ID = id
	r.roles[id] = updated
	return nil
}

func (r *roleRepo) Delete(_ context.Context, id uint64) error {
	if _, ok := r.roles[id]; !ok {
		return roledomain.ErrNotFound
	}
	delete(r.roles, id)
	return nil
}

func (r *roleRepo) FindByID(_ context.Context, id uint64) (*roledomain.Role, error) {
	role, ok := r.roles[id]
	if !ok {
		return nil, roledomain.ErrNotFound
	}
	return cloneRole(role), nil
}

func (r *roleRepo) FindByName(_ context.Context, name string) (*roledomain.Role, error) {
	for _, role := range r.roles {
		if role.Name == name {
			return cloneRole(role), nil
		}
	}
	return nil, roledomain.ErrNotFound
}

func cloneRole(role *roledomain.Role) *roledomain.Role {
	if role == nil {
		return nil
	}
	cp := *role
	cp.Policies = append([]roledomain.PermissionPolicy(nil), role.Policies...)
	return &cp
}

type roleAssignments struct {
	count int64
}

func (a *roleAssignments) CountByRole(context.Context, uint64) (int64, error) {
	return a.count, nil
}

type rolePermissionBinding struct {
	replacedRoleID   uint64
	replacedPolicies []roledomain.PermissionPolicy
	deletedRoleID    uint64
}

func (b *rolePermissionBinding) ReplaceRolePermissions(_ context.Context, roleID uint64, policies []roledomain.PermissionPolicy) error {
	b.replacedRoleID = roleID
	b.replacedPolicies = append([]roledomain.PermissionPolicy(nil), policies...)
	return nil
}

func (b *rolePermissionBinding) DeleteRole(_ context.Context, roleID uint64) error {
	b.deletedRoleID = roleID
	return nil
}

type roleLocks struct {
	roleCreateCalls int
	roleUpdateCalls int
	userCreateCalls int
}

func (l *roleLocks) WithRoleCreateLock(ctx context.Context, fn func(context.Context) error) error {
	l.roleCreateCalls++
	return fn(ctx)
}

func (l *roleLocks) WithRoleUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	l.roleUpdateCalls++
	return fn(ctx)
}

func (l *roleLocks) WithUserCreateLock(ctx context.Context, fn func(context.Context) error) error {
	l.userCreateCalls++
	return fn(ctx)
}

func equalPolicies(a, b []roledomain.PermissionPolicy) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
