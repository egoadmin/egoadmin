package permission

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/app/gateway/application"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
)

// PolicyCleaner removes stale permission policies through the user service.
type PolicyCleaner struct {
	auth userclient.InternalAuthService
}

var _ application.PermissionPolicyCleaner = (*PolicyCleaner)(nil)

// NewPolicyCleaner creates a user-service-backed permission policy cleaner.
func NewPolicyCleaner(auth userclient.InternalAuthService) *PolicyCleaner {
	return &PolicyCleaner{auth: auth}
}

func (c *PolicyCleaner) DeletePermissionPolicies(ctx context.Context, policies []permissiondomain.Policy) error {
	if c == nil || c.auth == nil {
		return nil
	}
	normalized := permissiondomain.NormalizePolicies(policies)
	out := make([]userclient.PermissionPolicy, 0, len(normalized))
	for _, policy := range normalized {
		out = append(out, userclient.PermissionPolicy{
			Service: policy.Service,
			Method:  policy.Method,
		})
	}
	return c.auth.DeletePermissionPolicies(ctx, out)
}
