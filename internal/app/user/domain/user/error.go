package user

import "errors"

var (
	ErrNotFound             = errors.New("user: not found")
	ErrInvalidDefaultPasswd = errors.New("user: invalid default password source")
	ErrBuiltinPasswordReset = errors.New("user: builtin password reset denied")
	ErrBuiltinUsername      = errors.New("user: builtin username denied")
	ErrUsernameExists       = errors.New("user: username exists")
	ErrPhoneExists          = errors.New("user: phone exists")
	ErrLoginDenied          = errors.New("user: login denied")
)
