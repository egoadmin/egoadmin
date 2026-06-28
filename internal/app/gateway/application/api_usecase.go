package application

import (
	"context"
	"encoding/json"
	"fmt"

	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

// APIUseCase orchestrates gateway API catalog workflows.
type APIUseCase struct {
	api     apidomain.Repository
	mysql   mysql.MysqlInterface
	cleaner PermissionPolicyCleaner
}

// PermissionPolicyCleaner removes stale API permission policies from the user service.
type PermissionPolicyCleaner interface {
	DeletePermissionPolicies(ctx context.Context, policies []permissiondomain.Policy) error
}

// APIOptions wires API use case dependencies.
type APIOptions struct {
	APIRepository apidomain.Repository
	Mysql         mysql.MysqlInterface
	PolicyCleaner PermissionPolicyCleaner
}

// APIDTO is the application representation returned to transport adapters.
type APIDTO struct {
	ID       uint64
	Signcode string
	Name     string
	Path     string
	Method   string
	FullPath string
}

// NewAPIUseCase creates an API catalog use case.
func NewAPIUseCase(options APIOptions) *APIUseCase {
	return &APIUseCase{
		api:     options.APIRepository,
		mysql:   options.Mysql,
		cleaner: options.PolicyCleaner,
	}
}

// SyncFromCatalog synchronizes persisted API dictionary rows from generated protobuf catalog data.
func (uc *APIUseCase) SyncFromCatalog(ctx context.Context, data []byte) error {
	apis, err := DecodeAPICatalog(data)
	if err != nil {
		return err
	}

	return uc.mysql.Transaction(ctx, func(txCtx context.Context) error {
		existsAPIs, er := uc.api.FindAll(txCtx)
		if er != nil {
			return er
		}

		existsBySign := make(map[string]*apidomain.API, len(existsAPIs))
		for _, api := range existsAPIs {
			existsBySign[api.Signcode] = api
		}

		catalogBySign := make(map[string]*apidomain.API, len(apis))
		createAPIs := make([]*apidomain.API, 0)
		for _, api := range apis {
			catalogBySign[api.Signcode] = api
			existsAPI, ok := existsBySign[api.Signcode]
			if !ok {
				createAPIs = append(createAPIs, api)
				continue
			}

			if existsAPI.Changed(*api) {
				api.ID = existsAPI.ID
				if er = uc.api.Update(txCtx, existsAPI.ID, api); er != nil {
					return er
				}
			}
		}

		if er = uc.api.CreateBatch(txCtx, createAPIs); er != nil {
			return er
		}

		deleteIDs := make([]uint64, 0)
		deletePolicies := make([]permissiondomain.Policy, 0)
		for _, api := range existsAPIs {
			if _, ok := catalogBySign[api.Signcode]; ok {
				continue
			}
			deleteIDs = append(deleteIDs, api.ID)
			deletePolicies = append(deletePolicies, permissiondomain.Policy{
				Service: api.Path,
				Method:  api.Method,
			})
		}

		if len(deleteIDs) == 0 {
			return nil
		}
		if uc.cleaner != nil {
			if er = uc.cleaner.DeletePermissionPolicies(txCtx, deletePolicies); er != nil {
				return er
			}
		}
		return uc.api.DeleteByIDs(txCtx, deleteIDs)
	})
}

// GetAll returns all API catalog entries.
func (uc *APIUseCase) GetAll(ctx context.Context) ([]APIDTO, error) {
	apis, err := uc.api.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return apiDTOs(apis), nil
}

type apiCatalog struct {
	Version int             `json:"version"`
	APIs    []apiCatalogAPI `json:"apis"`
}

type apiCatalogAPI struct {
	Signcode string `json:"signcode"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Action   string `json:"action"`
}

// DecodeAPICatalog decodes generated API catalog data into domain API entries.
func DecodeAPICatalog(data []byte) ([]*apidomain.API, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("API catalog 为空")
	}

	var catalog apiCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("解析 API catalog 失败: %w", err)
	}
	if catalog.Version != 1 {
		return nil, fmt.Errorf("不支持的 API catalog 版本: %d", catalog.Version)
	}

	apis := make([]*apidomain.API, 0, len(catalog.APIs))
	seen := make(map[string]struct{}, len(catalog.APIs))
	for i, api := range catalog.APIs {
		if api.Signcode == "" || api.Path == "" || api.Action == "" || api.Method == "" {
			return nil, fmt.Errorf("API catalog 第 %d 项缺少必要字段", i)
		}
		if _, ok := seen[api.Signcode]; ok {
			return nil, fmt.Errorf("API catalog 存在重复 signcode: %s", api.Signcode)
		}
		seen[api.Signcode] = struct{}{}
		apis = append(apis, &apidomain.API{
			Signcode: api.Signcode,
			Name:     api.Action,
			Path:     api.Path,
			Method:   api.Method,
		})
	}

	return apis, nil
}

func apiDTOs(apis []*apidomain.API) []APIDTO {
	out := make([]APIDTO, 0, len(apis))
	for _, api := range apis {
		if api == nil {
			continue
		}
		out = append(out, APIDTO{
			ID:       api.ID,
			Signcode: api.Signcode,
			Name:     api.Name,
			Path:     api.Path,
			Method:   api.Method,
			FullPath: api.FullPath(),
		})
	}
	return out
}
