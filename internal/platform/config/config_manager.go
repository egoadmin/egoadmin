package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
	"github.com/imdario/mergo"
)

//go:embed default_common.toml
var defaultCommonConfigContent string

//go:embed default_gateway.toml
var defaultGatewayConfigContent string

//go:embed default_user.toml
var defaultUserConfigContent string

//go:embed default_idgen.toml
var defaultIDGenConfigContent string

const (
	// DefaultEnvPrefix is the default environment variable prefix for config overrides.
	DefaultEnvPrefix = "EGOADMIN"
)

type ServiceName string

const (
	ServiceGateway ServiceName = "gateway"
	ServiceUser    ServiceName = "user"
	ServiceIDGen   ServiceName = "idgen"
)

// Option 是配置管理器的选项类型
type Option func(*MG)

// MG 配置管理器
type MG struct {
	extendLoadFn func() string // 加载外部配置方法
	envPrefix    string        // 环境变量覆盖前缀
	service      ServiceName   // 服务名
}

// NewMG 新建配置管理器
func NewMG(opts ...Option) *MG {
	m := &MG{
		envPrefix: DefaultEnvPrefix,
		service:   ServiceGateway,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// DefaultConfigTOML returns the embedded default TOML template.
func DefaultConfigTOML(service ServiceName) string {
	doc, err := defaultConfigTOML(service)
	if err != nil {
		return ""
	}
	return doc
}

// DefaultConfigDocument returns the operator-facing default config document.
func DefaultConfigDocument(prefix string, service ServiceName) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = DefaultEnvPrefix
	}
	doc := DefaultConfigTOML(service)
	return fmt.Sprintf(
		"# 环境变量覆盖前缀：%s\n# 完整环境变量名格式：%s_<EnvSuffix>\n\n%s",
		prefix,
		prefix,
		doc,
	)
}

func defaultConfigTOML(service ServiceName) (string, error) {
	service = normalizeService(service)
	switch service {
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

func normalizeService(service ServiceName) ServiceName {
	switch strings.TrimSpace(strings.ToLower(string(service))) {
	case "", "gateway":
		return ServiceGateway
	case "user":
		return ServiceUser
	case "idgen":
		return ServiceIDGen
	default:
		return service
	}
}

// WithExtendLoad 设置加载外部配置的方法
func WithExtendLoad(fn func() string) Option {
	return func(m *MG) {
		m.extendLoadFn = fn
	}
}

// WithEnvPrefix sets the environment variable prefix used for config overrides.
// Empty prefix disables environment variable overrides.
func WithEnvPrefix(prefix string) Option {
	return func(m *MG) {
		m.envPrefix = strings.TrimSpace(prefix)
	}
}

func WithService(service ServiceName) Option {
	return func(m *MG) {
		m.service = normalizeService(service)
	}
}

// Load 加载配置
func (m *MG) Load() {
	m.load()
}

// load 配置混入
//
// 优先用配置文件中的配置,如果没有则用包默认配置.
// 将配置混合到一起,以减少最终程序需配置的配置信息
func (m *MG) load() {
	mergedConfig, err := m.loadAndParseConfig()
	if err != nil {
		elog.Error("配置加载错误", elog.FieldErr(err))
		return
	}
	reader := strings.NewReader(mergedConfig)

	if err = econf.LoadFromReader(reader, toml.Unmarshal); err != nil {
		elog.Error("配置加载错误", elog.FieldErr(err))
	}
}

// loadAndParseConfig 加载和解析配置文件，并返回最终的 TOML 字符串
func (m *MG) loadAndParseConfig() (string, error) {
	userConfig, defaultConfig, err := m.readConfig()
	if err != nil {
		return "", err
	}
	mergedConfig, err := m.mergeConfig(userConfig, defaultConfig)
	if err != nil {
		return "", err
	}

	if err = m.loadDotEnv(); err != nil {
		return "", err
	}
	if err = m.applyEnvOverrides(mergedConfig); err != nil {
		return "", err
	}

	// 创建一个字节缓冲区
	var buf bytes.Buffer

	// 使用 TOML 编码器将配置写入缓冲区
	encoder := toml.NewEncoder(&buf)
	if err = encoder.Encode(mergedConfig); err != nil {
		return "", err
	}

	// 将缓冲区中的内容转换为字符串并返回
	return buf.String(), nil
}

// readConfig 读取用户配置和默认配置
func (m *MG) readConfig() (map[string]interface{}, map[string]interface{}, error) {
	// 从文件中读取用户配置
	userConfig := make(map[string]interface{})
	if err := toml.Unmarshal(econf.RawConfig(), &userConfig); err != nil {
		return nil, nil, err
	}

	// 从文件中读取默认配置
	defaultConfig := make(map[string]interface{})
	defaultConfigContent, err := defaultConfigTOML(m.service)
	if err != nil {
		return nil, nil, err
	}
	if err := toml.Unmarshal([]byte(defaultConfigContent), &defaultConfig); err != nil {
		return nil, nil, err
	}

	return userConfig, defaultConfig, nil
}

// mergeConfig 将用户配置和默认配置进行合并
func (m *MG) mergeConfig(userConfig map[string]interface{}, defaultConfig map[string]interface{}) (map[string]interface{}, error) {
	// 创建一个新的 map 用于存储合并后的配置
	mergedConfig := make(map[string]interface{})

	// 将默认配置合并到最终配置中
	if err := mergo.Merge(&mergedConfig, defaultConfig, mergo.WithOverride); err != nil {
		// 处理错误
		return nil, err
	}

	// 合并外部配置,如数据库等
	if m.extendLoadFn != nil {
		exConfTOML := m.extendLoadFn()
		if exConfTOML != "" {
			exConf := make(map[string]interface{})
			if er := toml.Unmarshal([]byte(exConfTOML), &exConf); er != nil {
				return nil, er
			}

			if er := mergo.Merge(&mergedConfig, exConf, mergo.WithOverride); er != nil {
				return nil, er
			}
		}
	}

	// 将用户配置合并到最终配置中
	if err := mergo.Merge(&mergedConfig, userConfig, mergo.WithOverride); err != nil {
		// 处理错误
		return nil, err
	}

	return mergedConfig, nil
}
