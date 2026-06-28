package upload

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalTempCleanupResult struct {
	DeletedFiles int
	DeletedBytes int64
	CurrentBytes int64
}

func (c *Component) CleanupTusLocalTemp(ctx context.Context, now time.Time) (*LocalTempCleanupResult, error) {
	dir := c.config.Tus.TemporaryDirectory
	if dir == "" {
		return &LocalTempCleanupResult{}, nil
	}
	result := &LocalTempCleanupResult{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		result.CurrentBytes += info.Size()
		if !isTusTempFile(entry.Name()) {
			return nil
		}
		if now.Sub(info.ModTime()) < c.config.Tus.LocalTempTTL {
			return nil
		}
		if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		result.DeletedFiles++
		result.DeletedBytes += info.Size()
		result.CurrentBytes -= info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return result, nil
	}
	return result, err
}

func (c *Component) CheckTusLocalTempLimit(ctx context.Context) error {
	limit := c.config.Tus.MaxTempDirSize
	if limit <= 0 || c.config.Tus.TemporaryDirectory == "" {
		return nil
	}
	size, err := dirSize(ctx, c.config.Tus.TemporaryDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if size > limit {
		return fmt.Errorf("upload: tus temporary directory exceeds limit")
	}
	return nil
}

func dirSize(ctx context.Context, dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func isTusTempFile(name string) bool {
	return strings.HasPrefix(name, "tusd-s3-tmp-")
}
