package mysql

import (
	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type apiModel struct {
	xorm.Model
	Signcode string `gorm:"uniqueIndex;type:varchar(255);not null;default:'';comment:接口编码,md5(path+method)"`
	Name     string `gorm:"index;type:varchar(255);not null;default:'';comment:接口名称"`
	Path     string `gorm:"index;type:varchar(255);not null;default:'';comment:gRPC 服务名,casbin obj"`
	Method   string `gorm:"index;type:varchar(255);not null;default:'';comment:gRPC 方法名,casbin act;"`
}

func (apiModel) TableName() string {
	return "api"
}

func (m *apiModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *apiModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

func (m *apiModel) toDomain() *apidomain.API {
	if m == nil {
		return nil
	}
	return &apidomain.API{
		ID:       m.ID,
		Signcode: m.Signcode,
		Name:     m.Name,
		Path:     m.Path,
		Method:   m.Method,
	}
}

func apiModelFromDomain(api *apidomain.API) *apiModel {
	if api == nil {
		return nil
	}
	return &apiModel{
		Model:    xorm.Model{ID: api.ID},
		Signcode: api.Signcode,
		Name:     api.Name,
		Path:     api.Path,
		Method:   api.Method,
	}
}

func apiModelsFromDomain(apis []*apidomain.API) []*apiModel {
	models := make([]*apiModel, 0, len(apis))
	for _, api := range apis {
		if api == nil {
			continue
		}
		models = append(models, apiModelFromDomain(api))
	}
	return models
}

func apiModelsToDomain(models []*apiModel) []*apidomain.API {
	apis := make([]*apidomain.API, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		apis = append(apis, model.toDomain())
	}
	return apis
}
