package store

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeIDGetter struct{}

func (fakeIDGetter) Get() (uint64, error) { return 1, nil }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	mysql.NewID(fakeIDGetter{})
	db, err := gorm.Open(sqlite.Open("file:"+url.QueryEscape(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

func setupAssociationTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`CREATE TABLE role (
			id integer PRIMARY KEY,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			name text,
			typ integer,
			built_in integer,
			data_perm integer,
			owner_user_id integer,
			owner_dept_id integer,
			uses text,
			view_menus text,
			desc text
		)`,
		`CREATE TABLE user (
			id integer PRIMARY KEY,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			built_in integer,
			username text,
			password text,
			salt text,
			name text,
			avatar text,
			phone text,
			email text,
			realname text,
			nickname text,
			gender integer,
			birthday datetime,
			first_login integer,
			last_login_ip text,
			last_login_at datetime,
			user_status integer,
			user_type integer,
			user_online integer,
			heartbeat_time datetime,
			remark text,
			dept_id integer
		)`,
		`CREATE TABLE user_role (
			user_model_id integer,
			role_model_id integer,
			PRIMARY KEY (user_model_id, role_model_id)
		)`,
		`CREATE TABLE role_permission_policy (
			role_model_id integer,
			service text,
			method text,
			PRIMARY KEY (role_model_id, service, method)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create association test schema: %v", err)
		}
	}
}

func TestUserRoleAssociationUpdateJoinsOuterTransaction(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	setupAssociationTestSchema(t, db)
	txer := mysql.NewMysql(db, mysql.NewID(fakeIDGetter{}))
	users := &User{cc: db}
	rollbackErr := errors.New("rollback")

	if err := db.Create(&RoleModel{
		Model: xorm.Model{ID: 1},
		Name:  "role-1",
		Typ:   RoleModelTypePlatform,
	}).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&UserModel{
		Model:    xorm.Model{ID: 1},
		Username: "user-1",
		Password: "pass",
		UserType: UserModelTypePlatform,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	err := txer.Transaction(ctx, func(txCtx context.Context) error {
		if err := users.Update(txCtx, 1, &UserModel{
			Name:  "updated",
			Roles: []RoleModel{{Model: xorm.Model{ID: 1}}},
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	var rels int64
	if err = db.Model(&UserRole{}).Count(&rels).Error; err != nil {
		t.Fatalf("count user_role: %v", err)
	}
	if rels != 0 {
		t.Fatalf("expected user_role association rollback, got %d rows", rels)
	}

	var user UserModel
	if err = db.First(&user, 1).Error; err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Name == "updated" {
		t.Fatalf("expected user fields to be rolled back")
	}
}

func TestRolePermissionPolicyUpdateJoinsOuterTransaction(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	setupAssociationTestSchema(t, db)
	txer := mysql.NewMysql(db, mysql.NewID(fakeIDGetter{}))
	roles := &Role{cc: db}
	rollbackErr := errors.New("rollback")

	if err := db.Create(&RoleModel{
		Model: xorm.Model{ID: 1},
		Name:  "role-1",
		Typ:   RoleModelTypePlatform,
	}).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	err := txer.Transaction(ctx, func(txCtx context.Context) error {
		if err := roles.Update(txCtx, 1, &RoleModel{
			Name: "updated",
			Typ:  RoleModelTypePlatform,
			Policies: []RolePermissionPolicy{
				{Service: "USER.V1.USERSERVICE", Method: "GETUSER"},
			},
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	var rels int64
	if err = db.Model(&RolePermissionPolicy{}).Count(&rels).Error; err != nil {
		t.Fatalf("count role_permission_policy: %v", err)
	}
	if rels != 0 {
		t.Fatalf("expected role permission policies rollback, got %d rows", rels)
	}

	var role RoleModel
	if err = db.First(&role, 1).Error; err != nil {
		t.Fatalf("get role: %v", err)
	}
	if role.Name == "updated" {
		t.Fatalf("expected role fields to be rolled back")
	}
}
