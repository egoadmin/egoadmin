package gormstore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestStore_FetchAllocatesNonOverlappingRanges(t *testing.T) {
	db := newTestDB(t)
	store := New(db)
	ctx := context.Background()

	first, _, err := store.Fetch(ctx, "test", "order", 10)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	second, _, err := store.Fetch(ctx, "test", "order", 10)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if first.Start != 1 || first.End != 11 {
		t.Fatalf("first = [%d,%d), want [1,11)", first.Start, first.End)
	}
	if second.Start != 11 || second.End != 21 {
		t.Fatalf("second = [%d,%d), want [11,21)", second.Start, second.End)
	}
}

func TestStore_FetchMissingName(t *testing.T) {
	db := newTestDB(t)
	store := New(db)
	_, _, err := store.Fetch(context.Background(), "test", "missing", 10)
	if !errors.Is(err, idgen.ErrNameNotFound) {
		t.Fatalf("err = %v, want ErrNameNotFound", err)
	}
}

func TestStore_EnsureCreatesMissingSegment(t *testing.T) {
	db := newTestDB(t)
	store := New(db)

	if err := store.Ensure(context.Background(), "test", "invoice", idgen.EnsureSegmentConfig{
		NextID:  100,
		Step:    50,
		MinStep: 10,
		MaxStep: 1000,
		Status:  idgen.SegmentStatusEnabled,
	}); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	r, cfg, err := store.Fetch(context.Background(), "test", "invoice", 50)
	if err != nil {
		t.Fatalf("fetch ensured segment: %v", err)
	}
	if r.Start != 100 || r.End != 150 {
		t.Fatalf("range = [%d,%d), want [100,150)", r.Start, r.End)
	}
	if cfg.Step != 50 || cfg.MinStep != 10 || cfg.MaxStep != 1000 {
		t.Fatalf("cfg = %+v, want ensured config", cfg)
	}
}

func TestStore_EnsureDoesNotOverwriteExistingSegment(t *testing.T) {
	db := newTestDB(t)
	store := New(db)

	if err := store.Ensure(context.Background(), "test", "order", idgen.EnsureSegmentConfig{
		NextID:  1000,
		Step:    1000,
		MinStep: 1000,
		MaxStep: 1000,
		Status:  idgen.SegmentStatusEnabled,
	}); err != nil {
		t.Fatalf("ensure existing: %v", err)
	}

	r, cfg, err := store.Fetch(context.Background(), "test", "order", 10)
	if err != nil {
		t.Fatalf("fetch existing segment: %v", err)
	}
	if r.Start != 1 || r.End != 11 {
		t.Fatalf("range = [%d,%d), want original [1,11)", r.Start, r.End)
	}
	if cfg.Step != 10 {
		t.Fatalf("step = %d, want original 10", cfg.Step)
	}
}

func TestStore_FetchConcurrentUnique(t *testing.T) {
	db := newTestDB(t)
	store := New(db)
	ctx := context.Background()
	const workers = 8
	const perWorker = 64

	ranges := make(chan idgen.Range, workers*perWorker)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				r, _, err := store.Fetch(ctx, "test", "order", 10)
				if err != nil {
					t.Errorf("fetch: %v", err)
					return
				}
				ranges <- r
			}
		}()
	}
	wg.Wait()
	close(ranges)

	seen := make(map[int64]struct{}, workers*perWorker*10)
	for r := range ranges {
		for id := r.Start; id < r.End; id++ {
			if _, ok := seen[id]; ok {
				t.Fatalf("duplicate id %d", id)
			}
			seen[id] = struct{}{}
		}
	}
	if got, want := len(seen), workers*perWorker*10; got != want {
		t.Fatalf("ids = %d, want %d", got, want)
	}
}

func BenchmarkFetchSegmentGorm(b *testing.B) {
	db := newTestDB(b)
	store := New(db)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := store.Fetch(ctx, "test", "order", 1000); err != nil {
			b.Fatal(err)
		}
	}
}

func newTestDB(tb testing.TB) *gorm.DB {
	tb.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(tb.TempDir(), "idgen.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		tb.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		tb.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err = db.AutoMigrate(&SegmentModel{}); err != nil {
		tb.Fatal(err)
	}
	if err = db.Create(&SegmentModel{
		Namespace: "test",
		Name:      "order",
		NextID:    1,
		Step:      10,
		MinStep:   10,
		MaxStep:   100000,
		Status:    StatusEnabled,
	}).Error; err != nil {
		tb.Fatal(err)
	}
	return db
}
