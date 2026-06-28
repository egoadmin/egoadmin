package config

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed default_common.toml
var defaultCommonConfigContent string

//go:embed default_gateway.toml
var defaultGatewayConfigContent string

//go:embed default_user.toml
var defaultUserConfigContent string

//go:embed default_idgen.toml
var defaultIDGenConfigContent string

// DefaultEnvPrefix 是配置环境变量覆盖的默认前缀。
const DefaultEnvPrefix = "EGOADMIN"

// defaultTOML 返回指定服务的内置默认配置 TOML（common + 服务专属）。
func defaultTOML(service Service) (string, error) {
	switch normalizeService(service) {
	case ServiceGateway:
		return defaultCommonConfigContent + "\n" + defaultGatewayConfigContent, nil
	case ServiceUser:
		return defaultCommonConfigContent + "\n" + defaultUserConfigContent, nil
	case ServiceIDGen:
		return defaultCommonConfigContent + "\n" + defaultIDGenConfigContent, nil
	default:
		return "", fmt.Errorf("unknown config service %q", service)
	}
}

// DefaultConfigTOML 返回指定服务的内置默认配置 TOML 模板。
func DefaultConfigTOML(service Service) string {
	doc, err := defaultTOML(service)
	if err != nil {
		return ""
	}
	return doc
}

// DefaultConfigDocument 返回面向运维的默认配置文档，含环境变量覆盖说明。
func DefaultConfigDocument(prefix string, service Service) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = DefaultEnvPrefix
	}
	return fmt.Sprintf(
		"# 环境变量覆盖前缀：%s\n# 完整环境变量名格式：%s_<EnvSuffix>\n\n%s",
		prefix,
		prefix,
		DefaultConfigTOML(service),
	)
}
