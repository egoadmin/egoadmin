package idcodec

import "errors"

var (
	ErrInvalidConfig = errors.New("idcodec invalid config")
	ErrInvalidID     = errors.New("idcodec invalid id")
	ErrInvalidFormat = errors.New("idcodec invalid format")
	ErrInvalidPrefix = errors.New("idcodec invalid prefix")
	ErrOverflow      = errors.New("idcodec id overflow")
)
