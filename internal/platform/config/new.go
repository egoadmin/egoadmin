package config

import (
	"github.com/gotomicro/ego/core/econf"
	"github.com/gotomicro/ego/core/elog"
)

// New 加载配置.
func New(opts ...Option) *Config {
	conf := &Config{}

	// 数据库配置管理器
	dbWatcher := DBConfig{}

	mgOptions := append([]Option{
		WithExtendLoad(func() string {
			return dbWatcher.Load()
		}),
	}, opts...)
	mg := NewMG(mgOptions...)
	conf.service = mg.service
	mg.Load()

	// 自定义配置初始化
	err := unmarhsalConfig(&econfUnmarshal{}, conf)
	if err != nil {
		panic(err)
	}

	// 数据库中配置监听
	dbWatcher.OnChange(func() {
		elog.Info("config changed, reloading...")
		mg.Load()

		err := unmarhsalConfig(&econfUnmarshal{}, conf)
		if err != nil {
			elog.Error("econf onchange unmarshal", elog.FieldErr(err))
		}
	})

	// 自定义配置监听
	econf.OnChange(func(config *econf.Configuration) {
		elog.Info("config changed, reloading...")
		mg.Load()

		err := unmarhsalConfig(config, conf)
		if err != nil {
			elog.Error("econf onchange unmarshal", elog.FieldErr(err))
		}
	})

	return conf
}

// econfUnmarshal 对象存储配置解析.
type econfUnmarshal struct{}

func (ec *econfUnmarshal) UnmarshalKey(key string, rawVal any, opts ...econf.Option) error {
	return econf.UnmarshalKey(key, rawVal, opts...)
}

// Unmarshaller 反序列化配置.
type Unmarshaller interface {
	UnmarshalKey(key string, rawVal any, opts ...econf.Option) error
}

// unmarhsalConfig 解析配置.
func unmarhsalConfig(ue Unmarshaller, cf *Config) (err error) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	// 服务配置
	err = ue.UnmarshalKey("app.service", &cf.app)
	if err != nil {
		return
	}

	// 停机配置
	err = ue.UnmarshalKey("app.shutdown", &cf.shutdown)
	if err != nil {
		return
	}

	// 数据库版本迁移配置
	err = ue.UnmarshalKey("app.dbMigration", &cf.dbMigration)
	if err != nil {
		return
	}

	switch normalizeService(cf.service) {
	case ServiceGateway:
		// 前端运行时配置
		err = ue.UnmarshalKey("app.web", &cf.web)
		if err != nil {
			return
		}
	case ServiceUser:
		// 用户管理配置
		err = ue.UnmarshalKey("app.user", &cf.user)
		if err != nil {
			return
		}
	case ServiceIDGen:
		// ID 生成服务配置
		err = ue.UnmarshalKey("app.idgen", &cf.idgen)
		if err != nil {
			return
		}
	}

	return nil
}
