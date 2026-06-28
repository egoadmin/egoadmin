package mysql

import (
	"context"
	"net/url"
	"testing"
	"time"

	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testIDGetter struct{}

func (testIDGetter) Get() (uint64, error) { return 1, nil }

type testMysql struct {
	db *gorm.DB
}

func (m testMysql) Migrate(context.Context, []any, []platformmysql.MigrationJoinTable) error {
	return nil
}

func (m testMysql) Transaction(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

func (m testMysql) WithTx(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

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

func setupUserRepositoryTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`CREATE TABLE user (
			id integer PRIMARY KEY,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			username text,
			password text,
			name text,
			phone text,
			gender integer,
			user_status integer,
			user_type integer,
			user_online integer,
			remark text,
			dept_id integer,
			heartbeat_time datetime
		)`,
		`CREATE TABLE role (
			id integer PRIMARY KEY,
			owner_user_id integer,
			owner_dept_id integer,
			uses text NOT NULL
		)`,
		`CREATE TABLE user_role (
			user_model_id integer,
			role_model_id integer,
			PRIMARY KEY (user_model_id, role_model_id)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create user repository test schema: %v", err)
		}
	}
}

func TestUserRepository_UpdateReplacesRoleJoinRowsWithoutRoleUpsert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := newTestDB(t)
	setupUserRepositoryTestSchema(t, db)
	repo := NewUserRepository(testMysql{db: db}, testIDGetter{})

	if err := db.Exec(`INSERT INTO user
		(id, username, password, name, phone, user_status, user_type, user_online, heartbeat_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1,
		"alice",
		"hash",
		"Alice",
		"13900000000",
		int32(userdomain.StatusValid),
		userdomain.TypePlatform,
		int32(userdomain.OnlineStatusOffline),
		testNow(),
	).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_role (user_model_id, role_model_id) VALUES (1, 10)`).Error; err != nil {
		t.Fatalf("create old user role: %v", err)
	}

	if err := repo.Update(ctx, 1, &userdomain.User{
		Name:    "Alice Updated",
		Phone:   "13900000001",
		Gender:  userdomain.GenderFemale,
		Status:  userdomain.StatusValid,
		DeptID:  2,
		RoleIDs: []uint64{41, 41, 42},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	var joins []userRoleModel
	if err := db.Order("role_model_id ASC").Find(&joins).Error; err != nil {
		t.Fatalf("list user roles: %v", err)
	}
	if len(joins) != 2 || joins[0].RoleModelID != 41 || joins[1].RoleModelID != 42 {
		t.Fatalf("joins = %+v, want role ids [41 42]", joins)
	}

	var roleCount int64
	if err := db.Table("role").Count(&roleCount).Error; err != nil {
		t.Fatalf("count roles: %v", err)
	}
	if roleCount != 0 {
		t.Fatalf("role rows = %d, want 0", roleCount)
	}
}

func testNow() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}
