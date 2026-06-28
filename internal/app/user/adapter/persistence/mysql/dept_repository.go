package mysql

import (
	"context"
	"errors"

	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

// DeptRepository implements dept.Repository with MySQL.
type DeptRepository struct {
	db    platformmysql.MysqlInterface
	idgen xflake.Geter
}

var _ deptdomain.Repository = (*DeptRepository)(nil)

// NewDeptRepository creates a MySQL-backed department repository.
func NewDeptRepository(db platformmysql.MysqlInterface, idgen xflake.Geter) *DeptRepository {
	return &DeptRepository{
		db:    db,
		idgen: idgen,
	}
}

func (r *DeptRepository) NextID(ctx context.Context) (uint64, error) {
	return r.idgen.Get()
}

func (r *DeptRepository) Create(ctx context.Context, aggregate *deptdomain.Dept) error {
	model := deptModelFromDomain(aggregate)
	if model == nil {
		return deptdomain.ErrNotFound
	}
	if err := r.db.WithTx(ctx).Create(model).Error; err != nil {
		return err
	}
	aggregate.ID = model.ID
	return nil
}

func (r *DeptRepository) UpdateName(ctx context.Context, id uint64, name string) error {
	return r.db.WithTx(ctx).
		Model(&deptModel{Model: xorm.Model{ID: id}}).
		UpdateColumn("dept_name", name).
		Error
}

func (r *DeptRepository) UpdatePriorities(ctx context.Context, updates []deptdomain.PriorityUpdate) error {
	db := r.db.WithTx(ctx)
	for _, update := range updates {
		if err := db.
			Model(&deptModel{}).
			Where("id = ?", update.ID).
			UpdateColumn("priority", update.Priority).
			Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *DeptRepository) DeleteByIDs(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithTx(ctx).
		Unscoped().
		Where("id IN (?)", ids).
		Delete(&deptModel{}).
		Error
}

func (r *DeptRepository) FindByID(ctx context.Context, id uint64) (*deptdomain.Dept, error) {
	model := &deptModel{}
	err := r.db.WithTx(ctx).First(model, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, deptdomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *DeptRepository) FindSubtree(ctx context.Context, id uint64) ([]*deptdomain.Dept, error) {
	models := make([]deptModel, 0)
	subquery := r.db.WithTx(ctx).Model(&deptModel{Model: xorm.Model{ID: id}}).Select("`code`")
	err := r.db.WithTx(ctx).
		Table("dept as dt, (?) as dp", subquery).
		Where("dt.`code` LIKE CONCAT(dp.`code`,'%')").
		Find(&models).
		Error
	if err != nil {
		return nil, err
	}
	return deptModelsToDomain(models), nil
}

func (r *DeptRepository) CountByParent(ctx context.Context, parentID uint64) (int64, error) {
	var count int64
	err := r.db.WithTx(ctx).
		Model(&deptModel{}).
		Where("parent_id = ?", parentID).
		Count(&count).
		Error
	return count, err
}

func (r *DeptRepository) FindByParentAndName(ctx context.Context, parentID uint64, name string) (*deptdomain.Dept, error) {
	model := &deptModel{}
	err := r.db.WithTx(ctx).
		Where("parent_id = ?", parentID).
		Where("dept_name = ?", name).
		First(model).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, deptdomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}
