package cdn

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/gin-gonic/gin"
)

const (
	dispositionInline     = "inline"
	dispositionAttachment = "attachment"
)

func (c *Component) FileHandler(opts Options) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		referenceID, err := c.parseReferenceID(ctx.Param("referenceId"))
		if err != nil {
			writeError(ctx, err)
			return
		}
		if err = validateDisplay(ctx.Query("display")); err != nil {
			writeError(ctx, err)
			return
		}
		now := time.Now()
		signErr := c.verifySignature(ctx, fileSignatureMaterial(ctx.Request.URL), now)
		if signErr != nil {
			if !errors.Is(signErr, ErrSignatureRequired) {
				writeError(ctx, signErr)
				return
			}
			if opts.BeforeFileHandle == nil {
				writeError(ctx, signErr)
				return
			}
			auth, authErr := opts.BeforeFileHandle(ctx)
			if authErr != nil {
				writeError(ctx, authErr)
				return
			}
			if auth == nil || auth.UserID == 0 {
				writeError(ctx, ErrSignatureRequired)
				return
			}
			c.serveFileForOwner(ctx, referenceID, auth.UserID, now)
			return
		}
		c.serveFile(ctx, referenceID, now)
	}
}

func (c *Component) serveFileForOwner(ctx *gin.Context, referenceID uint64, ownerUserID uint64, now time.Time) {
	object, err := c.upload.GetDownloadReferenceForOwner(ctx.Request.Context(), referenceID, ownerUserID)
	if err != nil {
		writeError(ctx, err)
		return
	}
	c.serveFileObject(ctx, object, now, true)
}

func (c *Component) serveFile(ctx *gin.Context, referenceID uint64, now time.Time) {
	object, err := c.upload.GetDownloadReference(ctx.Request.Context(), referenceID)
	if err != nil {
		writeError(ctx, err)
		return
	}
	c.serveFileObject(ctx, object, now, true)
}

func (c *Component) serveFileObject(ctx *gin.Context, object *upload.DownloadObject, now time.Time, allowTemporary bool) {
	if err := ensureDownloadable(object, now, allowTemporary); err != nil {
		writeError(ctx, err)
		return
	}
	reader, err := c.object.Get(ctx.Request.Context(), object.ObjectKey)
	if err != nil {
		writeError(ctx, err)
		return
	}
	defer reader.Close()

	stat, err := reader.Stat()
	if err != nil {
		writeError(ctx, err)
		return
	}
	fileName := safeDownloadName(object.OriginalName, object.ObjectKey)
	disposition := dispositionAttachment
	if ctx.Query("display") == dispositionInline {
		disposition = dispositionInline
	}
	contentType := firstNonEmpty(object.ContentType, stat.ContentType, mime.TypeByExtension(path.Ext(fileName)), "application/octet-stream")
	modTime := time.Now()
	if object.AvailableAt != nil {
		modTime = *object.AvailableAt
	}
	ctx.Header("Content-Disposition", contentDisposition(disposition, fileName))
	ctx.Header("Content-Type", contentType)
	ctx.Header("Content-Length", strconv.FormatInt(stat.Size, 10))
	ctx.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(ctx.Writer, ctx.Request, fileName, modTime, reader)
}

// 预留:签名有效性快捷判断，供后续访问控制分支使用。
//
//nolint:unused // 预留:签名有效性判断
func (c *Component) hasValidSignature(ctx *gin.Context, material string, now time.Time) bool {
	return c.verifySignature(ctx, material, now) == nil
}

func (c *Component) verifySignature(ctx *gin.Context, material string, now time.Time) error {
	return verifyAccessSignature(
		c.config.SignSecret,
		material,
		ctx.Query(queryExpires),
		ctx.Query(queryToken),
		now,
	)
}

func fileSignatureMaterial(u *url.URL) string {
	material := "/" + strings.TrimLeft(u.Path, "/")
	if display := u.Query().Get("display"); display != "" {
		values := url.Values{}
		values.Set("display", display)
		material += "?" + values.Encode()
	}
	return material
}

func validateDisplay(display string) error {
	if display == "" || display == dispositionAttachment || display == dispositionInline {
		return nil
	}
	return ErrInvalidDisplay
}

func safeDownloadName(originalName string, objectKey string) string {
	name := strings.TrimSpace(path.Base(originalName))
	if name == "." || name == "/" || name == "" {
		name = path.Base(objectKey)
	}
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, `"`, "")
	if name == "." || name == "/" || name == "" {
		return "download"
	}
	return name
}

func contentDisposition(disposition string, fileName string) string {
	escaped := strings.ReplaceAll(fileName, `\`, "")
	escaped = strings.ReplaceAll(escaped, `"`, "")
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, escaped, pathEscape(fileName))
}

func pathEscape(value string) string {
	replacer := strings.NewReplacer("+", "%20")
	return replacer.Replace(url.QueryEscape(value))
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}

func writeError(ctx *gin.Context, err error) {
	status := statusFromError(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	message := "request failed"
	switch {
	case errors.Is(err, ErrSignatureRequired):
		message = "signature or authorization required"
	case errors.Is(err, ErrSignatureInvalid):
		message = "signature invalid"
	case errors.Is(err, ErrSignatureExpired):
		message = "signature expired"
	case errors.Is(err, upload.ErrReferenceNotFound):
		message = "file reference not found"
	case errors.Is(err, upload.ErrReferenceForbidden):
		message = "file reference forbidden"
	case errors.Is(err, upload.ErrObjectNotFound):
		message = "object not found"
	case errors.Is(err, ErrReferenceGone):
		message = "file reference expired or released"
	case errors.Is(err, ErrObjectUnavailable):
		message = "object unavailable"
	case errors.Is(err, ErrInvalidReferenceID), errors.Is(err, ErrInvalidDisplay), errors.Is(err, ErrInvalidProcessPath):
		message = err.Error()
	}
	ctx.JSON(status, gin.H{"error": message})
}
