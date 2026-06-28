package cache

import (
	"context"
	"fmt"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	stdjetcache "github.com/mgtv-tech/jetcache-go"
)

// AuthSnapshotCache removes auth snapshot cache entries.
type AuthSnapshotCache struct {
	cache stdjetcache.Cache
}

var _ application.AuthSnapshotCache = (*AuthSnapshotCache)(nil)

// NewAuthSnapshotCache creates the auth snapshot cache adapter.
func NewAuthSnapshotCache(component *jetcache.Component) *AuthSnapshotCache {
	if component == nil {
		return &AuthSnapshotCache{}
	}
	return &AuthSnapshotCache{cache: component.Cache()}
}

func (c *AuthSnapshotCache) DeleteUser(ctx context.Context, userID uint64) error {
	if c == nil || c.cache == nil || userID == 0 {
		return nil
	}
	return c.cache.Delete(ctx, authUserSnapshotCacheKey(userID))
}

func authUserSnapshotCacheKey(userID uint64) string {
	return fmt.Sprintf("%s:auth:user:snapshot:%d", defaults.RedisKeyPrefix, userID)
}
