package mysql

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testMysql struct {
	db *gorm.DB
}

func (m testMysql) Migrate(context.Context, []any, []platformmysql.MigrationJoinTable) error {
	return nil
}

func (m testMysql) Transaction(ctx context.Context, callback func(context.Context) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return callback(context.WithValue(ctx, txKey{}, tx))
	})
}

func (m testMysql) WithTx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return m.db.WithContext(ctx)
}

type txKey struct{}

func TestMachineLeaseRepository_AllocateUniqueAndExhausted(t *testing.T) {
	t.Parallel()

	repo := newTestMachineLeaseRepository(t)
	ctx := context.Background()
	req := testMachineRequest("test", "instance-a", 1)

	first, err := repo.Allocate(ctx, req)
	if err != nil {
		t.Fatalf("allocate first: %v", err)
	}
	second, err := repo.Allocate(ctx, testMachineRequest("test", "instance-b", 1))
	if err != nil {
		t.Fatalf("allocate second: %v", err)
	}
	if first.MachineID == second.MachineID {
		t.Fatalf("machine ids should be unique: first=%d second=%d", first.MachineID, second.MachineID)
	}
	_, err = repo.Allocate(ctx, testMachineRequest("test", "instance-c", 1))
	if !errors.Is(err, idgen.ErrMachineIDOverflow) {
		t.Fatalf("allocate exhausted err = %v, want ErrMachineIDOverflow", err)
	}
}

func TestMachineLeaseRepository_ReusesUnexpiredInstanceLease(t *testing.T) {
	t.Parallel()

	repo := newTestMachineLeaseRepository(t)
	ctx := context.Background()
	req := testMachineRequest("test", "instance-a", 1)

	first, err := repo.Allocate(ctx, req)
	if err != nil {
		t.Fatalf("allocate first: %v", err)
	}
	second, err := repo.Allocate(ctx, req)
	if err != nil {
		t.Fatalf("allocate second: %v", err)
	}
	if first.MachineID != second.MachineID {
		t.Fatalf("machine id = %d, want reused %d", second.MachineID, first.MachineID)
	}
	if first.SessionID == second.SessionID {
		t.Fatalf("session id should rotate when reusing instance lease")
	}
}

func TestMachineLeaseRepository_RecyclesExpiredLease(t *testing.T) {
	t.Parallel()

	repo := newTestMachineLeaseRepository(t)
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	repo.now = func() time.Time { return base }
	ctx := context.Background()

	first, err := repo.Allocate(ctx, testMachineRequest("test", "instance-a", 0))
	if err != nil {
		t.Fatalf("allocate first: %v", err)
	}
	repo.now = func() time.Time { return base.Add(time.Minute) }
	second, err := repo.Allocate(ctx, testMachineRequest("test", "instance-b", 0))
	if err != nil {
		t.Fatalf("allocate recycled: %v", err)
	}
	if second.MachineID != first.MachineID {
		t.Fatalf("machine id = %d, want recycled %d", second.MachineID, first.MachineID)
	}
	if second.InstanceID != "instance-b" {
		t.Fatalf("instance id = %q, want instance-b", second.InstanceID)
	}
}

func TestMachineLeaseRepository_RenewAndReleaseValidateSession(t *testing.T) {
	t.Parallel()

	repo := newTestMachineLeaseRepository(t)
	ctx := context.Background()
	lease, err := repo.Allocate(ctx, testMachineRequest("test", "instance-a", 0))
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if err = repo.Renew(ctx, lease); err != nil {
		t.Fatalf("renew: %v", err)
	}
	bad := lease
	bad.SessionID = "wrong-session"
	if err = repo.Renew(ctx, bad); !errors.Is(err, idgen.ErrMachineLeaseLost) {
		t.Fatalf("renew bad session err = %v, want ErrMachineLeaseLost", err)
	}
	if err = repo.Release(ctx, bad); !errors.Is(err, idgen.ErrMachineLeaseLost) {
		t.Fatalf("release bad session err = %v, want ErrMachineLeaseLost", err)
	}
	if err = repo.Release(ctx, lease); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func TestMachineLeaseRepository_CleanupExpiredOnlyDeletesBeforeRetention(t *testing.T) {
	t.Parallel()

	repo := newTestMachineLeaseRepository(t)
	base := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	rows := []machineLeaseModel{
		{
			Namespace:     "test",
			MachineID:     0,
			InstanceID:    "old-instance",
			SessionID:     "old-session",
			TTLMillis:     30000,
			RenewMillis:   10000,
			ExpiresAt:     base.Add(-8 * 24 * time.Hour),
			LastRenewedAt: base.Add(-8 * 24 * time.Hour),
			CreatedAt:     base.Add(-8 * 24 * time.Hour),
			UpdatedAt:     base.Add(-8 * 24 * time.Hour),
		},
		{
			Namespace:     "test",
			MachineID:     1,
			InstanceID:    "recent-instance",
			SessionID:     "recent-session",
			TTLMillis:     30000,
			RenewMillis:   10000,
			ExpiresAt:     base.Add(-time.Hour),
			LastRenewedAt: base.Add(-time.Hour),
			CreatedAt:     base.Add(-time.Hour),
			UpdatedAt:     base.Add(-time.Hour),
		},
	}
	if err := repo.db.WithTx(ctx).Create(&rows).Error; err != nil {
		t.Fatalf("seed leases: %v", err)
	}

	deleted, err := repo.CleanupExpired(ctx, base.Add(-7*24*time.Hour), 100)
	if err != nil {
		t.Fatalf("cleanup expired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	rows = nil
	if err = repo.db.WithTx(ctx).Order("machine_id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("list leases: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].MachineID != 1 {
		t.Fatalf("remaining machine id = %d, want recent 1", rows[0].MachineID)
	}
}

func newTestMachineLeaseRepository(t *testing.T) *MachineLeaseRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+url.QueryEscape(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&machineLeaseModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewMachineLeaseRepository(testMysql{db: db})
}

func testMachineRequest(namespace string, instanceID string, maxMachineID int) idgen.MachineRequest {
	return idgen.MachineRequest{
		Namespace:     namespace,
		InstanceID:    instanceID,
		MaxMachineID:  maxMachineID,
		TTL:           30 * time.Second,
		RenewInterval: 10 * time.Second,
	}
}
