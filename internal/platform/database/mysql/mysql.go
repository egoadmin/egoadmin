package mysql

import (
	"context"

	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego-component/egorm"
	"github.com/gotomicro/ego/core/elog"
	"gorm.io/gorm"
)

type txKey struct{}

type Mysql struct {
	cc *egorm.Component
}

type MigrationJoinTable struct {
	Model any
	Field string
	Table any
}

func GormConfig() *gorm.Config {
	return &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: &egorm.NamingStrategy{
			SingularTable: true,
		},
	}
}

func ApplyGormConfig(db *egorm.Component) {
	gormConfig := GormConfig()
	db.Config.DisableForeignKeyConstraintWhenMigrating = gormConfig.DisableForeignKeyConstraintWhenMigrating
	db.Config.NamingStrategy = gormConfig.NamingStrategy
}

func MigrationModels() []any {
	return []any{}
}

func MigrationJoinTables() []MigrationJoinTable {
	return []MigrationJoinTable{}
}

func SetupMigrationJoinTables(db interface {
	SetupJoinTable(model any, field string, joinTable any) error
}, joinTables []MigrationJoinTable,
) error {
	for _, joinTable := range joinTables {
		if err := db.SetupJoinTable(joinTable.Model, joinTable.Field, joinTable.Table); err != nil {
			return err
		}
	}
	return nil
}

func NewMysql(db *egorm.Component, id xorm.IDSetter) MysqlInterface {
	return &Mysql{
		cc: db,
	}
}

func (m *Mysql) Migrate(ctx context.Context, models []any, joinTables []MigrationJoinTable) (err error) {
	ApplyGormConfig(m.cc)

	err = SetupMigrationJoinTables(m.cc, joinTables)
	if err != nil {
		return
	}

	err = m.cc.WithContext(ctx).AutoMigrate(models...)
	if err != nil {
		elog.Error("autoMigrate failed", elog.FieldErr(err))
		return err
	}

	return
}

// Transaction starts a GORM transaction and propagates it through context.
func (m *Mysql) Transaction(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return callback(ctx)
	}

	return m.cc.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return callback(context.WithValue(ctx, txKey{}, tx))
	})
}

// WithTx returns the current transaction DB or the normal DB when no transaction exists.
func (m *Mysql) WithTx(ctx context.Context) *gorm.DB {
	return dbWithContext(ctx, m.cc)
}

func txFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	return tx, ok
}

func SetID(model xorm.IDer) error {
	return idGen.Set(model)
}

func DBWithContext(ctx context.Context, cc *egorm.Component) *gorm.DB {
	return dbWithContext(ctx, cc)
}

func Transaction(ctx context.Context, cc *egorm.Component, callback func(*gorm.DB) error) error {
	return transaction(ctx, cc, callback)
}

func dbWithContext(ctx context.Context, cc *egorm.Component) *gorm.DB {
	tx, ok := txFromContext(ctx)
	if ok {
		return tx.WithContext(ctx)
	}

	return cc.WithContext(ctx)
}

func transaction(ctx context.Context, cc *egorm.Component, callback func(*gorm.DB) error) error {
	tx, ok := txFromContext(ctx)
	if ok {
		return callback(tx.WithContext(ctx))
	}

	return cc.WithContext(ctx).Transaction(callback)
}
