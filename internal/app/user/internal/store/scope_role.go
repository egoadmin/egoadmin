package store

import "gorm.io/gorm"

// roleScopeNameLike 查询角色名称.
func roleScopeNameLike(name string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if name == "" {
			return db
		}

		return db.Where("name LIKE ?", "%"+name+"%")
	}
}
