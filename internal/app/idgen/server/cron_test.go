package server

import (
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/config"
)

func TestMachineCleanupOptionsDefaults(t *testing.T) {
	t.Parallel()

	retention, limit, err := machineCleanupOptions(&config.Config{})
	if err != nil {
		t.Fatalf("machine cleanup options: %v", err)
	}
	if retention != 7*24*time.Hour {
		t.Fatalf("retention = %s, want 168h", retention)
	}
	if limit != 1000 {
		t.Fatalf("limit = %d, want 1000", limit)
	}
}
