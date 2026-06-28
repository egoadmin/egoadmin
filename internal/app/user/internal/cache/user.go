package cache

import (
	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/eredis/ecronlock"
	"github.com/gotomicro/ego/task/ecron"
)

// User 用户操作
type User struct {
	cc *eredis.Component
	ec *ecronlock.Component
}

// NewUser 实例化用户
func NewUser(cc *eredis.Component, ec *ecronlock.Component) UserInterface {
	return &User{
		cc: cc,
		ec: ec,
	}
}

// LogLockClean 用户日志清理锁
func (s *User) LogLockClean() ecron.Lock {
	return s.ec.NewLock("user-log:lock:clean")
}

// LockAdd 锁定新增
func (s *User) LockAdd() ecron.Lock {
	return s.ec.NewLock("user:lock:add")
}

// LockUpdate 锁定修改
func (s *User) LockUpdate() ecron.Lock {
	return s.ec.NewLock("user:lock:update")
}

// DeptLockAdd 组织新增锁
func (s *User) DeptLockAdd() ecron.Lock {
	return s.ec.NewLock("user-dept:lock:add")
}

// DeptLockUpdate 组织修改锁
func (s *User) DeptLockUpdate() ecron.Lock {
	return s.ec.NewLock("user-dept:lock:update")
}

// RoleLockAdd 角色新增锁
func (s *User) RoleLockAdd() ecron.Lock {
	return s.ec.NewLock("user-role:lock:add")
}

// RoleUpdateAdd 角色修改锁
func (s *User) RoleUpdateAdd() ecron.Lock {
	return s.ec.NewLock("user-role:lock:update")
}
