package application

import (
	"errors"
)

// UserMessageError carries a safe message for transport adapters.
type UserMessageError struct {
	Message string
}

func (e *UserMessageError) Error() string {
	return e.Message
}

func userMessageError(message string) error {
	return &UserMessageError{Message: message}
}

// UserMessage extracts a user-facing message from an application error.
func UserMessage(err error) (string, bool) {
	var target *UserMessageError
	if errors.As(err, &target) {
		return target.Message, true
	}
	return "", false
}
