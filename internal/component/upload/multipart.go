package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/eerrors"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
	"github.com/samber/lo"
)

type MultipartCommand struct {
	Profile      string
	OwnerUserID  uint64
	OriginalName string
	ContentType  string
	SHA256       string
	Size         int64
	Reader       io.Reader
}

type MultipartInfo struct {
	Name        string `json:"name"`
	Size        string `json:"size"`
	Profile     string `json:"profile"`
	ContentType string `json:"contentType"`
	SHA256      string `json:"sha256"`
}

type MultipartResp struct {
	Filename    string `json:"filename"`
	Size        string `json:"size"`
	Originame   string `json:"originame"`
	FileID      string `json:"fileId,omitempty"`
	ReferenceID string `json:"referenceId"`
	Profile     string `json:"profile"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt"`
	URL         string `json:"url,omitempty"`
}

type AuthContext struct {
	UserID uint64
}

type MultipartOptions struct {
	MaxDescSize     int64
	BeforeHandle    func(*gin.Context) (*AuthContext, error)
	BeforeTusHandle func(context.Context, http.Header) (*AuthContext, error)
}

func Start(cc *egin.Component, component *Component, opts MultipartOptions) {
	if opts.MaxDescSize <= 0 {
		opts.MaxDescSize = 4 * 1024 * 1024
	}
	cc.POST(component.config.MultipartPath, component.MultipartHandler(opts))
}

func (c *Component) MultipartHandler(opts MultipartOptions) gin.HandlerFunc {
	if opts.MaxDescSize <= 0 {
		opts.MaxDescSize = 4 * 1024 * 1024
	}
	return func(ctx *gin.Context) {
		var (
			err   error
			auth  *AuthContext
			infos []MultipartInfo
			resp  []MultipartResp
		)
		defer func() {
			if err != nil {
				if se := new(eerrors.EgoError); errors.As(err, &se) {
					ctx.JSON(http.StatusOK, se)
					return
				}
				elog.Error("文件上传失败", elog.FieldErr(err))
				ctx.JSON(http.StatusOK, platformi18n.ErrorFailed(requestContext(ctx), "FileUploadFailed", nil))
				return
			}
			ctx.JSON(http.StatusOK, gin.H{"files": resp})
		}()
		if opts.BeforeHandle != nil {
			auth, err = opts.BeforeHandle(ctx)
			if err != nil {
				return
			}
		}
		reader, er := ctx.Request.MultipartReader()
		if er != nil {
			err = platformi18n.ErrorFailed(requestContext(ctx), "ReadFileInfoFailed", nil)
			return
		}
		infos, err = readMultipartInfos(requestContext(ctx), reader, opts.MaxDescSize)
		if err != nil {
			return
		}
		for {
			part, er := reader.NextPart()
			if er == io.EOF {
				break
			}
			if er != nil {
				err = platformi18n.ErrorFailed(requestContext(ctx), "FileUploadFailed", nil)
				return
			}
			if part.FormName() != "file" {
				err = platformi18n.ErrorFailed(requestContext(ctx), "UploadedFileMissing", nil)
				return
			}
			filename := part.FileName()
			if filename == "" {
				err = platformi18n.ErrorFailed(requestContext(ctx), "FileNameRequired", nil)
				return
			}
			info, ok := lo.Find(infos, func(item MultipartInfo) bool {
				return item.Name == filename
			})
			if !ok {
				err = platformi18n.ErrorFailed(requestContext(ctx), "FileDescriptionMissing", nil)
				return
			}
			fileSize, er := strconv.ParseInt(info.Size, 10, 64)
			if er != nil {
				err = platformi18n.ErrorFailed(requestContext(ctx), "FileUploadFailed", nil)
				return
			}
			userID := uint64(0)
			if auth != nil {
				userID = auth.UserID
			}
			result, er := c.UploadMultipart(ctx.Request.Context(), MultipartCommand{
				Profile:      info.Profile,
				OwnerUserID:  userID,
				OriginalName: filename,
				ContentType:  firstNonEmpty(info.ContentType, part.Header.Get("Content-Type")),
				SHA256:       info.SHA256,
				Size:         fileSize,
				Reader:       part,
			})
			if er != nil {
				err = er
				return
			}
			resp = append(resp, MultipartResp{
				Filename:    filename,
				Size:        strconv.FormatInt(fileSize, 10),
				Originame:   filename,
				FileID:      c.encodePublicID(publicFileIDPrefix, result.FileID),
				ReferenceID: c.encodePublicID(publicReferenceIDPrefix, result.ReferenceID),
				Profile:     result.Profile,
				Status:      ReferenceStatusTemporary,
				ExpiresAt:   result.ExpiresAt.Format(time.RFC3339),
				URL:         result.URL,
			})
		}
	}
}

func (c *Component) UploadMultipart(ctx context.Context, cmd MultipartCommand) (*UploadResult, error) {
	profileName, profile, err := c.config.RequireProfile(cmd.Profile)
	if err != nil {
		return nil, err
	}
	if err := validateMultipart(profile, cmd); err != nil {
		return nil, err
	}
	fileID, err := c.flake.Get()
	if err != nil {
		return nil, fmt.Errorf("upload: generate file id: %w", err)
	}
	objectKey, err := c.objectKey(profileName, fileID, path.Ext(cmd.OriginalName))
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(profile.TTL)
	created, err := c.store.CreateMultipart(ctx, CreateMultipartCommand{
		FileID:       fileID,
		Bucket:       c.bucket,
		ObjectKey:    objectKey,
		OriginalName: cmd.OriginalName,
		ContentType:  cmd.ContentType,
		SHA256:       cmd.SHA256,
		Size:         cmd.Size,
		CreatedBy:    cmd.OwnerUserID,
		OwnerUserID:  cmd.OwnerUserID,
		Profile:      profileName,
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return nil, err
	}
	if err = c.object.Put(ctx, objectKey, cmd.Reader, cmd.Size, PutOptions{ContentType: cmd.ContentType}); err != nil {
		_ = c.store.MarkUploadFailed(context.WithoutCancel(ctx), created.SessionID)
		return nil, err
	}
	if err = c.store.MarkObjectAvailable(ctx, created.FileID); err != nil {
		return nil, err
	}
	if err = c.store.MarkSessionFinished(ctx, created.SessionID); err != nil {
		return nil, err
	}
	accessURL, err := c.mustAccessURL(profileName, created.ReferenceID)
	if err != nil {
		return nil, err
	}
	return &UploadResult{
		FileID:      created.FileID,
		ReferenceID: created.ReferenceID,
		Profile:     profileName,
		ObjectKey:   objectKey,
		URL:         accessURL,
		ExpiresAt:   created.ExpiresAt,
	}, nil
}

func readMultipartInfos(ctx context.Context, reader *multipart.Reader, maxDescSize int64) ([]MultipartInfo, error) {
	part, err := reader.NextPart()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, platformi18n.ErrorFailed(ctx, "FileUploadFailed", nil)
	}
	if part.FormName() != "json" {
		return nil, platformi18n.ErrorFailed(ctx, "FileDescriptionMissing", nil)
	}
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, part, maxDescSize+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, platformi18n.ErrorFailed(ctx, "FileUploadFailed", nil)
	}
	if n > maxDescSize {
		return nil, platformi18n.ErrorFailed(ctx, "UploadDescriptionTooLarge", nil)
	}
	infos := make([]MultipartInfo, 0)
	if err = json.Unmarshal(buf.Bytes(), &infos); err != nil {
		return nil, platformi18n.ErrorFailed(ctx, "FileUploadFailed", nil)
	}
	return infos, nil
}

func (c *Component) objectKey(profile string, fileID uint64, ext string) (string, error) {
	ext = strings.ToLower(ext)
	publicFileID, err := c.publicID(publicFileIDPrefix, fileID)
	if err != nil {
		return "", err
	}
	now := time.Now()
	return path.Join(
		"files",
		profile,
		strconv.Itoa(now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		fmt.Sprintf("%02d", now.Day()),
		fmt.Sprintf("%s%s", publicFileID, ext),
	), nil
}

func validateMultipart(profile ProfileConfig, cmd MultipartCommand) error {
	if cmd.Reader == nil {
		return fmt.Errorf("upload: file reader is required")
	}
	if profile.TusRequired {
		return fmt.Errorf("upload: profile requires tus upload")
	}
	return validateFileAttributes(profile, cmd.OriginalName, cmd.ContentType, cmd.Size)
}

func validateFileAttributes(profile ProfileConfig, originalName string, contentType string, size int64) error {
	if originalName == "" {
		return fmt.Errorf("upload: file name is required")
	}
	if size < 0 {
		return fmt.Errorf("upload: file size is invalid")
	}
	if profile.MaxSize > 0 && size > profile.MaxSize {
		return fmt.Errorf("upload: file too large")
	}
	extension := strings.TrimPrefix(strings.ToLower(path.Ext(originalName)), ".")
	if len(profile.AllowedExtensions) > 0 && !contains(profile.AllowedExtensions, extension) {
		return fmt.Errorf("upload: unsupported file extension")
	}
	normalizedContentType := strings.ToLower(strings.TrimSpace(contentType))
	if len(profile.AllowedMimeTypes) > 0 && normalizedContentType != "" && !contains(profile.AllowedMimeTypes, normalizedContentType) {
		return fmt.Errorf("upload: unsupported content type")
	}
	return nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// 预留:拼接对象访问 URL，供后续直连 objectKey 访问场景使用。
//
//nolint:unused // 预留:对象访问 URL 拼接
func joinURL(baseURL, objectKey string) string {
	if baseURL == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(objectKey, "/")
}

// 预留:按 referenceID 生成访问 URL，供后续文件访问入口使用。
//
//nolint:unused // 预留:按引用 ID 生成访问 URL
func (c *Component) accessURL(profile string, referenceID uint64) string {
	if referenceID == 0 {
		return ""
	}
	publicReferenceID := c.encodePublicID(publicReferenceIDPrefix, referenceID)
	if publicReferenceID == "" {
		return ""
	}
	urlPath := c.fileURLPath
	if c.isImageProfile(profile) {
		urlPath = c.imageURLPath
	}
	urlPath = strings.TrimRight(urlPath, "/")
	if urlPath == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", urlPath, publicReferenceID)
}

func (c *Component) mustAccessURL(profile string, referenceID uint64) (string, error) {
	publicReferenceID, err := c.publicID(publicReferenceIDPrefix, referenceID)
	if err != nil {
		return "", err
	}
	urlPath := c.fileURLPath
	if c.isImageProfile(profile) {
		urlPath = c.imageURLPath
	}
	urlPath = strings.TrimRight(urlPath, "/")
	if urlPath == "" {
		return "", nil
	}
	return fmt.Sprintf("%s/%s", urlPath, publicReferenceID), nil
}

func (c *Component) isImageProfile(profile string) bool {
	profile = strings.ToLower(strings.TrimSpace(profile))
	if profile == "image" || profile == "avatar" {
		return true
	}
	cfg, ok := c.config.Profiles[profile]
	if !ok {
		return false
	}
	for _, contentType := range cfg.AllowedMimeTypes {
		if strings.HasPrefix(strings.ToLower(contentType), "image/") {
			return true
		}
	}
	for _, ext := range cfg.AllowedExtensions {
		switch strings.TrimPrefix(strings.ToLower(ext), ".") {
		case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg":
			return true
		}
	}
	return false
}

func requestContext(c *gin.Context) context.Context {
	return platformi18n.WithAcceptLanguage(c.Request.Context(), c.GetHeader(platformi18n.HeaderAcceptLanguage))
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if item != "" {
			return item
		}
	}
	return ""
}
