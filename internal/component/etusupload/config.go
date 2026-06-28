package etusupload

import (
	"time"

	"github.com/gotomicro/ego/core/util/xtime"
)

// Config TUS上传组件配置
type Config struct {
	// 基础配置
	BasePath  string `toml:"basePath" json:"basePath" default:"/tus/upload"` // TUS基础路由路径
	MaxSize   int64  `toml:"maxSize" json:"maxSize" default:"1073741824"`    // 最大文件大小 (字节)，默认1GB
	DataDir   string `toml:"dataDir" json:"dataDir" default:"./data/tus"`    // 临时数据目录
	UploadDir string `toml:"uploadDir" json:"uploadDir" default:"./uploads"` // 上传文件保存目录

	// 验证配置
	EnableValidation     bool     `toml:"enableValidation" json:"enableValidation" default:"true"`         // 是否启用文件验证
	ValidateBeforeUpload bool     `toml:"validateBeforeUpload" json:"validateBeforeUpload" default:"true"` // 上传前验证
	ValidateAfterUpload  bool     `toml:"validateAfterUpload" json:"validateAfterUpload" default:"true"`   // 上传后验证
	AllowedExtensions    []string `toml:"allowedExtensions" json:"allowedExtensions"`                      // 允许的扩展名 (空=不限制)
	RejectedExtensions   []string `toml:"rejectedExtensions" json:"rejectedExtensions"`                    // 拒绝的扩展名
	AllowedMimeTypes     []string `toml:"allowedMimeTypes" json:"allowedMimeTypes"`                        // 允许的MIME类型 (空=不限制)
	RejectedMimeTypes    []string `toml:"rejectedMimeTypes" json:"rejectedMimeTypes"`                      // 拒绝的MIME类型

	// 日志和监控
	EnableAccessLog         bool          `toml:"enableAccessLog" json:"enableAccessLog" default:"false"`                // 是否启用访问日志
	EnableMetricInterceptor bool          `toml:"enableMetricInterceptor" json:"enableMetricInterceptor" default:"true"` // 是否启用指标
	SlowLogThreshold        time.Duration `toml:"slowLogThreshold" json:"slowLogThreshold" default:"1s"`                 // 慢日志阈值
	EnableHealthCheck       bool          `toml:"enableHealthCheck" json:"enableHealthCheck" default:"true"`             // 是否启用健康检查

	// CORS配置
	AllowAllOrigins bool     `toml:"allowAllOrigins" json:"allowAllOrigins" default:"true"` // 是否允许所有源
	AllowedOrigins  []string `toml:"allowedOrigins" json:"allowedOrigins"`                  // 允许的源列表
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BasePath:                "/tus/upload",
		MaxSize:                 1073741824, // 1GB
		DataDir:                 "./data/tus",
		UploadDir:               "./uploads",
		EnableValidation:        true,
		ValidateBeforeUpload:    true,
		ValidateAfterUpload:     true,
		EnableAccessLog:         false,
		EnableMetricInterceptor: true,
		SlowLogThreshold:        xtime.Duration("1s"),
		EnableHealthCheck:       true,
		AllowAllOrigins:         true,
		AllowedExtensions:       []string{},
		RejectedExtensions:      []string{},
		AllowedMimeTypes:        []string{},
		RejectedMimeTypes:       []string{},
		AllowedOrigins:          []string{},
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.BasePath == "" {
		c.BasePath = "/tus/upload"
	}
	if c.MaxSize <= 0 {
		c.MaxSize = 1073741824
	}
	if c.DataDir == "" {
		c.DataDir = "./data/tus"
	}
	if c.UploadDir == "" {
		c.UploadDir = "./uploads"
	}
	return nil
}
