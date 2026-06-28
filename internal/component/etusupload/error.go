package etusupload

import "fmt"

// Error 定义TUS上传组件的错误类型

var (
	ErrConfigNotInitialized = func() error {
		return fmt.Errorf("etusupload: config not initialized")
	}

	ErrComponentNotInitialized = func() error {
		return fmt.Errorf("etusupload: component not initialized")
	}

	ErrMissingFilename = func() error {
		return fmt.Errorf("etusupload: missing filename in metadata")
	}

	ErrInvalidFileSize = func(actual, max int64) error {
		return fmt.Errorf("etusupload: file size %d exceeds max %d", actual, max)
	}

	ErrGetUploadFailed = func(err error) error {
		return fmt.Errorf("etusupload: failed to get upload: %w", err)
	}

	ErrGetReaderFailed = func(err error) error {
		return fmt.Errorf("etusupload: failed to get reader: %w", err)
	}

	ErrValidationFailed = func(msg string) error {
		return fmt.Errorf("etusupload: validation failed: %s", msg)
	}

	ErrUnsupportedFileType = func(filename string) error {
		return fmt.Errorf("etusupload: unsupported file type: %s", filename)
	}

	ErrUnsupportedMimeType = func(mimeType string) error {
		return fmt.Errorf("etusupload: unsupported mime type: %s", mimeType)
	}

	ErrCreateDirectoryFailed = func(dir string, err error) error {
		return fmt.Errorf("etusupload: failed to create directory %s: %w", dir, err)
	}

	ErrCreateHandlerFailed = func(err error) error {
		return fmt.Errorf("etusupload: failed to create TUS handler: %w", err)
	}

	ErrHandlerNotReady = func() error {
		return fmt.Errorf("etusupload: handler not ready")
	}

	ErrValidatorNotSet = func() error {
		return fmt.Errorf("etusupload: validator not set")
	}

	ErrCompletionHandlerNotSet = func() error {
		return fmt.Errorf("etusupload: completion handler not set")
	}
)
