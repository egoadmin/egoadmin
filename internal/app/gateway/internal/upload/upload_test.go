package upload

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWithS3UploadLocalizesMultipartReaderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	opt := WithS3Upload(nil, nil)
	router.POST(opt.RelativePath, opt.HandleFunc)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "Failed to read file information") {
		t.Fatalf("body = %q, want localized upload error", body)
	}
}
