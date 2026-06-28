package etusupload

import (
	"os"
	"path/filepath"
	"time"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

// createTusStore 创建TUS存储和composer
func createTusStore(dataDir string) (tusd.DataStore, *tusd.StoreComposer, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, err
	}

	// 创建文件存储
	store := filestore.FileStore{
		Path: dataDir,
	}

	// 创建文件锁
	locker := filelocker.New(dataDir)

	// 创建composer
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	return composer.Core, composer, nil
}

// CleanupOldUploads 清理旧的未完成上传，maxAge 单位为秒。
func CleanupOldUploads(dataDir string, maxAge int64) error {
	maxAgeDuration := time.Duration(maxAge) * time.Second
	return filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理info文件
		if filepath.Ext(path) != ".info" {
			return nil
		}

		if time.Since(info.ModTime()) > maxAgeDuration {
			// 删除info文件和对应的bin文件
			os.Remove(path)
			binPath := path[:len(path)-5] + ".bin"
			os.Remove(binPath)
		}

		return nil
	})
}
