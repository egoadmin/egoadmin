package store

import "gorm.io/gorm"

func scopeStableIDOrder() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(fieldID + " " + asc)
	}
}
