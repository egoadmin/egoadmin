package store

import "gorm.io/gorm"

// userScopeIds id筛选.
//
//nolint:unused // 预留:按 ID 列表过滤用户的 scope
func userScopeIds(ids []uint64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(ids) == 0 {
			return db
		}

		return db.Where("id IN (?)", ids)
	}
}

// UserScopeUsernameLike 用户名.
func UserScopeUsernameLike(username string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if username == "" {
			return db
		}

		return db.Where("username LIKE ?", "%"+username+"%")
	}
}

// UserScopeNameLike 姓名.
func UserScopeNameLike(name string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if name == "" {
			return db
		}

		return db.Where("name LIKE ?", "%"+name+"%")
	}
}

// UserScopeUsernameOrNameLike 用户名或姓名
func UserScopeUsernameOrNameLike(name string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if name == "" {
			return db
		}

		return db.Where("username LIKE ? OR name LIKE ?", "%"+name+"%", "%"+name+"%")
	}
}

// userScopeUserStatus 用户状态
func userScopeUserStatus(status int32) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if status == UserModelStatusInvalid {
			return db.Where("user_status = ?", UserModelStatusInvalid)
		} else if status == UserModelStatusValid {
			return db.Where("user_status = ?", UserModelStatusValid)
		}
		return db
	}
}

// userScopeUserOnline 用户在线状态
func userScopeUserOnline(online int32) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if online == UserModelOnline {
			return db.Where("user_online = ?", UserModelOnline)
		} else if online == UserModelOffline {
			return db.Where("user_online = ?", UserModelOffline)
		}
		return db
	}
}

// userScopeLastLoginIP 用户最后登录IP
func userScopeLastLoginIP(ip string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if ip == "" {
			return db
		}
		return db.Where("last_login_ip LIKE ?", "%"+ip+"%")
	}
}

// userScopePhone 用户手机号
func userScopePhone(phone string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if phone == "" {
			return db
		}
		return db.Where("phone = ?", phone)
	}
}

// userScopeNameHiddenRoot 隐藏root用户.
func userScopeNameHiddenRoot() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("username <> ?", UserModelUsernameRoot)
	}
}

// userScopeNameHiddenAdmin 隐藏admin用户.
func userScopeNameHiddenAdmin() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("username <> ?", UserModelUsernameAdmin)
	}
}

// userScopeDept 组织.
func userScopeDept(id uint64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if id == 0 {
			return db
		}

		return db.Where("dept_id = ?", id)
	}
}

// userScopeDeptsIn 组织列表.
func userScopeDeptsIn(ids []uint64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(ids) == 0 {
			return db
		}

		return db.Where("dept_id IN (?)", ids)
	}
}

func userScopeRoleExists(roleID uint64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if roleID == 0 {
			return db
		}

		subQuery := db.Session(&gorm.Session{NewDB: true}).Model(&UserRole{}).
			Select("user_model_id").
			Where(&UserRole{RoleModelID: roleID})
		return db.Where("id IN (?)", subQuery)
	}
}
