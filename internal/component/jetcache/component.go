package jetcache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/gotomicro/ego/core/elog"
	cache "github.com/mgtv-tech/jetcache-go"
	jetcache "github.com/mgtv-tech/jetcache-go"
	"github.com/mgtv-tech/jetcache-go/local"
	"github.com/mgtv-tech/jetcache-go/remote"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// Component jetcache组件
type Component struct {
	name   string
	config *Config
	logger *elog.Component

	cache  jetcache.Cache
	rd     *eredis.Component
	pubSub *redis.PubSub
	done   chan struct{}
}

// newComponent 创建组件
func newComponent(name string, config *Config, logger *elog.Component, rd *eredis.Component) *Component {
	sourceID := lo.Ternary(config.SourceId != "", config.SourceId, ulid.Make().String()) // Unique identifier for this cache instance
	channelName := "syncLocalChannel"

	var pubSub *redis.PubSub
	if config.EnableSyncLocal {
		switch rd.Mode() {
		case eredis.StubMode:
			pubSub = rd.Stub().Subscribe(context.Background(), channelName)
		case eredis.SentinelMode:
			pubSub = rd.Sentinel().Subscribe(context.Background(), channelName)
		case eredis.ClusterMode:
			pubSub = rd.Cluster().Subscribe(context.Background(), channelName)
		case eredis.RingMode:
			pubSub = rd.Ring().Subscribe(context.Background(), channelName)
		}
		if pubSub == nil {
			panic("jetcache: failed to subscribe to syncLocalChannel")
		}
	}

	cacheOptions := []jetcache.Option{
		jetcache.WithName(config.Name),
		jetcache.WithRemote(remote.NewGoRedisV9Adapter(rd.Client())),
		jetcache.WithLocal(local.NewFreeCache(local.Size(config.LocalSize)*local.MB, config.LocalExpiry)),
		jetcache.WithRemoteExpiry(config.RemoteExpiry),
		jetcache.WithNotFoundExpiry(config.NotFoundExpiry),
		jetcache.WithRefreshDuration(config.RefreshDuration),
		jetcache.WithStopRefreshAfterLastAccess(config.StopRefreshAfter),
		jetcache.WithSyncLocal(config.EnableSyncLocal),
		jetcache.WithSourceId(sourceID),
		jetcache.WithErrNotFound(gorm.ErrRecordNotFound),
	}
	if config.EnableSyncLocal {
		cacheOptions = append(cacheOptions, jetcache.WithEventHandler(func(event *jetcache.Event) {
			// Broadcast local cache invalidation for the received keys
			bs, _ := json.Marshal(event)

			rd.Client().Publish(context.Background(), channelName, string(bs))
		}))
	}
	cacheInstance := jetcache.New(cacheOptions...)

	comp := &Component{
		name:   name,
		config: config,
		logger: logger,
		cache:  cacheInstance,
		rd:     rd,
		pubSub: pubSub,
		done:   make(chan struct{}),
	}

	if pubSub != nil {
		go comp.handleSyncLocalEvents(pubSub, cacheInstance, sourceID)
	}

	logger.Info("jetcache component initialized",
		elog.String("name", config.Name),
		elog.Int("localSizeMB", config.LocalSize),
	)

	return comp
}

func (c *Component) handleSyncLocalEvents(pubSub *redis.PubSub, cacheInstance jetcache.Cache, sourceID string) {
	ch := pubSub.Channel()
	for {
		select {
		case <-c.done:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event *cache.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				c.logger.Error("jetcache: failed to unmarshal event", elog.FieldErr(err))
				continue
			}

			// Invalidate local cache for received keys except own events.
			if event.SourceID != sourceID {
				for _, key := range event.Keys {
					cacheInstance.DeleteFromLocalCache(key)
				}
			}
		}
	}
}

// Cache 返回jetcache实例
func (c *Component) Cache() jetcache.Cache {
	return c.cache
}

// GetDelBytes 原子获取并删除远端缓存值.
func (c *Component) GetDelBytes(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := c.rd.Client().GetDel(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("jetcache getdel %s: %w", key, err)
	}
	c.cache.DeleteFromLocalCache(key)
	return value, true, nil
}

// BulkDeleteCache 批量删除缓存
//
// 键为redis格式的通配符
func (c *Component) BulkDeleteCache(ctx context.Context, pattern string) error {
	client := c.rd.Client()
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		if err := c.cache.Delete(ctx, iter.Val()); err != nil {
			return err
		}
	}

	return iter.Err()
}

// Health 健康检查
func (c *Component) Health(ctx context.Context) error {
	// 简单的健康检查：通过执行一个简单的缓存操作来测试连接
	testKey := "health_check_" + c.name
	testValue := "test"

	// 设置测试值
	err := c.cache.Set(ctx, testKey, jetcache.Value(testValue), jetcache.TTL(time.Second*10))
	if err != nil {
		return fmt.Errorf("cache set failed: %v", err)
	}

	// 获取测试值
	var retrievedValue string
	err = c.cache.Get(ctx, testKey, &retrievedValue)
	if err != nil {
		return fmt.Errorf("cache get failed: %v", err)
	}

	if retrievedValue != testValue {
		return fmt.Errorf("cache value mismatch: expected %s, got %s", testValue, retrievedValue)
	}

	// 清理测试值
	err = c.cache.Delete(ctx, testKey)
	if err != nil {
		c.logger.Warn("failed to delete health check key", elog.String("key", testKey), elog.FieldErr(err))
	}

	return nil
}

// Close 关闭组件
func (c *Component) Close() error {
	close(c.done)
	if c.pubSub != nil {
		if err := c.pubSub.Close(); err != nil {
			return err
		}
	}
	if c.cache != nil {
		c.cache.Close()
	}
	return nil
}
