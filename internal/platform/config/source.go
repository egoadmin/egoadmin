package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gotomicro/ego/core/constant"
	"github.com/gotomicro/ego/core/elog"
)

// Source 是一个配置源。它提供一段 TOML 配置内容，并能在内容变化时通知。
// 多个 Source 按 Priority 从低到高合并，数值大的覆盖数值小的。
type Source interface {
	// Name 源名称，用于日志。
	Name() string
	// Priority 合并优先级，数值越大覆盖优先级越高。
	Priority() int
	// Load 返回该源当前的 TOML 内容。空字符串表示无内容。
	Load() (string, error)
	// Watch 注册内容变化回调。不支持热重载的源可空实现。
	Watch(onChange func())
}

// 内置源优先级。
const (
	priorityDefault = 0  // 内置默认配置
	priorityFile    = 10 // 文件配置
	priorityDB      = 20 // 数据库覆盖配置
)

// defaultSource 内置默认配置源（embed），优先级最低，不支持热重载。
type defaultSource struct {
	service Service
}

func newDefaultSource(service Service) *defaultSource {
	return &defaultSource{service: service}
}

func (s *defaultSource) Name() string  { return "default" }
func (s *defaultSource) Priority() int { return priorityDefault }
func (s *defaultSource) Watch(func())  {}

func (s *defaultSource) Load() (string, error) {
	return defaultTOML(s.service)
}

// fileSource 文件配置源。读取源配置文件并通过 fsnotify 监听其变化。
type fileSource struct {
	path string
}

func newFileSource(path string) *fileSource {
	return &fileSource{path: strings.TrimSpace(path)}
}

func (s *fileSource) Name() string  { return "file" }
func (s *fileSource) Priority() int { return priorityFile }

func (s *fileSource) Load() (string, error) {
	if s.path == "" {
		return "", nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// 源文件不存在时退化为只用默认配置，不报错。
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// Watch 监听源文件的写入和重建事件（覆盖编辑器原子保存、k8s ConfigMap 软链替换）。
func (s *fileSource) Watch(onChange func()) {
	if s.path == "" || onChange == nil {
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		elog.Error("config file watcher init", elog.FieldErr(err))
		return
	}

	dir := filepath.Dir(s.path)
	target := filepath.Clean(s.path)
	if err := watcher.Add(dir); err != nil {
		elog.Error("config file watcher add", elog.FieldErr(err), elog.String("dir", dir))
		watcher.Close()
		return
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != target {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				onChange()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				elog.Error("config file watcher error", elog.FieldErr(err))
			}
		}
	}()
}

// dbSource 数据库覆盖配置源。优先级最高，预留扩展，当前为空实现。
// 后续接入数据库配置中心时，Load 返回数据库中的 TOML 覆盖内容，
// Watch 在数据库配置变化时调用 onChange 触发整体重载。
type dbSource struct{}

func newDBSource() *dbSource { return &dbSource{} }

func (s *dbSource) Name() string          { return "db" }
func (s *dbSource) Priority() int         { return priorityDB }
func (s *dbSource) Load() (string, error) { return "", nil }
func (s *dbSource) Watch(func())          {}

// resolveSourcePath 解析源配置文件路径，优先级与 ego 保持一致：
// 命令行 --config 参数 > EGO_CONFIG_PATH 环境变量 > ego 默认值。
// ego 的 config flag 在 ego.New() 内部才注册，此处无法用 eflag.String，
// 因此手动扫描 os.Args。
func resolveSourcePath() string {
	if p := configFlagFromArgs(os.Args[1:]); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv(constant.EgoConfigPath)); p != "" {
		return p
	}
	return constant.DefaultConfig
}

// configFlagFromArgs 从命令行参数中提取 --config / -config 的值。
// 支持 "--config=path"、"--config path" 及单横线形式。
func configFlagFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "--config", "-config":
			if hasValue {
				return strings.TrimSpace(value)
			}
			if i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
		}
	}
	return ""
}
