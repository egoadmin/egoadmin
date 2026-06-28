package dept

import (
	"fmt"
	"math"
	"strconv"
)

const (
	ParentTop uint64 = 0

	StatusValid   int32 = 1
	StatusInvalid int32 = 2

	MaxLevel int32 = 5
)

type Dept struct {
	ID       uint64
	Code     string
	ParentID uint64
	Name     string
	Leader   string
	Phone    string
	Email    string
	Remark   string
	Priority int32
	Status   int32
	Level    int32
}

type PriorityUpdate struct {
	ID       uint64
	Priority int32
}

func NewRootChild(id uint64, name string, siblingCount int64) (*Dept, error) {
	dept, err := newChild(id, ParentTop, "", 0, name, siblingCount)
	if err != nil {
		return nil, err
	}
	return dept, nil
}

func NewChild(id uint64, parent *Dept, name string, siblingCount int64) (*Dept, error) {
	if parent == nil {
		return nil, ErrNotFound
	}
	if parent.Level >= MaxLevel {
		return nil, MaxLevelError{MaxLevel: MaxLevel}
	}
	return newChild(id, parent.ID, parent.Code, parent.Level, name, siblingCount)
}

func ValidatePriorityUpdates(updates []PriorityUpdate, siblingCount int64) error {
	if siblingCount < 0 || int64(len(updates)) != siblingCount {
		return ErrPriorityChanged
	}
	if siblingCount > math.MaxInt32 {
		return ErrTooManySiblings
	}
	maxPriority := int32(siblingCount)
	seen := make(map[int32]struct{}, len(updates))
	for _, update := range updates {
		if update.Priority < 1 || update.Priority > maxPriority {
			return ErrInvalidPriority
		}
		seen[update.Priority] = struct{}{}
	}
	if int64(len(seen)) != siblingCount {
		return ErrInvalidPriority
	}
	return nil
}

func newChild(id uint64, parentID uint64, parentCode string, parentLevel int32, name string, siblingCount int64) (*Dept, error) {
	if siblingCount >= math.MaxInt32 {
		return nil, ErrTooManySiblings
	}
	dept := &Dept{
		ID:       id,
		ParentID: parentID,
		Name:     name,
		Priority: int32(siblingCount + 1),
		Status:   StatusValid,
		Level:    parentLevel + 1,
	}
	if parentCode == "" {
		dept.Code = strconv.FormatUint(id, 10)
	} else {
		dept.Code = fmt.Sprintf("%s-%d", parentCode, id)
	}
	return dept, nil
}
