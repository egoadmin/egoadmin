package mysql

import (
	"context"

	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
)

// APIRepository implements api.Repository with MySQL.
type APIRepository struct {
	db platformmysql.MysqlInterface
}

var _ apidomain.Repository = (*APIRepository)(nil)

// NewAPIRepository creates a MySQL-backed API repository.
func NewAPIRepository(db platformmysql.MysqlInterface) *APIRepository {
	return &APIRepository{db: db}
}

func (r *APIRepository) CreateBatch(ctx context.Context, apis []*apidomain.API) error {
	if len(apis) == 0 {
		return nil
	}
	models := apiModelsFromDomain(apis)
	if len(models) == 0 {
		return nil
	}
	if err := r.db.WithTx(ctx).CreateInBatches(&models, 100).Error; err != nil {
		return err
	}
	for i, model := range models {
		apis[i].ID = model.ID
	}
	return nil
}

func (r *APIRepository) Update(ctx context.Context, id uint64, api *apidomain.API) error {
	model := apiModelFromDomain(api)
	if model == nil {
		return nil
	}
	return r.db.WithTx(ctx).
		Model(&apiModel{Model: xorm.Model{ID: id}}).
		Select("Signcode", "Name", "Path", "Method").
		Updates(model).
		Error
}

func (r *APIRepository) DeleteByIDs(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithTx(ctx).Unscoped().Where(map[string]any{"id": ids}).Delete(&apiModel{}).Error
}

func (r *APIRepository) FindAll(ctx context.Context) ([]*apidomain.API, error) {
	models := make([]*apiModel, 0)
	if err := r.db.WithTx(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	return apiModelsToDomain(models), nil
}

func (r *APIRepository) FindAllBySign(ctx context.Context, signs []string) ([]*apidomain.API, error) {
	if len(signs) == 0 {
		return []*apidomain.API{}, nil
	}
	models := make([]*apiModel, 0)
	if err := r.db.WithTx(ctx).Where(map[string]any{"signcode": signs}).Find(&models).Error; err != nil {
		return nil, err
	}
	return apiModelsToDomain(models), nil
}
