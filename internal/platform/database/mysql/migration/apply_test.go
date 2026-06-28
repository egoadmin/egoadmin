package migration

import (
	"context"
	"testing"

	"github.com/egoadmin/egoadmin/internal/platform/config"
)

func TestApplyAtlasSkipsDisabledMigration(t *testing.T) {
	t.Parallel()

	err := ApplyAtlas(context.Background(), config.DBMigrationConf{
		Enabled: false,
	}, "file://atlas/migrations/test")
	if err != nil {
		t.Fatalf("ApplyAtlas() error = %v", err)
	}
}

func TestApplyAtlasRejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()

	err := ApplyAtlas(context.Background(), config.DBMigrationConf{
		Enabled: true,
		Driver:  "gorm",
	}, "file://atlas/migrations/test")
	if err == nil {
		t.Fatal("ApplyAtlas() expected error")
	}
}
