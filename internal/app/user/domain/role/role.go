package role

import (
	"strconv"
	"strings"
)

const (
	// BuiltIn marks a built-in role.
	BuiltIn int32 = 1
	// NonBuiltIn marks a normal editable role.
	NonBuiltIn int32 = 2
	// TypePlatform is the platform role type.
	TypePlatform int32 = 1
)

type Role struct {
	ID          uint64
	Name        string
	Type        int32
	BuiltIn     int32
	DataPerm    int32
	OwnerUserID uint64
	OwnerDeptID uint64
	Uses        string
	ViewMenus   string
	Desc        string
	Policies    []PermissionPolicy
}

type PermissionPolicy struct {
	Service string
	Method  string
}

// NormalizePolicies removes empty and duplicate API permission policies.
func NormalizePolicies(policies []PermissionPolicy) []PermissionPolicy {
	normalized := make([]PermissionPolicy, 0, len(policies))
	seen := make(map[string]struct{}, len(policies))
	for _, policy := range policies {
		service := strings.ToUpper(strings.TrimSpace(policy.Service))
		method := strings.ToUpper(strings.TrimSpace(policy.Method))
		if service == "" || method == "" {
			continue
		}
		key := service + "/" + method
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, PermissionPolicy{
			Service: service,
			Method:  method,
		})
	}
	return normalized
}

// CasbinName returns the role identity used by Casbin.
func CasbinName(id uint64) string {
	return strconv.FormatUint(id, 10)
}
