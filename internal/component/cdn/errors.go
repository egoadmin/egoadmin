package cdn

import "errors"

var (
	ErrSignatureRequired     = errors.New("cdn: signature required")
	ErrSignatureInvalid      = errors.New("cdn: signature invalid")
	ErrSignatureExpired      = errors.New("cdn: signature expired")
	ErrReferenceGone         = errors.New("cdn: reference gone")
	ErrObjectUnavailable     = errors.New("cdn: object unavailable")
	ErrInvalidReferenceID    = errors.New("cdn: invalid reference id")
	ErrInvalidDisplay        = errors.New("cdn: invalid display")
	ErrInvalidProcessPath    = errors.New("cdn: invalid process path")
	ErrImageProcessorMissing = errors.New("cdn: image processor url is required")
)
