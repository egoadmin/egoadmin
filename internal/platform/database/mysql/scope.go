package mysql

import "gorm.io/gorm"

// 预留:稳定 ID 排序 scope，用于分页查询的确定性排序。
//
//nolint:unused // 预留:供后续分页排序使用
func scopeStableIDOrder() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(fieldID + " " + asc)
	}
}
