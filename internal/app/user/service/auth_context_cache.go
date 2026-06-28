package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	jetcache "github.com/mgtv-tech/jetcache-go"
	"gorm.io/gorm"
)

const authUserSnapshotTTL = 2 * time.Minute

type authUserSnapshot struct {
	ID         uint64 `json:"id"`
	Username   string `json:"username"`
	UserStatus int32  `json:"userStatus"`
	UserType   int32  `json:"userType"`
	DeptID     uint64 `json:"deptID"`
}

type authSnapshotCache interface {
	Get(ctx context.Context, key string, val *authUserSnapshot) error
	Set(ctx context.Context, key string, val authUserSnapshot, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type jetAuthSnapshotCache struct {
	cache jetcache.Cache
}

func newJetAuthSnapshotCache(cache jetcache.Cache) authSnapshotCache {
	if cache == nil {
		return nil
	}
	return jetAuthSnapshotCache{cache: cache}
}

func (c jetAuthSnapshotCache) Get(ctx context.Context, key string, val *authUserSnapshot) error {
	return c.cache.Get(ctx, key, val)
}

func (c jetAuthSnapshotCache) Set(ctx context.Context, key string, val authUserSnapshot, ttl time.Duration) error {
	return c.cache.Set(ctx, key, jetcache.Value(val), jetcache.TTL(ttl))
}

func (c jetAuthSnapshotCache) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}

type authContextValidator struct {
	cache authSnapshotCache
	user  store.UserInterface
}

func newAuthContextValidator(cache authSnapshotCache, user store.UserInterface) authContextValidator {
	return authContextValidator{
		cache: cache,
		user:  user,
	}
}

func (s *UserService) AuthSnapshotCache() authSnapshotCache {
	if s == nil || s.JetCache == nil {
		return nil
	}
	return newJetAuthSnapshotCache(s.JetCache.Cache())
}

func (v authContextValidator) ValidateAuthContext(ctx context.Context, auth *authsession.AuthContext) error {
	snapshot, err := v.getSnapshot(ctx, auth.UserID)
	if err != nil {
		return err
	}
	if snapshot.UserStatus == store.UserModelStatusInvalid {
		return platformi18n.ErrorNotLogin(ctx, "UserInvalidated", nil)
	}

	auth.Username = snapshot.Username
	auth.UserType = snapshot.UserType
	auth.DeptID = snapshot.DeptID
	auth.Subject = snapshot.Username
	auth.IsBuiltinAdmin = snapshot.Username == store.UserModelUsernameRoot || snapshot.Username == store.UserModelUsernameAdmin

	return nil
}

func (v authContextValidator) getSnapshot(ctx context.Context, userID uint64) (authUserSnapshot, error) {
	key := authUserSnapshotCacheKey(userID)
	if v.cache != nil {
		var snapshot authUserSnapshot
		err := v.cache.Get(ctx, key, &snapshot)
		if err == nil {
			return snapshot, nil
		}
		if !errors.Is(err, jetcache.ErrCacheMiss) && !errors.Is(err, gorm.ErrRecordNotFound) {
			return authUserSnapshot{}, err
		}
	}

	savedUser, err := v.user.GetAuthSnapshot(ctx, userID)
	if err != nil {
		return authUserSnapshot{}, err
	}
	snapshot := authUserSnapshot{
		ID:         savedUser.ID,
		Username:   savedUser.Username,
		UserStatus: savedUser.UserStatus,
		UserType:   savedUser.UserType,
		DeptID:     savedUser.DeptID,
	}
	if v.cache != nil {
		if err = v.cache.Set(ctx, key, snapshot, authUserSnapshotTTL); err != nil {
			return authUserSnapshot{}, err
		}
	}

	return snapshot, nil
}

func authUserSnapshotCacheKey(userID uint64) string {
	return fmt.Sprintf("%s:auth:user:snapshot:%d", defaults.RedisKeyPrefix, userID)
}

func deleteAuthUserSnapshotCache(ctx context.Context, cache authSnapshotCache, userIDs ...uint64) error {
	if cache == nil {
		return nil
	}
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if err := cache.Delete(ctx, authUserSnapshotCacheKey(userID)); err != nil {
			return err
		}
	}

	return nil
}
