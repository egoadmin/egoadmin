package mysql

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/google/wire"
	"github.com/gotomicro/ego-component/egorm"
	"gorm.io/gorm"
)

var CoreProviderSet = wire.NewSet(
	NewDB,
	NewMysql,
	NewID,
)

var NoIDProviderSet = wire.NewSet(
	NewDB,
	NewMysqlNoID,
)

// idGen id发号器
var idGen xorm.IDSetter

// NewDB 初始化数据库。
// 依赖 config.EgoReady 以保证在 ego 配置加载（econf 就绪）之后构造。
func NewDB(_ config.EgoReady) *egorm.Component {
	db := egorm.Load("client.mysql").Build()
	ApplyGormConfig(db)

	return db
}

// NewID 初始化id发号器
func NewID(g xflake.Geter) xorm.IDSetter {
	st := xorm.NewIDGen(g)
	idGen = st

	return st
}

func NewMysqlNoID(db *egorm.Component) MysqlInterface {
	return &Mysql{
		cc: db,
	}
}

// MysqlInterface ...
type MysqlInterface interface {
	Migrate(ctx context.Context, models []any, joinTables []MigrationJoinTable) error
	// Transaction starts a transaction and propagates it through context.
	//
	// If ctx already carries a transaction, callback is executed with the
	// existing transaction context.
	Transaction(ctx context.Context, callback func(context.Context) error) error
	// WithTx returns the transaction DB from ctx, or the normal DB when ctx has no transaction.
	WithTx(ctx context.Context) *gorm.DB
}
