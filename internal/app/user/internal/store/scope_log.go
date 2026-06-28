package store

import (
	"time"

	"gorm.io/gorm"
)

// logScopeUsernameLike 模糊查询日志操作用户.
func logScopeUsernameLike(username string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if username == "" {
			return db
		}

		return db.Where("username LIKE ?", "%"+username+"%")
	}
}

// logScopeTitleLike 查询日志名称.
func logScopeTitleLike(title string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if title == "" {
			return db
		}

		return db.Where("title LIKE ?", "%"+title+"%")
	}
}

// logScopeCratedAtRange 查询日志创建时间范围
func logScopeCratedAtRange(start, end time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if !start.IsZero() && !end.IsZero() {
			return db.Where("created_at BETWEEN ? AND ?", start, end.Add(999*time.Millisecond))
		}
		return db
	}
}
