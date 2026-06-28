package role

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound   = errors.New("role: not found")
	ErrNameExists = errors.New("role: name exists")
	ErrInUse      = errors.New("role: in use")
)

// InUseError reports how many users still reference a role.
type InUseError struct {
	Count int64
}

func (e InUseError) Error() string {
	return fmt.Sprintf("role: in use by %d users", e.Count)
}

func (e InUseError) Unwrap() error {
	return ErrInUse
}
