package service

import (
	"time"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	cache "github.com/egoadmin/egoadmin/internal/app/user/internal/cache"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
	"github.com/egoadmin/egoadmin/internal/platform/cache/local"
	"github.com/egoadmin/egoadmin/internal/platform/cache/redis"
	"github.com/egoadmin/egoadmin/internal/platform/captcha"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	idgenplatform "github.com/egoadmin/egoadmin/internal/platform/idgen"
	"github.com/egoadmin/egoadmin/internal/platform/objectstore"
	"github.com/egoadmin/elib/pkg/middleware/perm"
	"github.com/egoadmin/elib/pkg/middleware/validate"
	"github.com/egoadmin/elib/pkg/util/xfile"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/google/wire"
)

const accessTokenBeforeExpires int64 = 3600

var ProviderSet = wire.NewSet(
	NewLogService,    // 日志服务
	NewUserService,   // 用户服务
	NewRoleService,   // 用户角色服务
	NewDeptService,   // 组织服务
	NewConfigService, // 系统设置服务
	NewAuthSession,
	NewLoginCrypto,
	wire.Struct(new(Options), "*"),
	wire.Bind(new(authsession.Interface), new(*authsession.Component)),
	wire.Bind(new(LoginCryptoInterface), new(*logincrypto.Component)),
	redis.CoreProviderSet,
	cache.ProviderSet,
	mysql.CoreProviderSet,
	store.ProviderSet,
	localcache.ProviderSet,
	jetcache.ProviderSet,
	idgenplatform.ProviderSet,
	objectstore.ProviderSet,
)

// Options wireservice
type Options struct {
	Conf   *config.Config
	Idgen  xflake.Geter
	Logger auditlog.Loger

	Log           store.LogInterface
	Dept          store.DeptInterface
	User          store.UserInterface
	Role          store.RoleInterface
	Config        store.ConfigInterface
	AuthCryptoKey store.AuthCryptoKeyInterface

	Validator   *validate.Validate
	Casbin      *perm.Casbin
	JetCache    *jetcache.Component
	Captcha     captcha.ICaptcha
	Auth        authsession.Interface
	IDGen       *idgen.Component
	LoginCrypto LoginCryptoInterface
	ConfigCache *localcache.ConfigCache
	S3          *xfile.S3
	UserUseCase *application.UserUseCase
	RoleUseCase *application.RoleUseCase
	DeptUseCase *application.DeptUseCase

	UserRedis cache.UserInterface
	Mysql     mysql.MysqlInterface
}

func NewAuthSession(conf *config.Config, redis *eredis.Component, cache *jetcache.Component, user store.UserInterface) *authsession.Component {
	uconf := conf.User()
	accessTTL := time.Second * time.Duration(uconf.JwtExpire-accessTokenBeforeExpires)
	if accessTTL <= 0 {
		accessTTL = time.Hour
	}
	refreshTTL := authSessionRefreshTokenTTL(uconf)

	return authsession.DefaultContainer().Build(
		authsession.WithEredis(redis),
		authsession.WithJetCache(cache),
		authsession.WithConfig(&authsession.Config{
			Name:                   "egoadmin",
			KeyPrefix:              defaults.RedisKeyPrefix,
			JWTSignKey:             uconf.JwtSignKey,
			AccessTokenTTL:         accessTTL,
			AccessTokenDisplaySkew: 30 * time.Minute,
			RefreshTokenTTL:        refreshTTL,
			RevokedRecordTTL:       24 * time.Hour,
			TouchInterval:          time.Minute,
			MultiLoginEnabled:      uconf.MultiLoginEnabled,
			MaxSessions:            int(uconf.MaxLoginClient),
			SameDeviceStrategy:     authsession.SameDeviceReplace,
			OverflowStrategy:       authsession.OverflowRevokeOldest,
		}),
		authsession.WithContextValidator(newAuthContextValidator(newJetAuthSnapshotCache(cache.Cache()), user)),
	)
}

func authSessionRefreshTokenTTL(uconf config.UserConf) time.Duration {
	if uconf.RefreshTokenExpire <= 0 {
		return authsession.DefaultConfig().RefreshTokenTTL
	}
	return time.Second * time.Duration(uconf.RefreshTokenExpire)
}
