package mysql

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T, models ...any) *gorm.DB {
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

	if len(models) > 0 {
		if err = db.AutoMigrate(models...); err != nil {
			t.Fatalf("migrate models: %v", err)
		}
	}

	return db
}

type testConfigModel struct {
	Ckey  string `gorm:"primaryKey"`
	Value string
}

func (testConfigModel) TableName() string {
	return "test_config"
}

type testConfigRepo struct {
	cc *gorm.DB
}

func (r *testConfigRepo) Add(ctx context.Context, conf *testConfigModel) error {
	return dbWithContext(ctx, r.cc).Create(conf).Error
}

func (r *testConfigRepo) Get(ctx context.Context, key string) (*testConfigModel, error) {
	var conf testConfigModel
	err := dbWithContext(ctx, r.cc).Where("ckey = ?", key).First(&conf).Error
	return &conf, err
}

func newTestMysql(t *testing.T) (*Mysql, *testConfigRepo) {
	db := newTestDB(t, &testConfigModel{})
	return &Mysql{cc: db}, &testConfigRepo{cc: db}
}

func TestRepositoryWorksWithoutExplicitTransaction(t *testing.T) {
	ctx := context.Background()
	_, configs := newTestMysql(t)

	if err := configs.Add(ctx, &testConfigModel{Ckey: "plain", Value: "1"}); err != nil {
		t.Fatalf("add config: %v", err)
	}

	got, err := configs.Get(ctx, "plain")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if got.Value != "1" {
		t.Fatalf("unexpected value: got %q want %q", got.Value, "1")
	}
}

func TestRepositoryJoinsOuterTransactionRollback(t *testing.T) {
	ctx := context.Background()
	mysql, configs := newTestMysql(t)
	rollbackErr := errors.New("rollback")

	err := mysql.Transaction(ctx, func(txCtx context.Context) error {
		if err := configs.Add(txCtx, &testConfigModel{Ckey: "rollback", Value: "1"}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	if _, err = configs.Get(ctx, "rollback"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record to be rolled back, got %v", err)
	}
}

func TestNestedTransactionReusesExistingContext(t *testing.T) {
	ctx := context.Background()
	mysql, configs := newTestMysql(t)
	rollbackErr := errors.New("rollback")

	err := mysql.Transaction(ctx, func(txCtx context.Context) error {
		return mysql.Transaction(txCtx, func(nestedCtx context.Context) error {
			if err := configs.Add(nestedCtx, &testConfigModel{Ckey: "nested", Value: "1"}); err != nil {
				return err
			}
			return rollbackErr
		})
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("unexpected transaction error: %v", err)
	}

	if _, err = configs.Get(ctx, "nested"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected nested write to be rolled back, got %v", err)
	}
}
