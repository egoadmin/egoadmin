package mysql

import (
	"context"
	"errors"

	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

// RoleRepository implements role.Repository with MySQL.
type RoleRepository struct {
	db platformmysql.MysqlInterface
}

var _ roledomain.Repository = (*RoleRepository)(nil)

// NewRoleRepository creates a MySQL-backed role repository.
func NewRoleRepository(db platformmysql.MysqlInterface) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, aggregate *roledomain.Role) error {
	model := roleModelFromDomain(aggregate)
	if model == nil {
		return roledomain.ErrNotFound
	}
	model.Policies = rolePolicyModelsFromDomain(model.ID, aggregate.Policies)
	if err := r.db.WithTx(ctx).Create(model).Error; err != nil {
		return err
	}
	aggregate.ID = model.ID
	return nil
}

func (r *RoleRepository) Update(ctx context.Context, id uint64, aggregate *roledomain.Role) error {
	model := roleModelFromDomain(aggregate)
	if model == nil {
		return roledomain.ErrNotFound
	}
	model.ID = id
	model.Policies = rolePolicyModelsFromDomain(id, aggregate.Policies)
	return r.db.Transaction(ctx, func(txCtx context.Context) error {
		db := r.db.WithTx(txCtx)
		if err := db.
			Model(&roleAggregateModel{Model: xorm.Model{ID: id}}).
			Select("Name", "Typ", "Desc", "Uses", "ViewMenus", "DataPerm", "OwnerUserID", "OwnerDeptID").
			Updates(model).
			Error; err != nil {
			return err
		}
		if err := db.
			Where(map[string]any{"role_model_id": id}).
			Delete(&rolePermissionPolicyModel{}).
			Error; err != nil {
			return err
		}
		if len(model.Policies) == 0 {
			return nil
		}
		return db.CreateInBatches(model.Policies, 100).Error
	})
}

func (r *RoleRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.Transaction(ctx, func(txCtx context.Context) error {
		db := r.db.WithTx(txCtx)
		if err := db.
			Where(map[string]any{"role_model_id": id}).
			Delete(&rolePermissionPolicyModel{}).
			Error; err != nil {
			return err
		}
		return db.Unscoped().Delete(&roleAggregateModel{Model: xorm.Model{ID: id}}).Error
	})
}

func (r *RoleRepository) FindByID(ctx context.Context, id uint64) (*roledomain.Role, error) {
	model := &roleAggregateModel{}
	err := r.db.WithTx(ctx).Preload("Policies").First(model, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, roledomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*roledomain.Role, error) {
	model := &roleAggregateModel{}
	err := r.db.WithTx(ctx).Where(&roleAggregateModel{Name: name}).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, roledomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}
