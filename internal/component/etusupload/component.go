package etusupload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/elog"
	"github.com/oklog/ulid/v2"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

const PackageName = "component.etusupload"

// Component TUS上传组件
type Component struct {
	name       string
	config     *Config
	logger     *elog.Component
	handler    *tusd.Handler
	store      tusd.DataStore
	validator  FileValidator
	hooks      []UploadHook
	shutdownCh chan struct{}
}

// UploadHook 上传生命周期钩子
type UploadHook interface {
	// OnBeforeUpload 上传前
	OnBeforeUpload(ctx context.Context, filename string) error

	// OnAfterUpload 上传完成
	OnAfterUpload(ctx context.Context, info *UploadInfo) error

	// OnValidationFailed 验证失败
	OnValidationFailed(ctx context.Context, filename string, err error) error
}

// UploadInfo 上传信息
type UploadInfo struct {
	FileID      string            // 生成的唯一文件ID
	FileName    string            // 原始文件名
	FilePath    string            // 文件保存路径
	FileSize    int64             // 文件大小
	UploadID    string            // TUS上传ID
	Metadata    map[string]string // 自定义元数据
	CompletedAt time.Time         // 完成时间
}

// newComponent 创建组件
func newComponent(name string, config *Config, logger *elog.Component) *Component {
	return &Component{
		name:       name,
		config:     config,
		logger:     logger,
		hooks:      make([]UploadHook, 0),
		shutdownCh: make(chan struct{}),
	}
}

// Build 构建组件（初始化TUS处理器）
func (c *Component) Build() *Component {
	if err := c.config.Validate(); err != nil {
		c.logger.Panic("validate config failed", elog.FieldErr(err))
	}

	// 创建必要的目录
	for _, dir := range []string{c.config.DataDir, c.config.UploadDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			c.logger.Panic("create directory failed",
				elog.String("dir", dir),
				elog.FieldErr(err))
		}
	}

	// 初始化TUS处理器
	if err := c.initTusHandler(); err != nil {
		c.logger.Panic("init tus handler failed", elog.FieldErr(err))
	}

	// 启用默认验证器
	if c.validator == nil && c.config.EnableValidation {
		c.validator = NewDefaultValidatorFromConfig(c.config)
	}

	// 启动完成事件处理
	go c.handleCompletedUploads()

	c.logger.Info("component initialized",
		elog.String("base_path", c.config.BasePath),
		elog.String("upload_dir", c.config.UploadDir))

	return c
}

// initTusHandler 初始化TUS处理器
func (c *Component) initTusHandler() error {
	store, composer, err := createTusStore(c.config.DataDir)
	if err != nil {
		return fmt.Errorf("create tus store failed: %w", err)
	}

	c.store = store

	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:                  c.config.BasePath,
		StoreComposer:             composer,
		NotifyCompleteUploads:     true,
		RespectForwardedHeaders:   true,
		MaxSize:                   c.config.MaxSize,
		PreUploadCreateCallback:   c.validatePreUpload,
		PreFinishResponseCallback: c.validatePostUpload,
	})
	if err != nil {
		return fmt.Errorf("create tus handler failed: %w", err)
	}

	c.handler = handler
	return nil
}

// validatePreUpload TUS的上传前验证回调
func (c *Component) validatePreUpload(hook tusd.HookEvent) (tusd.HTTPResponse, tusd.FileInfoChanges, error) {
	if c.validator == nil || !c.config.ValidateBeforeUpload {
		return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, nil
	}

	filename, ok := hook.Upload.MetaData["filename"]
	if !ok || filename == "" {
		return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, ErrMissingFilename()
	}

	metadata := make(map[string]string)
	for k, v := range hook.Upload.MetaData {
		metadata[k] = v
	}

	for _, item := range c.hooks {
		if err := item.OnBeforeUpload(hook.Context, filename); err != nil {
			return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, err
		}
	}

	if err := c.validator.ValidateBeforeUpload(hook.Context, metadata); err != nil {
		for _, item := range c.hooks {
			_ = item.OnValidationFailed(hook.Context, filename, err)
		}
		return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, err
	}

	return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, nil
}

// validatePostUpload TUS的上传后验证回调
func (c *Component) validatePostUpload(hook tusd.HookEvent) (tusd.HTTPResponse, error) {
	if c.validator == nil || !c.config.ValidateAfterUpload {
		return tusd.HTTPResponse{}, nil
	}

	upload, err := c.store.GetUpload(hook.Context, hook.Upload.ID)
	if err != nil {
		return tusd.HTTPResponse{}, ErrGetUploadFailed(err)
	}

	reader, err := upload.GetReader(hook.Context)
	if err != nil {
		return tusd.HTTPResponse{}, ErrGetReaderFailed(err)
	}
	defer reader.Close()

	filename, ok := hook.Upload.MetaData["filename"]
	if !ok {
		filename = hook.Upload.ID
	}

	if err := c.validator.ValidateAfterUpload(hook.Context, filename, reader); err != nil {
		if term, ok := upload.(tusd.TerminatableUpload); ok {
			if termErr := term.Terminate(hook.Context); termErr != nil {
				c.logger.Warn("terminate invalid upload failed",
					elog.String("upload_id", hook.Upload.ID),
					elog.FieldErr(termErr))
			}
		}
		for _, item := range c.hooks {
			_ = item.OnValidationFailed(hook.Context, filename, err)
		}
		return tusd.HTTPResponse{}, err
	}

	return tusd.HTTPResponse{}, nil
}

// handleCompletedUploads 处理完成的上传
func (c *Component) handleCompletedUploads() {
	if c.handler == nil {
		return
	}

	for {
		select {
		case <-c.shutdownCh:
			c.logger.Info("completion handler stopped")
			return
		case event := <-c.handler.CompleteUploads:
			c.processCompletedUpload(event)
		}
	}
}

// processCompletedUpload 处理单个完成的上传
func (c *Component) processCompletedUpload(event tusd.HookEvent) {
	info := event.Upload
	filename, ok := info.MetaData["filename"]
	if !ok {
		filename = info.ID + ".upload"
	}

	// 生成唯一的文件ID
	fileID := ulid.Make().String()
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".upload"
	}
	savedFilename := fileID + ext
	destPath := filepath.Join(c.config.UploadDir, savedFilename)

	// 从TUS存储读取文件
	upload, err := c.store.GetUpload(event.Context, info.ID)
	if err != nil {
		c.logger.Error("get upload failed",
			elog.String("upload_id", info.ID),
			elog.FieldErr(err))
		return
	}

	reader, err := upload.GetReader(event.Context)
	if err != nil {
		c.logger.Error("get reader failed",
			elog.String("upload_id", info.ID),
			elog.FieldErr(err))
		return
	}
	defer reader.Close()

	// 创建目标文件
	destFile, err := os.Create(destPath)
	if err != nil {
		c.logger.Error("create destination file failed",
			elog.String("path", destPath),
			elog.FieldErr(err))
		return
	}
	defer destFile.Close()

	// 复制文件内容
	if _, err := io.Copy(destFile, reader); err != nil {
		c.logger.Error("copy file failed",
			elog.String("upload_id", info.ID),
			elog.FieldErr(err))
		os.Remove(destPath)
		return
	}

	// 清理TUS临时文件
	if term, ok := upload.(tusd.TerminatableUpload); ok {
		if err := term.Terminate(event.Context); err != nil {
			c.logger.Warn("terminate completed upload failed",
				elog.String("upload_id", info.ID),
				elog.FieldErr(err))
		}
	}

	// 构建上传信息
	uploadInfo := &UploadInfo{
		FileID:      fileID,
		FileName:    filename,
		FilePath:    destPath,
		FileSize:    info.Size,
		UploadID:    info.ID,
		Metadata:    info.MetaData,
		CompletedAt: time.Now(),
	}

	// 触发钩子
	for _, hook := range c.hooks {
		if err := hook.OnAfterUpload(event.Context, uploadInfo); err != nil {
			c.logger.Error("hook OnAfterUpload failed",
				elog.String("file_id", fileID),
				elog.FieldErr(err))
		}
	}

	if c.config.EnableAccessLog {
		c.logger.Info("upload completed",
			elog.String("file_id", fileID),
			elog.String("filename", filename),
			elog.String("path", destPath),
			elog.Int64("size", info.Size))
	}
}

// RegisterValidator 注册验证器
func (c *Component) RegisterValidator(validator FileValidator) {
	if validator != nil {
		c.validator = validator
	}
}

// RegisterHook 注册上传钩子
func (c *Component) RegisterHook(hook UploadHook) {
	if hook != nil {
		c.hooks = append(c.hooks, hook)
	}
}

// RegisterRoutes 注册路由到Gin引擎
func (c *Component) RegisterRoutes(engine *gin.Engine) error {
	if c.handler == nil {
		return ErrHandlerNotReady()
	}

	engine.Any(c.config.BasePath+"/*path", c.handleRequest)
	engine.Any(c.config.BasePath, c.handleRequest)

	return nil
}

// RegisterRoutesWithGroup 注册路由到路由组
func (c *Component) RegisterRoutesWithGroup(group *gin.RouterGroup, path string) error {
	if c.handler == nil {
		return ErrHandlerNotReady()
	}

	group.Any(path+"/*path", c.handleRequest)
	group.Any(path, c.handleRequest)

	return nil
}

// handleRequest 处理TUS请求
func (c *Component) handleRequest(ctx *gin.Context) {
	start := time.Now()

	// 使用StripPrefix移除基础路径
	http.StripPrefix(c.config.BasePath, c.handler).ServeHTTP(ctx.Writer, ctx.Request)

	duration := time.Since(start)
	if c.config.EnableAccessLog {
		c.logger.Info("tus request",
			elog.String("method", ctx.Request.Method),
			elog.String("path", ctx.Request.RequestURI),
			elog.Duration("duration", duration))
	}
	if duration > c.config.SlowLogThreshold {
		c.logger.Warn("slow tus request",
			elog.String("method", ctx.Request.Method),
			elog.String("path", ctx.Request.RequestURI),
			elog.Duration("duration", duration))
	}
}

// GetHandler 获取TUS处理器
func (c *Component) GetHandler() *tusd.Handler {
	return c.handler
}

// GetStore 获取TUS存储
func (c *Component) GetStore() tusd.DataStore {
	return c.store
}

// GetConfig 获取配置
func (c *Component) GetConfig() *Config {
	return c.config
}

// Health 健康检查
func (c *Component) Health(ctx context.Context) error {
	if c.handler == nil {
		return ErrHandlerNotReady()
	}
	return nil
}

// Close 关闭组件
func (c *Component) Close() error {
	close(c.shutdownCh)
	c.logger.Info("component closed")
	return nil
}
