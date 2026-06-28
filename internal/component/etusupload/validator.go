package etusupload

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// FileValidator 文件验证器接口
type FileValidator interface {
	// ValidateBeforeUpload 上传前验证（基于元数据）
	ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error

	// ValidateAfterUpload 上传后验证（基于文件内容）
	ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error
}

// DefaultValidator 默认验证器
type DefaultValidator struct {
	allowedExtensions  []string
	rejectedExtensions []string
	allowedMimeTypes   []string
	rejectedMimeTypes  []string
}

// NewDefaultValidator 创建默认验证器
func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{
		allowedExtensions:  []string{},
		rejectedExtensions: []string{},
		allowedMimeTypes:   []string{},
		rejectedMimeTypes:  []string{},
	}
}

func NewDefaultValidatorFromConfig(config *Config) *DefaultValidator {
	return NewDefaultValidator().
		WithAllowedExtensions(config.AllowedExtensions).
		WithRejectedExtensions(config.RejectedExtensions).
		WithAllowedMimeTypes(config.AllowedMimeTypes).
		WithRejectedMimeTypes(config.RejectedMimeTypes)
}

// WithAllowedExtensions 设置允许的扩展名
func (v *DefaultValidator) WithAllowedExtensions(exts []string) *DefaultValidator {
	v.allowedExtensions = exts
	return v
}

// WithRejectedExtensions 设置拒绝的扩展名
func (v *DefaultValidator) WithRejectedExtensions(exts []string) *DefaultValidator {
	v.rejectedExtensions = exts
	return v
}

// WithAllowedMimeTypes 设置允许的MIME类型
func (v *DefaultValidator) WithAllowedMimeTypes(types []string) *DefaultValidator {
	v.allowedMimeTypes = types
	return v
}

// WithRejectedMimeTypes 设置拒绝的MIME类型
func (v *DefaultValidator) WithRejectedMimeTypes(types []string) *DefaultValidator {
	v.rejectedMimeTypes = types
	return v
}

// ValidateBeforeUpload 实现FileValidator接口
func (v *DefaultValidator) ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error {
	filename := metadata["filename"]
	if filename == "" {
		return ErrMissingFilename()
	}
	if err := v.validateExtension(filename); err != nil {
		return err
	}

	mimeType := firstNonEmpty(metadata["filetype"], metadata["contentType"], metadata["content-type"])
	if mimeType != "" {
		if err := v.validateMimeType(mimeType); err != nil {
			return err
		}
	}

	return nil
}

// ValidateAfterUpload 实现FileValidator接口
func (v *DefaultValidator) ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error {
	if err := v.validateExtension(filename); err != nil {
		return err
	}

	var header [512]byte
	n, err := reader.Read(header[:])
	if err != nil && err != io.EOF {
		return err
	}
	if n > 0 {
		if err := v.validateMimeType(http.DetectContentType(header[:n])); err != nil {
			return err
		}
	}

	return nil
}

func (v *DefaultValidator) validateExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if len(v.rejectedExtensions) > 0 && containsNormalized(v.rejectedExtensions, ext) {
		return ErrUnsupportedFileType(filename)
	}
	if len(v.allowedExtensions) > 0 && !containsNormalized(v.allowedExtensions, ext) {
		return ErrUnsupportedFileType(filename)
	}
	return nil
}

func (v *DefaultValidator) validateMimeType(mimeType string) error {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if len(v.rejectedMimeTypes) > 0 && containsNormalized(v.rejectedMimeTypes, mimeType) {
		return ErrUnsupportedMimeType(mimeType)
	}
	if len(v.allowedMimeTypes) > 0 && !containsNormalized(v.allowedMimeTypes, mimeType) {
		return ErrUnsupportedMimeType(mimeType)
	}
	return nil
}

func containsNormalized(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// NoopValidator 空验证器（不做任何验证）
type NoopValidator struct{}

// ValidateBeforeUpload 不做任何验证
func (v *NoopValidator) ValidateBeforeUpload(ctx context.Context, metadata map[string]string) error {
	return nil
}

// ValidateAfterUpload 不做任何验证
func (v *NoopValidator) ValidateAfterUpload(ctx context.Context, filename string, reader io.Reader) error {
	return nil
}
