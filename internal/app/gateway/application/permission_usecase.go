package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
)

const permissionContractPath = "web/dist/permission-contract.json"

// PermissionUseCase owns gateway-side permission contract and API boundary workflows.
type PermissionUseCase struct {
	conf   *config.Config
	api    apidomain.Repository
	assets fs.FS
}

// PermissionOptions wires permission use case dependencies.
type PermissionOptions struct {
	Conf             *config.Config
	APIRepository    apidomain.Repository
	FrontendAssetsFS fs.FS
}

// MenuContract represents one frontend menu permission declaration.
type MenuContract struct {
	Name string   `json:"name"`
	APIs []string `json:"apis"`
}

// NewPermissionUseCase creates a permission boundary use case.
func NewPermissionUseCase(options PermissionOptions) *PermissionUseCase {
	return &PermissionUseCase{
		conf:   options.Conf,
		api:    options.APIRepository,
		assets: options.FrontendAssetsFS,
	}
}

// EnsurePermissionContract validates the embedded frontend permission contract at startup.
func (uc *PermissionUseCase) EnsurePermissionContract(ctx context.Context) error {
	if uc.skipContractCheck() {
		return nil
	}

	_, err := uc.loadPermissionContract(ctx)
	return err
}

// ValidateRoleAPIBoundary checks whether requested API IDs are within selected menu contracts.
func (uc *PermissionUseCase) ValidateRoleAPIBoundary(ctx context.Context, viewMenus string, requestAPIIDs []uint64) ([]permissiondomain.Policy, error) {
	skipContractCheck := uc.skipContractCheck()

	allowedAPIs := make(map[string]struct{})
	if !skipContractCheck {
		contract, err := uc.loadPermissionContract(ctx)
		if err != nil {
			return nil, userMessageError(platformi18n.Message(ctx, "PermissionContractUnavailable"))
		}

		for _, key := range strings.Split(viewMenus, ",") {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}

			menuContract, ok := contract[trimmedKey]
			if !ok {
				continue
			}

			for _, api := range menuContract.APIs {
				allowedAPIs[strings.ToUpper(api)] = struct{}{}
			}
		}

		if len(allowedAPIs) == 0 && len(requestAPIIDs) > 0 {
			return nil, userMessageError(platformi18n.Message(ctx, "PermissionEmptyMenuAPIs"))
		}
	}

	allDBAPIs, err := uc.api.FindAll(ctx)
	if err != nil {
		return nil, userMessageError(platformi18n.Message(ctx, "PermissionAPIMetadataUnavailable"))
	}

	dbAPIMap := make(map[uint64]*apidomain.API, len(allDBAPIs))
	for _, api := range allDBAPIs {
		dbAPIMap[api.ID] = api
	}

	seenIDs := make(map[uint64]struct{}, len(requestAPIIDs))
	policies := make([]permissiondomain.Policy, 0, len(requestAPIIDs))
	for _, id := range requestAPIIDs {
		if _, ok := seenIDs[id]; ok {
			continue
		}
		seenIDs[id] = struct{}{}

		model, ok := dbAPIMap[id]
		if !ok {
			return nil, userMessageError(platformi18n.Localize(ctx, "PermissionIllegalAPIID", map[string]any{"ID": id}))
		}

		apiFullPath := model.FullPath()
		if !skipContractCheck {
			if _, ok := allowedAPIs[apiFullPath]; ok {
				policies = append(policies, permissiondomain.Policy{
					Service: model.Path,
					Method:  model.Method,
				})
				continue
			}
			return nil, userMessageError(platformi18n.Localize(ctx, "PermissionAPIBoundaryDenied", map[string]any{"Name": model.Name}))
		}

		policies = append(policies, permissiondomain.Policy{
			Service: model.Path,
			Method:  model.Method,
		})
	}

	return permissiondomain.NormalizePolicies(policies), nil
}

// APIIDsByPolicies maps persisted permission policies back to gateway API dictionary IDs.
func (uc *PermissionUseCase) APIIDsByPolicies(ctx context.Context, policies []permissiondomain.Policy) ([]uint64, error) {
	policies = permissiondomain.NormalizePolicies(policies)
	if len(policies) == 0 {
		return []uint64{}, nil
	}

	allDBAPIs, err := uc.api.FindAll(ctx)
	if err != nil {
		return nil, userMessageError(platformi18n.Message(ctx, "PermissionAPIMetadataUnavailable"))
	}

	apiIDsByPolicy := make(map[string]uint64, len(allDBAPIs))
	for _, api := range allDBAPIs {
		apiIDsByPolicy[api.FullPath()] = api.ID
	}

	seenIDs := make(map[uint64]struct{}, len(policies))
	apiIDs := make([]uint64, 0, len(policies))
	for _, policy := range policies {
		id, ok := apiIDsByPolicy[policy.FullPath()]
		if !ok {
			continue
		}
		if _, ok = seenIDs[id]; ok {
			continue
		}
		seenIDs[id] = struct{}{}
		apiIDs = append(apiIDs, id)
	}

	return apiIDs, nil
}

func (uc *PermissionUseCase) skipContractCheck() bool {
	return uc.conf != nil && uc.conf.App().SkipPermissionContractCheck
}

func (uc *PermissionUseCase) loadPermissionContract(ctx context.Context) (map[string]MenuContract, error) {
	if uc.assets == nil {
		return nil, fmt.Errorf("加载嵌入的前端权限契约失败: frontend assets not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := fs.ReadFile(uc.assets, permissionContractPath)
	if err != nil {
		return nil, fmt.Errorf("加载嵌入的前端权限契约失败: %w", err)
	}

	var contract map[string]MenuContract
	if err := json.Unmarshal(data, &contract); err != nil {
		return nil, fmt.Errorf("解析前端权限契约失败: %w", err)
	}
	return contract, nil
}
