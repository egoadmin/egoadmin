package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileSourceWatchSkipsMissingFile(t *testing.T) {
	t.Parallel()

	called := make(chan struct{}, 1)
	src := newFileSource(filepath.Join(t.TempDir(), "missing", "config.toml"))

	src.Watch(func() {
		called <- struct{}{}
	})

	select {
	case <-called:
		t.Fatal("watch callback was called for missing config file")
	case <-time.After(20 * time.Millisecond):
	}
}
