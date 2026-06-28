package user

import (
	"context"
	"time"
)

// Repository persists user aggregates.
type Repository interface {
	NextID(ctx context.Context) (uint64, error)
	Create(ctx context.Context, aggregate *User) error
	Save(ctx context.Context, aggregate *User) error
	Update(ctx context.Context, id uint64, aggregate *User) error
	FindByID(ctx context.Context, id uint64) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	List(ctx context.Context, query ListQuery) ([]*User, int64, error)
	UpdatePassword(ctx context.Context, id uint64, passwordHash string) error
	MarkLoggedIn(ctx context.Context, id uint64, at time.Time, ip string) error
	MarkOnline(ctx context.Context, id uint64, at time.Time) error
	MarkOffline(ctx context.Context, ids []uint64) error
	FindHeartbeatExpiredIDs(ctx context.Context, before time.Time) ([]uint64, error)
	CountOnline(ctx context.Context) (int64, error)
	Delete(ctx context.Context, ids []uint64) error
}

type ListQuery struct {
	Page     int
	Limit    int
	Sort     string
	Order    string
	Name     string
	Phone    string
	RoleID   uint64
	DeptIDs  []uint64
	Status   Status
	Username string
}
