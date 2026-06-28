package etusupload

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// NewCorsMiddleware 创建TUS上传所需的CORS中间件
func NewCorsMiddleware(config *Config) gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowAllOrigins: config.AllowAllOrigins,
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
		},
		AllowHeaders: []string{
			"Authorization",
			"X-Requested-With",
			"X-Request-ID",
			"X-HTTP-Method-Override",
			"X-User-ID",
			"Upload-Length",
			"Upload-Offset",
			"Tus-Resumable",
			"Upload-Metadata",
			"Upload-Defer-Length",
			"Upload-Concat",
			"User-Agent",
			"Referrer",
			"Origin",
			"Content-Type",
			"Content-Length",
		},
		ExposeHeaders: []string{
			"Upload-Offset",
			"Location",
			"Upload-Length",
			"Tus-Version",
			"Tus-Resumable",
			"Tus-Max-Size",
			"Tus-Extension",
			"Upload-Metadata",
			"Upload-Defer-Length",
			"Upload-Concat",
		},
	}

	if !config.AllowAllOrigins && len(config.AllowedOrigins) > 0 {
		corsConfig.AllowOrigins = config.AllowedOrigins
	}

	return cors.New(corsConfig)
}
