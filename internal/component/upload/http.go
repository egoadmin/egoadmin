package upload

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/eerrors"
	"github.com/gotomicro/ego/server/egin"
)

type InstantRequest struct {
	Profile     string `json:"profile"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
}

func RegisterRoutes(cc *egin.Component, component *Component, opts MultipartOptions) error {
	Start(cc, component, opts)
	cc.GET(component.config.MultipartPath+"/profiles", component.ProfilesHandler(opts))
	cc.POST(component.config.MultipartPath+"/instant", component.InstantHandler(opts))
	return RegisterTusRoutes(cc, component, opts)
}

func (c *Component) Profiles() []ProfileInfo {
	infos := make([]ProfileInfo, 0, len(c.config.Profiles))
	for name, profile := range c.config.Profiles {
		infos = append(infos, ProfileInfo{
			Name:              name,
			MaxSize:           profile.MaxSize,
			TTLSeconds:        int64(profile.TTL / time.Second),
			AllowedExtensions: append([]string(nil), profile.AllowedExtensions...),
			AllowedMimeTypes:  append([]string(nil), profile.AllowedMimeTypes...),
			TusRequired:       profile.TusRequired,
			MaxCount:          profile.MaxCount,
			InstantEnabled:    true,
		})
	}
	sort.Slice(infos, func(left, right int) bool {
		return infos[left].Name < infos[right].Name
	})
	return infos
}

func (c *Component) ProfilesHandler(opts MultipartOptions) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx := requestContext(ctx)
		if _, err := requireAuth(ctx, opts); err != nil {
			writeUploadError(ctx, reqCtx, err)
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"profiles": c.Profiles()})
	}
}

func (c *Component) InstantHandler(opts MultipartOptions) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx := requestContext(ctx)
		auth, err := requireAuth(ctx, opts)
		if err != nil {
			writeUploadError(ctx, reqCtx, err)
			return
		}
		var req InstantRequest
		if err = ctx.ShouldBindJSON(&req); err != nil {
			writeUploadError(ctx, reqCtx, platformi18n.ErrorFailed(reqCtx, "FileUploadFailed", nil))
			return
		}
		result, err := c.InstantUpload(ctx.Request.Context(), InstantCommand{
			Profile:      req.Profile,
			OwnerUserID:  auth.UserID,
			SHA256:       req.SHA256,
			Size:         req.Size,
			OriginalName: req.Filename,
			ContentType:  req.ContentType,
		})
		if err != nil {
			writeUploadError(ctx, reqCtx, err)
			return
		}
		ctx.JSON(http.StatusOK, result)
	}
}

func requireAuth(ctx *gin.Context, opts MultipartOptions) (*AuthContext, error) {
	if opts.BeforeHandle == nil {
		return &AuthContext{}, nil
	}
	return opts.BeforeHandle(ctx)
}

func writeUploadError(ctx *gin.Context, reqCtx context.Context, err error) {
	if se := new(eerrors.EgoError); err != nil && errors.As(err, &se) {
		ctx.JSON(http.StatusOK, se)
		return
	}
	ctx.JSON(http.StatusOK, platformi18n.ErrorFailed(reqCtx, "FileUploadFailed", nil))
}
