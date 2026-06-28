package config

import (
	"fmt"
	"os"
)

// writeRendered 将渲染后的完整配置 TOML 写入临时文件，返回文件路径。
// 首次调用创建临时文件并记录路径；后续调用覆写同一路径，保持路径不变，
// 以便 ego 的 fsnotify watcher 能感知内容变化（监听 Write 事件）。
func (m *Manager) writeRendered(content string) (string, error) {
	if m.tempPath != "" {
		if err := os.WriteFile(m.tempPath, []byte(content), 0o600); err != nil {
			return m.tempPath, fmt.Errorf("update rendered config: %w", err)
		}
		return m.tempPath, nil
	}

	f, err := os.CreateTemp("", "egoadmin-config-*.toml")
	if err != nil {
		return "", fmt.Errorf("create rendered config: %w", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write rendered config: %w", err)
	}
	f.Close()
	m.tempPath = f.Name()
	return f.Name(), nil
}

// removeRendered 删除临时配置文件。
func (m *Manager) removeRendered() {
	if m.tempPath != "" {
		os.Remove(m.tempPath)
		m.tempPath = ""
	}
}
