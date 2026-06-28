package config

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type envBinding struct {
	path  []string
	value any
}

// EnvSuffix returns the environment variable suffix for a TOML path.
func EnvSuffix(path ...string) string {
	parts := make([]string, 0, len(path))
	for _, part := range path {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, strings.ToUpper(part))
	}
	return strings.Join(parts, "_")
}

func (m *Manager) loadDotEnv() error {
	if _, err := os.Stat(".env"); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat .env: %w", err)
	}
	if err := godotenv.Load(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

func (m *Manager) applyEnvOverrides(config map[string]interface{}) error {
	prefix := strings.TrimSpace(m.envPrefix)
	if prefix == "" {
		return nil
	}

	bindings, err := buildEnvBindings(config)
	if err != nil {
		return err
	}
	envPrefix := strings.ToUpper(prefix) + "_"
	for suffix, binding := range bindings {
		raw, ok := os.LookupEnv(envPrefix + suffix)
		if !ok {
			continue
		}
		converted, er := convertEnvValue(raw, binding.value)
		if er != nil {
			return fmt.Errorf("convert env %s%s for %s: %w", envPrefix, suffix, strings.Join(binding.path, "."), er)
		}
		setConfigPath(config, binding.path, converted)
	}
	return nil
}

func buildEnvBindings(config map[string]interface{}) (map[string]envBinding, error) {
	bindings := make(map[string]envBinding)
	if err := walkConfig(config, nil, bindings); err != nil {
		return nil, err
	}
	return bindings, nil
}

func walkConfig(node any, path []string, bindings map[string]envBinding) error {
	if table, ok := node.(map[string]interface{}); ok {
		keys := make([]string, 0, len(table))
		for key := range table {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := walkConfig(table[key], append(path, key), bindings); err != nil {
				return err
			}
		}
		return nil
	}

	if len(path) == 0 {
		return nil
	}
	suffix := EnvSuffix(path...)
	if existing, ok := bindings[suffix]; ok {
		return fmt.Errorf(
			"env suffix conflict %s for %s and %s",
			suffix,
			strings.Join(existing.path, "."),
			strings.Join(path, "."),
		)
	}
	bindings[suffix] = envBinding{
		path:  append([]string(nil), path...),
		value: node,
	}
	return nil
}

func setConfigPath(config map[string]interface{}, path []string, value any) {
	node := config
	for _, part := range path[:len(path)-1] {
		next, _ := node[part].(map[string]interface{})
		node = next
	}
	node[path[len(path)-1]] = value
}

func convertEnvValue(raw string, target any) (any, error) {
	if target == nil {
		return raw, nil
	}
	if _, ok := target.(time.Duration); ok {
		return time.ParseDuration(raw)
	}
	targetType := reflect.TypeOf(target)
	if targetType == reflect.TypeOf(time.Duration(0)) {
		return time.ParseDuration(raw)
	}

	switch target := target.(type) {
	case string:
		return raw, nil
	case bool:
		return cast.ToBoolE(raw)
	case int:
		return cast.ToIntE(raw)
	case int8:
		v, err := cast.ToInt8E(raw)
		return v, err
	case int16:
		v, err := cast.ToInt16E(raw)
		return v, err
	case int32:
		v, err := cast.ToInt32E(raw)
		return v, err
	case int64:
		return cast.ToInt64E(raw)
	case uint:
		return cast.ToUintE(raw)
	case uint8:
		v, err := cast.ToUint8E(raw)
		return v, err
	case uint16:
		v, err := cast.ToUint16E(raw)
		return v, err
	case uint32:
		v, err := cast.ToUint32E(raw)
		return v, err
	case uint64:
		return cast.ToUint64E(raw)
	case float32:
		v, err := cast.ToFloat32E(raw)
		return v, err
	case float64:
		return cast.ToFloat64E(raw)
	case []string:
		return splitEnvStringSlice(raw), nil
	case []interface{}:
		return convertInterfaceSlice(raw, target)
	default:
		return nil, fmt.Errorf("unsupported target type %T", target)
	}
}

func splitEnvStringSlice(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func convertInterfaceSlice(raw string, target []interface{}) ([]interface{}, error) {
	for _, item := range target {
		if _, ok := item.(string); !ok {
			return nil, fmt.Errorf("unsupported target type []interface{} with non-string element")
		}
	}
	parts := splitEnvStringSlice(raw)
	out := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		out = append(out, part)
	}
	return out, nil
}
