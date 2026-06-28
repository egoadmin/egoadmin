package config

// 外部配置,优先级为中等
type ConfigWatcher interface {
	Load() string
	OnChange(func())
}

// DBConfig 数据库配置
type DBConfig struct{}

// Load 配置加载方法
//
// 被加载的配置需要被转换为TOML格式的文本
func (d *DBConfig) Load() string {
	return ""
}

// OnChange 实现在数据库中配置变化时进行通知执行
func (d *DBConfig) OnChange(fn func()) {
}
