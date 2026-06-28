package dept

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound        = errors.New("dept: not found")
	ErrNameExists      = errors.New("dept: name exists")
	ErrMaxLevel        = errors.New("dept: max level reached")
	ErrTooManySiblings = errors.New("dept: too many siblings")
	ErrPriorityChanged = errors.New("dept: priority data changed")
	ErrInvalidPriority = errors.New("dept: invalid priority")
	ErrInUse           = errors.New("dept: in use")
)

// NameExistsError reports a duplicate department name in the same parent.
type NameExistsError struct {
	Name string
}

func (e NameExistsError) Error() string {
	return fmt.Sprintf("dept: name %q exists", e.Name)
}

func (e NameExistsError) Unwrap() error {
	return ErrNameExists
}

// MaxLevelError reports that a parent department cannot accept more children.
type MaxLevelError struct {
	MaxLevel int32
}

func (e MaxLevelError) Error() string {
	return fmt.Sprintf("dept: max level %d reached", e.MaxLevel)
}

func (e MaxLevelError) Unwrap() error {
	return ErrMaxLevel
}

// InUseError reports how many users still reference a department subtree.
type InUseError struct {
	Count int64
}

func (e InUseError) Error() string {
	return fmt.Sprintf("dept: in use by %d users", e.Count)
}

func (e InUseError) Unwrap() error {
	return ErrInUse
}
