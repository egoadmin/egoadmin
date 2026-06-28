package migration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/egoadmin/egoadmin/internal/platform/config"
)

// ApplyAtlas applies Atlas migrations for one service database boundary.
func ApplyAtlas(ctx context.Context, conf config.DBMigrationConf, defaultDir string) error {
	if skipped, _ := strconv.ParseBool(os.Getenv("EGOADMIN_ATLAS_MIGRATED")); skipped {
		return nil
	}
	if !conf.Enabled {
		return nil
	}
	if conf.Driver != "" && !strings.EqualFold(conf.Driver, "atlas") {
		return fmt.Errorf("unsupported db migration driver %q", conf.Driver)
	}

	bin := conf.Bin
	if bin == "" {
		bin = "atlas"
	}
	url := os.ExpandEnv(conf.URL)
	if url == "" {
		url = os.Getenv("ATLAS_URL")
	}
	if url == "" {
		return fmt.Errorf("atlas migration url is empty")
	}
	dir := os.ExpandEnv(conf.Dir)
	if dir == "" {
		dir = defaultDir
	}

	cmd := exec.CommandContext(ctx, bin, "migrate", "apply", "--url", url, "--dir", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
