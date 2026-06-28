package config

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/imdario/mergo"
)

// mergeSources 按优先级从低到高合并所有配置源，再应用环境变量覆盖，
// 返回最终的完整配置 map。sources 必须已按 Priority 升序排列。
func (m *Manager) mergeSources() (map[string]any, error) {
	merged := make(map[string]any)

	for _, src := range m.sources {
		content, err := src.Load()
		if err != nil {
			return nil, fmt.Errorf("load config source %q: %w", src.Name(), err)
		}
		if content == "" {
			continue
		}
		part := make(map[string]any)
		if err := toml.Unmarshal([]byte(content), &part); err != nil {
			return nil, fmt.Errorf("parse config source %q: %w", src.Name(), err)
		}
		if err := mergo.Merge(&merged, part, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("merge config source %q: %w", src.Name(), err)
		}
	}

	// 环境变量覆盖（优先级最高）。
	if err := m.loadDotEnv(); err != nil {
		return nil, err
	}
	if err := m.applyEnvOverrides(merged); err != nil {
		return nil, err
	}

	return merged, nil
}

// renderTOML 将配置 map 渲染为 TOML 文本。
func renderTOML(config map[string]any) (string, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(config); err != nil {
		return "", fmt.Errorf("encode config toml: %w", err)
	}
	return buf.String(), nil
}
