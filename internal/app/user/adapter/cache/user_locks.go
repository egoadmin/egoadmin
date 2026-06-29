package cache

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	usercache "github.com/egoadmin/egoadmin/internal/app/user/internal/cache"
)

const userWriteLockTTL = 5 * time.Second

// UserLocks adapts existing Redis locks to application write locks.
type UserLocks struct {
	locks usercache.UserInterface
}

var (
	_ application.UserLocks = (*UserLocks)(nil)
	_ application.RoleLocks = (*UserLocks)(nil)
	_ application.DeptLocks = (*UserLocks)(nil)
)

// NewUserLocks creates a user write-lock adapter.
func NewUserLocks(locks usercache.UserInterface) *UserLocks {
	return &UserLocks{locks: locks}
}

func (l *UserLocks) WithCreateLocks(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	addLock := l.locks.LockAdd()
	if err := addLock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = addLock.Unlock(ctx)
	}()

	updateLock := l.locks.LockUpdate()
	if err := updateLock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = updateLock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithUpdateLock(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	lock := l.locks.LockUpdate()
	if err := lock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithRoleCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	lock := l.locks.RoleLockAdd()
	if err := lock.Lock(ctx, 2*userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithRoleUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	createLock := l.locks.RoleLockAdd()
	if err := createLock.Lock(ctx, 2*userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = createLock.Unlock(ctx)
	}()

	updateLock := l.locks.RoleUpdateAdd()
	if err := updateLock.Lock(ctx, 2*userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = updateLock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithUserCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	lock := l.locks.LockAdd()
	if err := lock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithDeptCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	lock := l.locks.DeptLockAdd()
	if err := lock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithDeptUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	createLock := l.locks.DeptLockAdd()
	if err := createLock.Lock(ctx, 2*userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = createLock.Unlock(ctx)
	}()

	updateLock := l.locks.DeptLockUpdate()
	if err := updateLock.Lock(ctx, 2*userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = updateLock.Unlock(ctx)
	}()

	return fn(ctx)
}

func (l *UserLocks) WithDeptDeleteLocks(ctx context.Context, fn func(context.Context) error) error {
	if l == nil || l.locks == nil {
		return fn(ctx)
	}
	userCreateLock := l.locks.LockAdd()
	if err := userCreateLock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = userCreateLock.Unlock(ctx)
	}()

	userUpdateLock := l.locks.LockUpdate()
	if err := userUpdateLock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = userUpdateLock.Unlock(ctx)
	}()

	deptCreateLock := l.locks.DeptLockAdd()
	if err := deptCreateLock.Lock(ctx, userWriteLockTTL); err != nil {
		return err
	}
	defer func() {
		_ = deptCreateLock.Unlock(ctx)
	}()

	return fn(ctx)
}
