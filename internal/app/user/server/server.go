package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoadmin/egoadmin/internal/app/user/controller"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/job"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/schema"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	idgenclient "github.com/egoadmin/egoadmin/internal/client/idgenclient"
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/platform/captcha"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql/migration"
	"github.com/egoadmin/egoadmin/internal/platform/health"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	ut "github.com/go-playground/universal-translator"
	validv10 "github.com/go-playground/validator/v10"
	"github.com/google/wire"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego-component/egorm"
	"github.com/gotomicro/ego/core/eregistry"
	"github.com/gotomicro/ego/server/egin"
	"github.com/gotomicro/ego/server/egovernor"
)

var ProviderSet = wire.NewSet(
	wire.Struct(new(controller.Options), "*"),
	NewGrpcServer,
	NewHttpServer,
	NewGovernServer,
)

type Options struct {
	conf        *config.Config
	health      *health.Options
	app         *ego.Ego
	db          *egorm.Component
	http        *egin.Component
	grpc        *GrpcServer
	govern      *egovernor.Component
	registry    eregistry.Registry
	shutdown    *shutdown.Manager
	redis       *eredis.Component
	jetcache    *jetcache.Component
	cron        *job.Cron
	idgenClient *idgenclient.Client
	idgen       *idgen.Component
	idm         idgen.MachineLeaseManager
	user        *service.UserService
}

type App struct {
	*ego.Ego
}

type schemaReady struct{}

func newSchemaReady(conf *config.Config, db mysql.MysqlInterface) (schemaReady, error) {
	if err := migration.ApplyAtlas(context.Background(), conf.DBMigration(), "file://atlas/migrations/user"); err != nil {
		return schemaReady{}, err
	}

	if conf.App().AutoMigrate {
		if err := db.Migrate(context.Background(), schema.MigrationModels(), schema.MigrationJoinTables()); err != nil {
			return schemaReady{}, err
		}
	}

	return schemaReady{}, nil
}

func newApp(opts Options) (*App, error) {
	cleanup := func() {
		if opts.idgen != nil {
			_ = opts.idgen.Stop()
		}
		if opts.idm != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			_ = opts.idm.Stop(ctx)
		}
	}

	if opts.idm != nil {
		if err := opts.idm.Start(context.Background()); err != nil {
			cleanup()
			return nil, fmt.Errorf("start idgen machine manager: %w", err)
		}
	}
	if opts.idgen != nil {
		if err := opts.idgen.Start(); err != nil {
			cleanup()
			return nil, fmt.Errorf("start idgen: %w", err)
		}
	}

	configureShutdown(opts)

	opts.app.Registry(opts.registry)
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	opts.app.Cron(opts.cron.Tasks()...)
	if opts.idm != nil {
		opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm))
	}

	if err := opts.user.CreateSuperuser(context.Background()); err != nil {
		cleanup()
		return nil, err
	}
	if err := opts.user.WarmupLoginCrypto(context.Background()); err != nil {
		cleanup()
		return nil, err
	}

	opts.health.Ready()

	return &App{Ego: opts.app}, nil
}

func newEgo() *ego.Ego {
	return ego.New()
}

func newConfig(e *ego.Ego) *config.Config {
	return config.New(config.WithService(config.ServiceUser))
}

func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
	return health.Start(func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		dbx, err := db.DB()
		if err != nil {
			return false
		}
		if err = dbx.PingContext(ctx); err != nil {
			return false
		}
		if _, err = rds.Ping(ctx); err != nil {
			return false
		}
		return true
	}, c)
}

func newLoger(logger store.LogInterface, user store.UserInterface, dept store.DeptInterface) auditlog.Loger {
	return auditlog.New(func(ctx context.Context, alog auditlog.AccessLogDetail) error {
		logArr := strings.Split(alog.Action, ".")
		moduleName := ""
		typName := ""
		title := ""
		if len(logArr) >= 3 {
			moduleName = logArr[0]
			typName = logArr[1]
			title = logArr[2]
		}

		lm := &store.LogModel{
			UserID:     alog.UserID,
			Username:   alog.Username,
			Typ:        typName,
			ModuleName: moduleName,
			Title:      title,
			URL:        alog.URL,
			Method:     alog.GrpcMethod,
			ClientIP:   alog.OriginIP,
			Params:     alog.Params,
			Remark:     alog.Remark,
		}

		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		uid, err := strconv.ParseUint(alog.UserID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse audit user id: %w", err)
		}
		lm.UserIDU64 = uid
		us, err := user.Get(ctx, uid)
		if err != nil {
			return fmt.Errorf("get audit user: %w", err)
		}
		dp, err := dept.GetSelf(ctx, us.DeptID)
		if err != nil {
			dp = &store.DeptModel{}
		}
		lm.DeptID = strconv.FormatUint(us.DeptID, 10)
		lm.DeptIDU64 = us.DeptID
		lm.DeptName = dp.DeptName
		if err := logger.Save(ctx, lm); err != nil {
			return fmt.Errorf("save audit log: %w", err)
		}
		return nil
	})
}

func newCasbin(cc *egorm.Component, _ schemaReady) (*perm.Casbin, error) {
	cb, err := perm.NewCasbin(
		perm.WithCasbinGormAdapter(cc),
		perm.WithCasbinModelString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) == true \
	&& r.obj == p.obj \
	&& r.act == p.act \
	|| r.sub == "root" || r.sub == "admin" \`),
	)
	if err != nil {
		return nil, err
	}

	return cb, nil
}

func newValidate() *validate.Validate {
	tag := "chinese"
	v, err := validate.NewV10(validate.WithAddonV10(validate.AddonV10{
		Tag: tag,
		ValidFn: func(fl validv10.FieldLevel) bool {
			if str, ok := fl.Field().Interface().(string); ok {
				for _, char := range str {
					if !unicode.Is(unicode.Scripts["Han"], char) {
						return false
					}
				}
				return true
			}
			return false
		},
		CallValidationEvenIfNull: []bool{},
		RegisterTransFn: func(utl ut.Translator) error {
			return utl.Add(tag, "{0}不正确", false)
		},
		TransFn: func(utl ut.Translator, fe validv10.FieldError) string {
			t, err := utl.T(tag, fe.Field())
			if err != nil {
				return ""
			}

			return t
		},
	}))
	if err != nil {
		panic(err)
	}

	return v
}

func newCaptcha() captcha.ICaptcha {
	return captcha.NewCaptcha()
}
