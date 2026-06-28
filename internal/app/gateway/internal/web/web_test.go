package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/server/egin"
)

func TestBuildAppConfigJSEscapesHTML(t *testing.T) {
	js := string(buildAppConfigJS(config.WebConf{
		ApiBaseUrl:         "/api",
		FileBaseUrl:        "</script><script>alert(1)</script>",
		OfflineOnPageLeave: true,
	}))

	if !strings.HasPrefix(js, "window.__APP_CONFIG__=") {
		t.Fatalf("expected app config assignment, got %q", js)
	}
	if strings.Contains(js, "</script>") {
		t.Fatalf("expected HTML-sensitive chars to be escaped, got %q", js)
	}
	if !strings.Contains(js, `\u003c/script\u003e`) {
		t.Fatalf("expected escaped script closing tag, got %q", js)
	}
}

func TestServeEmbeddedWebSecurityHeadersAndAppConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	component := &egin.Component{Engine: router}

	StartWithFS(testWebFS(), config.WebConf{ApiBaseUrl: "/api"}, component)

	appConfig := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app-config.js", nil)
	router.ServeHTTP(appConfig, req)
	if appConfig.Code != http.StatusOK {
		t.Fatalf("expected app-config status 200, got %d", appConfig.Code)
	}
	if got := appConfig.Header().Get("Content-Security-Policy"); got != contentSecurityPolicy {
		t.Fatalf("unexpected CSP header %q", got)
	}
	if got := appConfig.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("unexpected app-config cache control %q", got)
	}
	if got := appConfig.Body.String(); !strings.Contains(got, `"apiBaseUrl":"/api"`) {
		t.Fatalf("unexpected app-config body %q", got)
	}

	index := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	router.ServeHTTP(index, req)
	if index.Code != http.StatusOK {
		t.Fatalf("expected SPA fallback status 200, got %d", index.Code)
	}
	if got := index.Header().Get("Content-Security-Policy"); got != contentSecurityPolicy {
		t.Fatalf("unexpected fallback CSP header %q", got)
	}
	if strings.Contains(index.Body.String(), "window.__APP_CONFIG__") {
		t.Fatalf("index should not contain inline runtime config: %q", index.Body.String())
	}

	api := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/not-found", nil)
	router.ServeHTTP(api, req)
	if api.Code != http.StatusNotFound {
		t.Fatalf("expected /api path to stay 404, got %d", api.Code)
	}
}

func testWebFS() fs.FS {
	return fstest.MapFS{
		"web/dist/index.html": {
			Data: []byte(`<!doctype html><html><head><script src="/app-config.js"></script></head><body><div id="app"></div></body></html>`),
		},
		"web/dist/assets/app.js": {
			Data: []byte(`console.log("ok")`),
		},
	}
}
