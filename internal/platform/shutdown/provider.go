package shutdown

import (
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/google/wire"
	"github.com/gotomicro/ego/core/elog"
)

var ProviderSet = wire.NewSet(
	NewConfig,
	NewLogger,
	NewManager,
)

func NewConfig(conf *config.Config) Config {
	if conf == nil {
		return DefaultConfig()
	}
	c := conf.Shutdown()
	return Config{
		StopTimeout:  parseDuration(c.StopTimeout),
		DrainTimeout: parseDuration(c.DrainTimeout),
		CloseTimeout: parseDuration(c.CloseTimeout),
	}.Normalize()
}

func NewLogger() *elog.Component {
	return elog.EgoLogger
}

func parseDuration(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return duration
}
