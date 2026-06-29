package shutdown

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/health"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/core/eregistry"
)

// CloseFunc closes a runtime resource. The context carries the configured
// close timeout for resources that can honor cancellation.
type CloseFunc func(context.Context) error

type closer struct {
	name string
	fn   CloseFunc
}

// Manager owns process-level shutdown hooks that are not native EGO servers.
type Manager struct {
	config Config
	health *health.Options
	logger *elog.Component
	closer []closer
}

func NewManager(config Config, health *health.Options, logger *elog.Component) *Manager {
	if logger == nil {
		logger = elog.EgoLogger
	}
	return &Manager{
		config: config.Normalize(),
		health: health,
		logger: logger,
		closer: make([]closer, 0),
	}
}

func (m *Manager) Config() Config {
	if m == nil {
		return DefaultConfig()
	}
	return m.config
}

func (m *Manager) Register(name string, fn CloseFunc) {
	if m == nil || fn == nil {
		return
	}
	if name == "" {
		name = "resource"
	}
	m.closer = append(m.closer, closer{name: name, fn: fn})
}

func (m *Manager) RegisterCloser(name string, c interface{ Close() error }) {
	if c == nil {
		return
	}
	m.Register(name, func(context.Context) error {
		return c.Close()
	})
}

func (m *Manager) RegisterRegistry(reg eregistry.Registry) {
	if m == nil || reg == nil {
		return
	}
	m.RegisterCloser("registry", reg)
}

func (m *Manager) RegisterDB(name string, db interface {
	DB() (*sql.DB, error)
},
) {
	if m == nil || db == nil {
		return
	}
	m.Register(name, func(context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	})
}

func (m *Manager) Bind(app *ego.Ego) {
	if m == nil || app == nil {
		return
	}
	ego.WithStopTimeout(m.config.StopTimeout)(app)
	ego.WithBeforeStopClean(m.beforeStop)(app)
	ego.WithAfterStopClean(m.afterStop, elog.DefaultLogger.Flush, elog.EgoLogger.Flush)(app)
}

func (m *Manager) beforeStop() error {
	if m == nil {
		return nil
	}
	if m.health != nil {
		m.health.NotReady()
		m.logger.Info("service marked not ready", elog.FieldComponent("shutdown"))
	}
	if m.config.DrainTimeout > 0 {
		timer := time.NewTimer(m.config.DrainTimeout)
		<-timer.C
	}
	return nil
}

func (m *Manager) afterStop() error {
	if m == nil {
		return nil
	}
	var joined error
	for i := len(m.closer) - 1; i >= 0; i-- {
		c := m.closer[i]
		ctx, cancel := context.WithTimeout(context.Background(), m.config.CloseTimeout)
		err := c.fn(ctx)
		cancel()
		if err != nil {
			wrapped := fmt.Errorf("%s: %w", c.name, err)
			joined = errors.Join(joined, wrapped)
			m.logger.Error("shutdown close resource", elog.FieldComponent("shutdown"), elog.FieldName(c.name), elog.FieldErr(err))
			continue
		}
		m.logger.Info("shutdown close resource", elog.FieldComponent("shutdown"), elog.FieldName(c.name))
	}
	return joined
}
