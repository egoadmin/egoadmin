package store

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserScopeRoleExistsUsesSubQueryInClause(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("open dry-run sqlite: %v", err)
	}

	stmt := db.Model(&UserModel{}).Scopes(userScopeRoleExists(20001)).Find(&[]UserModel{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "IN (SELECT") {
		t.Fatalf("SQL = %q, want sub-query IN clause", sql)
	}
	if strings.Contains(sql, "=`SELECT") || strings.Contains(sql, "= SELECT") {
		t.Fatalf("SQL = %q, want no direct equality against sub-query", sql)
	}
}
