package authsession

import (
	"context"
	"errors"

	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
)

var (
	ErrMissingToken       = errors.New("authsession: token is missing")
	ErrInvalidToken       = errors.New("authsession: token is invalid")
	ErrTokenExpired       = errors.New("authsession: token has expired")
	ErrTokenRevoked       = errors.New("authsession: token has been revoked")
	ErrSessionExpired     = errors.New("authsession: session has expired")
	ErrSessionRevoked     = errors.New("authsession: session has been revoked")
	ErrSubjectInvalid     = errors.New("authsession: subject is invalid")
	ErrSubjectDisabled    = errors.New("authsession: subject is disabled")
	ErrRefreshReused      = errors.New("authsession: refresh token has been reused")
	ErrRefreshExpired     = errors.New("authsession: refresh token has expired")
	ErrSessionExists      = errors.New("authsession: session already exists for device")
	ErrTooManySessions    = errors.New("authsession: too many active sessions")
	ErrInvalidConfig      = errors.New("authsession: invalid config")
	errRecordNotFound     = errors.New("authsession: record not found")
	errTokenHashMismatch  = errors.New("authsession: token hash mismatch")
	errRefreshHashMissing = errors.New("authsession: refresh hash is missing")
)

type statusError struct {
	cause  error
	status Status
}

func (e *statusError) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	if e.status == "" {
		return e.cause.Error()
	}
	return e.cause.Error() + ": " + string(e.status)
}

func (e *statusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func revokedStatusError(cause error, status Status, reason Status) error {
	if reason != "" {
		status = reason
	}
	if status == "" || status == StatusActive {
		return cause
	}
	return &statusError{cause: cause, status: status}
}

func statusFromError(err error) (Status, bool) {
	var target *statusError
	if !errors.As(err, &target) || target == nil || target.status == "" {
		return "", false
	}
	return target.status, true
}

func toEcode(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrMissingToken):
		return ecode.ErrorUnauthenticated().WithMessage(platformi18n.Message(ctx, "AuthMissingToken"))
	case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrRefreshExpired), errors.Is(err, ErrSessionExpired), errors.Is(err, errRecordNotFound):
		return ecode.ErrorLoginExpired().WithMessage(platformi18n.Message(ctx, "LoginExpired"))
	case errors.Is(err, ErrInvalidToken), errors.Is(err, ErrTokenRevoked), errors.Is(err, ErrSessionRevoked), errors.Is(err, ErrRefreshReused), errors.Is(err, errTokenHashMismatch):
		if status, ok := statusFromError(err); ok {
			return revokedStatusEcode(ctx, status)
		}
		return ecode.ErrorNotLogin().WithMessage(platformi18n.Message(ctx, "LoginInvalid"))
	case errors.Is(err, ErrSubjectInvalid), errors.Is(err, ErrSubjectDisabled):
		return ecode.ErrorNotLogin().WithMessage(platformi18n.Message(ctx, "AccountUnavailable"))
	default:
		return err
	}
}

// ToEcode maps authsession validation errors to outward API errors.
func ToEcode(err error) error {
	return ToEcodeContext(context.Background(), err)
}

// ToEcodeContext maps authsession validation errors to outward API errors using request locale.
func ToEcodeContext(ctx context.Context, err error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return toEcode(ctx, err)
}

// IsSessionError reports whether err is an auth/session state error handled by ToEcode.
func IsSessionError(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrMissingToken),
		errors.Is(err, ErrTokenExpired),
		errors.Is(err, ErrRefreshExpired),
		errors.Is(err, ErrSessionExpired),
		errors.Is(err, errRecordNotFound),
		errors.Is(err, ErrInvalidToken),
		errors.Is(err, ErrTokenRevoked),
		errors.Is(err, ErrSessionRevoked),
		errors.Is(err, ErrRefreshReused),
		errors.Is(err, errTokenHashMismatch),
		errors.Is(err, ErrSubjectInvalid),
		errors.Is(err, ErrSubjectDisabled):
		return true
	default:
		return false
	}
}

func revokedStatusEcode(ctx context.Context, status Status) error {
	messageID := "LoginInvalid"
	switch status {
	case StatusLogout:
		messageID = "LoggedOut"
	case StatusExpired:
		messageID = "LoginExpired"
	case StatusKicked:
		messageID = "ForcedOffline"
	case StatusReplaced:
		messageID = "LoginReplaced"
	case StatusRefreshReused:
		messageID = "LoginAbnormal"
	}
	return ecode.ErrorNotLogin().
		WithMessage(platformi18n.Message(ctx, messageID)).
		WithMd(map[string]string{"auth_status": string(status)})
}
