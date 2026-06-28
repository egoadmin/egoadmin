package cdn

import (
	"context"
	"errors"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/upload"
)

func ensureDownloadable(object *upload.DownloadObject, now time.Time, allowTemporary bool) error {
	if object == nil {
		return upload.ErrReferenceNotFound
	}
	switch object.ReferenceStatus {
	case upload.ReferenceStatusBound:
	case upload.ReferenceStatusTemporary:
		if !allowTemporary || now.After(object.ExpiresAt) {
			return ErrReferenceGone
		}
	case upload.ReferenceStatusReleased, upload.ReferenceStatusExpired:
		return ErrReferenceGone
	default:
		return ErrReferenceGone
	}
	if object.ObjectStatus != upload.ObjectStatusAvailable {
		return ErrObjectUnavailable
	}
	return nil
}

func statusFromError(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrInvalidReferenceID),
		errors.Is(err, ErrInvalidDisplay),
		errors.Is(err, ErrInvalidProcessPath):
		return 400
	case errors.Is(err, ErrSignatureRequired):
		return 401
	case errors.Is(err, ErrSignatureInvalid),
		errors.Is(err, ErrSignatureExpired),
		errors.Is(err, upload.ErrReferenceForbidden):
		return 403
	case errors.Is(err, upload.ErrReferenceNotFound),
		errors.Is(err, upload.ErrObjectNotFound):
		return 404
	case errors.Is(err, ErrReferenceGone):
		return 410
	case errors.Is(err, ErrImageProcessorMissing):
		return 502
	case errors.Is(err, context.DeadlineExceeded):
		return 504
	case errors.Is(err, ErrObjectUnavailable):
		return 410
	default:
		return 500
	}
}
