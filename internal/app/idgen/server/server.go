package server

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/idgen/controller"
	"github.com/egoadmin/egoadmin/internal/app/idgen/schema"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql/migration"
	"github.com/egoadmin/egoadmin/internal/platform/health"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/google/wire"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego-component/egorm"
	"github.com/gotomicro/ego/core/eregistry"
	"github.com/gotomicro/ego/server/egin"
	"github.com/gotomicro/ego/server/egovernor"
	"github.com/gotomicro/ego/task/ecron"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(controller.Options), "*"),
	NewGrpcServer,
	NewHttpServer,
	NewGovernServer,
	newMachineLeaseCleanupCron,
)

type Options struct {
	conf     *config.Config
	health   *health.Options
	app      *ego.Ego
	db       *egorm.Component
	http     *egin.Component
	grpc     *GrpcServer
	govern   *egovernor.Component
	cron     ecron.Ecron
	registry eregistry.Registry
	shutdown *shutdown.Manager
}

type App struct {
	*ego.Ego
}

type schemaReady struct{}

func newSchemaReady(conf *config.Config, db mysql.MysqlInterface) (schemaReady, error) {
	if err := migration.ApplyAtlas(context.Background(), conf.DBMigration(), "file://atlas/migrations/idgen"); err != nil {
		return schemaReady{}, err
	}

	if conf.App().AutoMigrate {
		if err := db.Migrate(context.Background(), schema.MigrationModels(), schema.MigrationJoinTables()); err != nil {
			return schemaReady{}, err
		}
	}

	return schemaReady{}, nil
}

func newApp(opts Options, _ schemaReady) (*App, error) {
	configureShutdown(opts)
	opts.app.Registry(opts.registry)
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	if opts.cron != nil {
		opts.app.Cron(opts.cron)
	}
	opts.health.Ready()
	return &App{Ego: opts.app}, nil
}

func newEgo(conf *config.Config) *ego.Ego {
	return ego.New(ego.WithArguments([]string{"--config", conf.RenderedPath()}))
}

// newEgoReady 在 ego.New() 之后产出配置就绪标志。读 econf 的组件依赖它，
// Wire 据此保证这些组件在 ego 加载配置（econf 就绪）之后才构造。
func newEgoReady(_ *ego.Ego) config.EgoReady {
	return config.EgoReady{}
}

func newConfig() *config.Config {
	return config.New(config.WithService(config.ServiceIDGen))
}

func newHealth(c *egin.Component, db *egorm.Component) *health.Options {
	return health.Start(func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		dbx, err := db.DB()
		if err != nil {
			return false
		}
		return dbx.PingContext(ctx) == nil
	}, c)
}
