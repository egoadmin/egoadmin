package cdn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/gin-gonic/gin"
)

var imageResponseHeaders = map[string]struct{}{
	"Content-Type":           {},
	"Content-Length":         {},
	"Cache-Control":          {},
	"ETag":                   {},
	"Last-Modified":          {},
	"Expires":                {},
	"Vary":                   {},
	"Accept-Ranges":          {},
	"Content-Range":          {},
	"X-Content-Type-Options": {},
}

func (c *Component) ImageHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		referenceID, err := c.parseReferenceID(ctx.Param("referenceId"))
		if err != nil {
			writeError(ctx, err)
			return
		}
		processPath, err := c.cleanProcessPath(ctx.Param("processPath"))
		if err != nil {
			writeError(ctx, err)
			return
		}
		if len(ctx.Request.URL.RawQuery) > c.config.MaxQueryBytes {
			writeError(ctx, ErrInvalidProcessPath)
			return
		}
		now := time.Now()
		signErr := c.verifySignature(ctx, imageSignatureMaterial(ctx.Request.URL.Path), now)
		if signErr != nil && !errors.Is(signErr, ErrSignatureRequired) {
			writeError(ctx, signErr)
			return
		}
		signed := signErr == nil
		if !signed && !c.config.PublicImage {
			writeError(ctx, ErrSignatureRequired)
			return
		}
		object, err := c.upload.GetDownloadReference(ctx.Request.Context(), referenceID)
		if err != nil {
			writeError(ctx, err)
			return
		}
		allowTemporary := signed || c.config.AllowTemporaryImage
		if err = ensureDownloadable(object, now, allowTemporary); err != nil {
			writeError(ctx, err)
			return
		}
		if object.ReferenceStatus == upload.ReferenceStatusTemporary && !signed && !c.config.AllowTemporaryImage {
			writeError(ctx, ErrSignatureRequired)
			return
		}
		if err = c.proxyImage(ctx, object, processPath); err != nil {
			writeError(ctx, err)
		}
	}
}

func (c *Component) cleanProcessPath(raw string) (string, error) {
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" {
		return "", nil
	}
	if len(raw) > c.config.MaxProcessPathBytes {
		return "", ErrInvalidProcessPath
	}
	for _, segment := range strings.Split(raw, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", ErrInvalidProcessPath
		}
		for _, r := range segment {
			if unicode.IsControl(r) {
				return "", ErrInvalidProcessPath
			}
		}
	}
	return raw, nil
}

func (c *Component) proxyImage(ctx *gin.Context, object *upload.DownloadObject, processPath string) error {
	if c.imageProcessor.URL == "" {
		return ErrImageProcessorMissing
	}
	processorPath := buildProcessorPath(processPath, object.ObjectKey)
	signedPath := imageProcessorSignature(c.imageProcessor.Secret, processorPath) + "/" + processorPath
	target, err := url.JoinPath(c.imageProcessor.URL, signedPath)
	if err != nil {
		return fmt.Errorf("cdn: build image processor url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("cdn: create image processor request: %w", err)
	}
	for _, header := range []string{"Accept", "Accept-Encoding", "If-None-Match", "If-Modified-Since", "Range", "User-Agent"} {
		if value := ctx.GetHeader(header); value != "" {
			req.Header.Set(header, value)
		}
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if errors.Is(ctx.Request.Context().Err(), context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("cdn: proxy image processor: %w", err)
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if _, ok := imageResponseHeaders[http.CanonicalHeaderKey(key)]; !ok {
			continue
		}
		for _, value := range values {
			ctx.Writer.Header().Add(key, value)
		}
	}
	ctx.Writer.Header().Set("X-Content-Type-Options", "nosniff")
	ctx.Status(resp.StatusCode)
	_, copyErr := io.Copy(ctx.Writer, resp.Body)
	return copyErr
}

func buildProcessorPath(processPath string, objectKey string) string {
	objectKey = strings.Trim(objectKey, "/")
	processPath = strings.Trim(processPath, "/")
	if processPath == "" {
		return objectKey
	}
	return path.Join(processPath, objectKey)
}

func imageSignatureMaterial(urlPath string) string {
	return "/" + strings.TrimLeft(urlPath, "/")
}
