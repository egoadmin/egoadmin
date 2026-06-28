package config

import (
	"sort"
	"strings"

	"github.com/gotomicro/ego/core/elog"
)

// Option 配置 Manager 的可选项。
type Option func(*Manager)

// EgoReady 是一个标志类型，表示 ego.New() 已执行完毕、配置已由 ego 的 loadConfig
// 加载进全局 econf。所有在构造时读取 econf 的 ego 组件（egin/egrpc/egovernor/
// egorm/eredis/eetcd 等）应在其构造函数中接收 EgoReady 参数，从而让 Wire 强制
// 这些组件在 newEgo() 之后构造，确保它们读到的 econf 已包含合并并覆盖后的配置。
//
// 它不携带任何数据，仅用于在 Wire 依赖图中表达「ego 配置已就绪」这一顺序约束。
type EgoReady struct{}

// Manager 是多源配置编排器：按优先级合并各配置源 + 环境变量覆盖，
// 产出给 ego 使用的临时配置文件，并把配置绑定到 typed *Config。
// 任一源变化时自动重新合并、覆写临时文件并重新绑定。
//
// Manager 不依赖 ego 的全局 econf：它自己读取源文件、自己监听变化。
// ego 框架只通过 --config 被动读取临时文件，两者仅以临时文件为接触点。
type Manager struct {
	service   Service
	envPrefix string
	sources   []Source
	bind      func(string) // 把渲染后的 TOML 绑定到 *Config

	tempPath string // 渲染产物临时文件路径
	rendered string // 最近一次渲染的完整 TOML
}

// WithService 设置服务标识，决定使用哪份内置默认配置。
func WithService(service Service) Option {
	return func(m *Manager) {
		m.service = normalizeService(service)
	}
}

// WithEnvPrefix 设置环境变量覆盖前缀。空前缀禁用环境变量覆盖。
func WithEnvPrefix(prefix string) Option {
	return func(m *Manager) {
		m.envPrefix = strings.TrimSpace(prefix)
	}
}

// WithSource 追加一个自定义配置源（按 Priority 参与合并）。
func WithSource(src Source) Option {
	return func(m *Manager) {
		if src != nil {
			m.sources = append(m.sources, src)
		}
	}
}

// New 加载配置并返回 typed *Config。
//
// 流程：
//  1. 组装配置源：内置默认 + 文件 + 数据库覆盖（按优先级）。
//  2. 合并所有源并应用环境变量覆盖。
//  3. 将渲染结果写入临时文件，供 ego.New(--config) 使用。
//  4. 解析渲染结果到 typed *Config 供业务使用。
//  5. 注册各源的变化监听，任一源变化时重做 2-4。
func New(opts ...Option) *Config {
	conf := &Config{}

	m := &Manager{
		service:   ServiceGateway,
		envPrefix: DefaultEnvPrefix,
		bind:      conf.bindTOML,
	}
	for _, opt := range opts {
		opt(m)
	}

	// 内置源：默认配置 + 源文件 + 数据库覆盖。自定义源（WithSource）已追加在后。
	m.sources = append(m.sources,
		newDefaultSource(m.service),
		newFileSource(resolveSourcePath()),
		newDBSource(),
	)
	sort.SliceStable(m.sources, func(i, j int) bool {
		return m.sources[i].Priority() < m.sources[j].Priority()
	})

	conf.service = m.service
	conf.manager = m

	m.reload()

	for _, src := range m.sources {
		src.Watch(m.reload)
	}

	return conf
}

// reload 重新合并所有源、覆写临时文件并重新绑定到 *Config。
// 由 New 初始化时调用，以及任一源变化时由 Watch 回调触发。
func (m *Manager) reload() {
	merged, err := m.mergeSources()
	if err != nil {
		elog.Error("config reload merge", elog.FieldErr(err))
		return
	}
	rendered, err := renderTOML(merged)
	if err != nil {
		elog.Error("config reload render", elog.FieldErr(err))
		return
	}
	m.rendered = rendered

	if _, err := m.writeRendered(rendered); err != nil {
		elog.Error("config reload write temp", elog.FieldErr(err))
		return
	}
	if m.bind != nil {
		m.bind(rendered)
	}
}
