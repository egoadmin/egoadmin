package config

import (
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/gotomicro/ego/core/elog"
)

// Config 是业务侧使用的强类型配置。字段由 Manager 从渲染后的完整 TOML 解析填充，
// 并发安全（读写受 mu 保护），支持热重载后原地更新。
type Config struct {
	mu sync.RWMutex
	// service 服务标识
	service Service
	// manager 配置编排器，提供临时文件路径与生命周期管理
	manager *Manager
	// 应用配置
	app ServiceConf
	// 停机配置
	shutdown ShutdownConf
	// 数据库迁移配置
	dbMigration DBMigrationConf
	// 用户配置
	user UserConf
	// ID 生成服务配置
	idgen IDGenConf
	// 前端运行时配置
	web WebConf
}

// rootConfig 是渲染后完整 TOML 的顶层结构，仅用于一次性解析到各 typed 字段。
type rootConfig struct {
	App struct {
		Service     ServiceConf     `toml:"service"`
		Shutdown    ShutdownConf    `toml:"shutdown"`
		DBMigration DBMigrationConf `toml:"dbMigration"`
		User        UserConf        `toml:"user"`
		IDGen       IDGenConf       `toml:"idgen"`
		Web         WebConf         `toml:"web"`
	} `toml:"app"`
}

// bindTOML 将渲染后的完整 TOML 解析并绑定到各 typed 字段。
// 由 Manager 在初始化和热重载时调用。
func (c *Config) bindTOML(rendered string) {
	var root rootConfig
	if _, err := toml.Decode(rendered, &root); err != nil {
		elog.Error("config bind toml", elog.FieldErr(err))
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.app = root.App.Service
	c.shutdown = root.App.Shutdown
	c.dbMigration = root.App.DBMigration
	c.user = root.App.User
	c.idgen = root.App.IDGen
	c.web = root.App.Web
}

// App 应用配置.
func (c *Config) App() ServiceConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.app
}

// Shutdown 停机配置.
func (c *Config) Shutdown() ShutdownConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.shutdown
}

// DBMigration 数据库迁移配置.
func (c *Config) DBMigration() DBMigrationConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.dbMigration
}

// Web 前端运行时配置.
func (c *Config) Web() WebConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.web
}

// User 用户配置.
func (c *Config) User() UserConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.user
}

// IDGen returns idgen service configuration.
func (c *Config) IDGen() IDGenConf {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.idgen
}

// ServiceConf 应用服务配置.
type ServiceConf struct {
	Name                        string `toml:"name"`
	PlatformName                string `toml:"platformName"`
	AutoMigrate                 bool   `toml:"autoMigrate"`
	WebPath                     string `toml:"webPath"`                     // 前端代码路径
	BucketName                  string `toml:"bucketName"`                  // 存储桶名称
	SkipPermissionContractCheck bool   `toml:"skipPermissionContractCheck"` // 是否跳过前端权限契约边界校验
}

// ShutdownConf 控制服务停机 drain 与资源关闭行为.
type ShutdownConf struct {
	StopTimeout  string `toml:"stopTimeout"`  // EGO graceful stop 总超时
	DrainTimeout string `toml:"drainTimeout"` // readiness 下线后等待服务发现/负载均衡摘流的时间
	CloseTimeout string `toml:"closeTimeout"` // 非 server 资源关闭单项超时
}

// DBMigrationConf 数据库版本迁移配置.
type DBMigrationConf struct {
	Enabled bool   `toml:"enabled"`
	Driver  string `toml:"driver"`
	URL     string `toml:"url"`
	Dir     string `toml:"dir"`
	Bin     string `toml:"bin"`
}

// UserConf 用户管理配置.
type UserConf struct {
	RootPassword                    string `toml:"rootPassword"`                    // root用户密码
	AdminPassword                   string `toml:"adminPassword"`                   // admin用户密码
	JwtSignKey                      string `toml:"jwtSignKey"`                      // jwt签名密码
	JwtExpire                       int64  `toml:"jwtExpire"`                       // jwt过期时间，单位秒
	RefreshTokenExpire              int64  `toml:"refreshTokenExpire"`              // refresh token过期时间，单位秒，未配置时使用认证组件默认值
	UseCaptcha                      bool   `toml:"useCaptcha"`                      // 是否使用验证码
	MultiLoginEnabled               bool   `toml:"multiLoginEnabled"`               // 是否开启多端登录
	MaxLoginClient                  int32  `toml:"maxLoginClient"`                  // 客户端上线 multiLoginEnabled 未开启该配置不生效
	HeartbeatOfflineEnabled         bool   `toml:"heartbeatOfflineEnabled"`         // 是否启用心跳超时离线标记
	HeartbeatOfflineSeconds         int64  `toml:"heartbeatOfflineSeconds"`         // 心跳超时离线秒数
	RevokeSessionOnHeartbeatOffline bool   `toml:"revokeSessionOnHeartbeatOffline"` // 心跳超时离线时是否撤销登录会话
}

// IDGenConf controls idgen service maintenance behavior.
type IDGenConf struct {
	MachineLeaseCleanupRetention string `toml:"machineLeaseCleanupRetention"`
	MachineLeaseCleanupLimit     int    `toml:"machineLeaseCleanupLimit"`
}

// WebConf 前端运行时配置，由 gateway 通过 /app-config.js 输出到 window.__APP_CONFIG__。
type WebConf struct {
	ApiBaseUrl         string `toml:"apiBaseUrl" json:"apiBaseUrl"`                 // API 前缀，空则前端默认使用 /api
	FileBaseUrl        string `toml:"fileBaseUrl" json:"fileBaseUrl"`               // 文件/图片访问地址前缀
	OfflineOnPageLeave bool   `toml:"offlineOnPageLeave" json:"offlineOnPageLeave"` // 是否在最后一个标签页离开时主动登出
}

// SetSkipPermissionContractCheckForTest 仅限测试时使用，用于设置 SkipPermissionContractCheck。
func (c *Config) SetSkipPermissionContractCheckForTest(skip bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.app.SkipPermissionContractCheck = skip
}

// SetUserForTest 仅限测试时使用，用于设置 UserConf。
func (c *Config) SetUserForTest(user UserConf) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.user = user
}

// RenderedPath 返回渲染后的完整配置临时文件路径（含环境变量覆盖）。
// 用于 ego.New(ego.WithArguments(["--config", path]))，确保 ego 内部的
// loadConfig / initTracer 等读到的是合并并覆盖后的配置。
func (c *Config) RenderedPath() string {
	if c.manager == nil {
		return ""
	}
	return c.manager.tempPath
}

// Close 删除临时配置文件。实现 io.Closer，可注册到 shutdown.Manager。
func (c *Config) Close() error {
	if c.manager != nil {
		c.manager.removeRendered()
	}
	return nil
}
