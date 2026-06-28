package authsession

import (
	"context"
	"errors"
	"fmt"
	"time"

	jetcache "github.com/mgtv-tech/jetcache-go"
	"gorm.io/gorm"
)

type recordCache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
}

type jetRecordCache struct {
	cache jetcache.Cache
}

func newJetRecordCache(cache jetcache.Cache) recordCache {
	return &jetRecordCache{cache: cache}
}

func (c *jetRecordCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if err := c.cache.Set(ctx, key, jetcache.Value(value), jetcache.TTL(ttl)); err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	return nil
}

func (c *jetRecordCache) Get(ctx context.Context, key string, value any) error {
	if err := c.cache.Get(ctx, key, value); err != nil {
		if errors.Is(err, jetcache.ErrCacheMiss) || errors.Is(err, gorm.ErrRecordNotFound) {
			return errRecordNotFound
		}
		return fmt.Errorf("cache get %s: %w", key, err)
	}
	return nil
}

func (c *jetRecordCache) Delete(ctx context.Context, key string) error {
	if err := c.cache.Delete(ctx, key); err != nil {
		return fmt.Errorf("cache delete %s: %w", key, err)
	}
	return nil
}
