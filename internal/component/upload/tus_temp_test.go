package upload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupTusLocalTempDeletesOnlyExpiredTusFiles(t *testing.T) {
	dir := t.TempDir()
	oldTemp := filepath.Join(dir, "tusd-s3-tmp-old")
	newTemp := filepath.Join(dir, "tusd-s3-tmp-new")
	other := filepath.Join(dir, "other.tmp")
	mustWriteFile(t, oldTemp, "old")
	mustWriteFile(t, newTemp, "new")
	mustWriteFile(t, other, "other")
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldTemp, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old temp: %v", err)
	}

	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithConfig(&Config{
		MultipartPath: "/upload",
		Profiles:      DefaultConfig().Profiles,
		Tus: TusConfig{
			TemporaryDirectory: dir,
			MetadataPrefix:     "tus-meta",
			LocalTempTTL:       24 * time.Hour,
			MaxTempDirSize:     1024,
		},
	}), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.CleanupTusLocalTemp(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("CleanupTusLocalTemp() error = %v", err)
	}
	if result.DeletedFiles != 1 {
		t.Fatalf("DeletedFiles = %d, want 1", result.DeletedFiles)
	}
	if _, err = os.Stat(oldTemp); !os.IsNotExist(err) {
		t.Fatalf("old temp still exists or stat error = %v", err)
	}
	if _, err = os.Stat(newTemp); err != nil {
		t.Fatalf("new temp should remain: %v", err)
	}
	if _, err = os.Stat(other); err != nil {
		t.Fatalf("other file should remain: %v", err)
	}
}

func TestCheckTusLocalTempLimit(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "tusd-s3-tmp-big"), "12345")
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithConfig(&Config{
		MultipartPath: "/upload",
		Profiles:      DefaultConfig().Profiles,
		Tus: TusConfig{
			TemporaryDirectory: dir,
			MetadataPrefix:     "tus-meta",
			LocalTempTTL:       24 * time.Hour,
			MaxTempDirSize:     4,
		},
	}), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err = component.CheckTusLocalTempLimit(context.Background()); err == nil {
		t.Fatalf("CheckTusLocalTempLimit() error = nil, want over limit")
	}
}

func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
