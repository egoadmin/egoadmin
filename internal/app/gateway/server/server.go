package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"
	"unicode"

	egoadmin "github.com/egoadmin/egoadmin"
	"github.com/egoadmin/egoadmin/internal/app/gateway/application"
	"github.com/egoadmin/egoadmin/internal/app/gateway/controller"
	"github.com/egoadmin/egoadmin/internal/app/gateway/internal/job"
	"github.com/egoadmin/egoadmin/internal/app/gateway/internal/web"
	"github.com/egoadmin/egoadmin/internal/app/gateway/schema"
	idgenclient "github.com/egoadmin/egoadmin/internal/client/idgenclient"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	cdncomponent "github.com/egoadmin/egoadmin/internal/component/cdn"
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql/migration"
	"github.com/egoadmin/egoadmin/internal/platform/health"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/egoadmin/internal/platform/shutdown"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/gin-gonic/gin"
	ut "github.com/go-playground/universal-translator"
	validv10 "github.com/go-playground/validator/v10"
	"github.com/google/wire"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego-component/egorm"
	"github.com/gotomicro/ego/core/elog"
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
	db          *egorm.Component
	app         *ego.Ego
	apiSrv      *application.APIUseCase
	http        *egin.Component
	grpc        *GrpcServer
	govern      *egovernor.Component
	registry    eregistry.Registry
	shutdown    *shutdown.Manager
	redis       *eredis.Component
	cron        *job.Cron
	idgen       *idgen.Component
	idm         idgen.MachineLeaseManager
	upload      *uploadcomponent.Component
	cdn         *cdncomponent.Component
	userClient  *userclient.Client
	idgenClient *idgenclient.Client

	permission  *application.PermissionUseCase
	schemaReady schemaReady
}

type App struct {
	*ego.Ego
}

type schemaReady struct{}

func newSchemaReady(conf *config.Config, db mysql.MysqlInterface) (schemaReady, error) {
	if err := migration.ApplyAtlas(context.Background(), conf.DBMigration(), "file://atlas/migrations/gateway"); err != nil {
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
	// http grpc governor
	opts.app.Serve(opts.http, opts.grpc, opts.govern)
	opts.app.Cron(opts.cron.Tasks()...)
	if opts.idm != nil {
		opts.app.Cron(idgen.NewMachineLeaseRenewCron(opts.idm))
	}

	// 接口字典同步
	if err := opts.apiSrv.SyncFromCatalog(context.Background(), egoadmin.APICatalog); err != nil {
		cleanup()
		return nil, err
	}

	if opts.conf.App().SkipPermissionContractCheck {
		elog.Info("开发模式：已跳过前端权限契约启动校验")
	}
	if err := opts.permission.EnsurePermissionContract(context.Background()); err != nil {
		cleanup()
		return nil, err
	}

	// 文件上传
	if err := uploadcomponent.RegisterRoutes(opts.http, opts.upload, uploadcomponent.MultipartOptions{
		BeforeHandle: func(ctx *gin.Context) (*uploadcomponent.AuthContext, error) {
			return validateUploadAuth(ctx.Request.Context(), ctx.Request.Header, opts.userClient)
		},
		BeforeTusHandle: func(ctx context.Context, header http.Header) (*uploadcomponent.AuthContext, error) {
			return validateUploadAuth(ctx, header, opts.userClient)
		},
	}); err != nil {
		cleanup()
		return nil, err
	}

	// 文件下载和图片处理 CDN 入口
	cdncomponent.RegisterRoutes(opts.http, opts.cdn, cdncomponent.Options{
		BeforeFileHandle: func(ctx *gin.Context) (*cdncomponent.AuthContext, error) {
			auth, err := validateUploadAuth(ctx.Request.Context(), ctx.Request.Header, opts.userClient)
			if err != nil {
				return nil, err
			}
			return &cdncomponent.AuthContext{UserID: auth.UserID}, nil
		},
	})

	// 初始化web
	web.StartWithFS(egoadmin.FrontendAssets, opts.conf.Web(), opts.http)
	// 准备就绪
	opts.health.Ready()

	return &App{Ego: opts.app}, nil
}

func validateUploadAuth(ctx context.Context, header http.Header, client *userclient.Client) (*uploadcomponent.AuthContext, error) {
	reqCtx := platformi18n.WithAcceptLanguage(ctx, header.Get(platformi18n.HeaderAcceptLanguage))
	token, err := extractBearerTokenFromValue(ctx, header.Get("Authorization"))
	var auth *authsession.AuthContext
	if err == nil {
		auth, err = client.InternalAuth.ValidateAccessToken(reqCtx, token)
	}
	if err != nil {
		return nil, platformi18n.ErrorFailed(reqCtx, "AuthMissingToken", nil)
	}
	return &uploadcomponent.AuthContext{UserID: auth.UserID}, nil
}

func newEgo(conf *config.Config) *ego.Ego {
	return ego.New(ego.WithArguments([]string{"--config", conf.RenderedPath()}))
}

// newEgoReady 在 ego.New() 之后产出配置就绪标志。读 econf 的组件依赖它，
// Wire 据此保证这些组件在 ego 加载配置（econf 就绪）之后才构造。
func newEgoReady(_ *ego.Ego) config.EgoReady {
	return config.EgoReady{}
}

// newConfig 配置处理
func newConfig() *config.Config {
	return config.New(config.WithService(config.ServiceGateway))
}

func newFrontendAssetsFS() fs.FS {
	return egoadmin.FrontendAssets
}

// newHealth 初始化健康检查
func newHealth(c *egin.Component, db *egorm.Component, rds *eredis.Component) *health.Options {
	return health.Start(func() bool {
		var err error
		defer func() {
			if err != nil {
				err = nil
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		// 检查数据库状态
		dbx, err := db.DB()
		if err != nil {
			return false
		}
		err = dbx.PingContext(ctx)
		if err != nil {
			return false
		}

		// 检查redis状态
		_, err = rds.Ping(ctx)
		if err != nil {
			return false
		}

		return true
	}, c)
}

// newValidate 初始化参数验证器
func newValidate() *validate.Validate {
	// 注册中文验证器
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
			t, er := utl.T(tag, fe.Field())
			if er != nil {
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
