package web

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/server/egin"
)

// indexHtml 主页html代码
var indexHtml []byte

// Start 注册静态服务
func Start(conf *config.Config, c *egin.Component) {
	// c.POST("/other/api/v1/upload")
	// c.GET("/other/api/v1/download")

	indexHtml = make([]byte, 0)

	appConf := conf.App()

	f, err := os.Open(path.Join(appConf.WebPath, "index.html"))
	if err != nil {
		indexHtml = make([]byte, 0)

		return
	}
	defer f.Close()

	indexHtml, err = io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// index.html
	c.GET("/", func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "text/html", indexHtml)
	})

	// 遍历第一级目录并设置静态服务
	dirs, _ := os.ReadDir(appConf.WebPath)
	for _, dir := range dirs {
		if dir.IsDir() {
			c.Static("/"+dir.Name(), path.Join(appConf.WebPath, dir.Name()))
		} else {
			c.StaticFile("/"+dir.Name(), path.Join(appConf.WebPath, dir.Name()))
		}
	}

	// admin web
	c.GET("/admin", func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "text/html", indexHtml)
	})

	// 遍历第一级目录并设置静态服务
	for _, dir := range dirs {
		if dir.IsDir() {
			c.Static("/admin/"+dir.Name(), path.Join(appConf.WebPath, dir.Name()))
		} else {
			c.StaticFile("/admin/"+dir.Name(), path.Join(appConf.WebPath, dir.Name()))
		}
	}
}

const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: http: https:; font-src 'self' data:; connect-src 'self' blob: http: https: ws: wss:; media-src 'self' blob: data:; worker-src 'self' blob:; form-action 'self'"

// StartWithFS 使用嵌入的 fs.FS 注册 SPA 静态服务，并通过 /app-config.js 输出运行时配置。
func StartWithFS(fsys fs.FS, webConf config.WebConf, c *egin.Component) {
	subFS, err := fs.Sub(fsys, "web/dist")
	if err != nil {
		return
	}

	rawIndex, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		return
	}

	appConfigJS := buildAppConfigJS(webConf)
	httpFS := http.FS(subFS)
	fileServer := http.FileServer(httpFS)

	c.GET("/app-config.js", func(ctx *gin.Context) {
		setSecurityHeaders(ctx)
		ctx.Header("Cache-Control", "no-store")
		ctx.Data(http.StatusOK, "application/javascript; charset=utf-8", appConfigJS)
	})

	c.NoRoute(func(ctx *gin.Context) {
		urlPath := ctx.Request.URL.Path
		if strings.HasPrefix(urlPath, "/api") {
			ctx.Status(http.StatusNotFound)
			return
		}
		if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodHead {
			ctx.Status(http.StatusNotFound)
			return
		}

		setSecurityHeaders(ctx)
		if serveStaticFile(ctx, httpFS, fileServer, strings.TrimPrefix(urlPath, "/")) {
			return
		}

		if strings.HasPrefix(urlPath, "/admin/") {
			adminPath := strings.TrimPrefix(urlPath, "/admin/")
			if serveStaticFile(ctx, httpFS, fileServer, adminPath) {
				return
			}
		}

		ctx.Header("Cache-Control", "no-store")
		ctx.Data(http.StatusOK, "text/html; charset=utf-8", rawIndex)
	})
}

func serveStaticFile(ctx *gin.Context, httpFS http.FileSystem, fileServer http.Handler, name string) bool {
	f, err := httpFS.Open(name)
	if err != nil {
		return false
	}

	fi, serr := f.Stat()
	_ = f.Close()
	if serr != nil || fi.IsDir() {
		return false
	}

	if strings.HasPrefix(ctx.Request.URL.Path, "/admin/") {
		req := ctx.Request.Clone(ctx.Request.Context())
		req.URL.Path = "/" + name
		fileServer.ServeHTTP(ctx.Writer, req)
		return true
	}

	fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	return true
}

func buildAppConfigJS(conf config.WebConf) []byte {
	cfgJSON, err := json.Marshal(conf)
	if err != nil {
		return []byte("window.__APP_CONFIG__={};\n")
	}

	return []byte("window.__APP_CONFIG__=" + string(cfgJSON) + ";\n")
}

func setSecurityHeaders(ctx *gin.Context) {
	ctx.Header("Content-Security-Policy", contentSecurityPolicy)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("X-Frame-Options", "DENY")
	ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}
