package cache

import (
	"github.com/gotomicro/ego/task/ecron"
)

// UserInterface 用户相关操作
type UserInterface interface {
	// LogLockClean 用户日志清理锁
	LogLockClean() ecron.Lock
	// LockAdd 用户新增锁
	LockAdd() ecron.Lock
	// LockUpdate 用户修改锁
	LockUpdate() ecron.Lock
	// DeptLockAdd 组织新增锁
	DeptLockAdd() ecron.Lock
	// DeptLockUpdate 组织修改锁
	DeptLockUpdate() ecron.Lock
	// RoleLockAdd 角色新增锁
	RoleLockAdd() ecron.Lock
	// RoleUpdateAdd 角色修改锁
	RoleUpdateAdd() ecron.Lock
}
