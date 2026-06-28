package application

import "errors"

var (
	ErrSubmittedPassword = errors.New("user application: submitted password denied")
	ErrLoginUARequired   = errors.New("user application: login user agent required")
)
