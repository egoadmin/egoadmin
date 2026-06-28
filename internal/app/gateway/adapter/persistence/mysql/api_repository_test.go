package mysql

import (
	"context"
	"net/url"
	"testing"

	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func TestAPIRepository_DeleteByIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := newTestDB(t)
	repo := NewAPIRepository(testMysql{db: db})

	if err := db.Exec(`CREATE TABLE api (
		id integer PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		signcode text,
		name text,
		path text,
		method text
	)`).Error; err != nil {
		t.Fatalf("create api table: %v", err)
	}
	if err := db.Exec(`INSERT INTO api (id, signcode, name, path, method) VALUES
		(1, 'api-1', 'API1', 'USER.V1.USERSERVICE', 'GETUSER'),
		(2, 'api-2', 'API2', 'USER.V1.USERSERVICE', 'ADDUSER')`).Error; err != nil {
		t.Fatalf("create apis: %v", err)
	}

	if err := repo.DeleteByIDs(ctx, []uint64{1}); err != nil {
		t.Fatalf("delete apis: %v", err)
	}

	var remaining []apiModel
	if err := db.Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining apis: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != 2 {
		t.Fatalf("remaining apis = %#v", remaining)
	}

	if err := repo.DeleteByIDs(ctx, nil); err != nil {
		t.Fatalf("delete empty apis: %v", err)
	}
}
