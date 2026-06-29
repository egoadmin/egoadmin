package localcache

import (
	"fmt"
	"runtime/debug"

	"github.com/coocood/freecache"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	New,
	NewConfigCache,
)

// New 新建缓存
func New() *freecache.Cache {
	cache := freecache.NewCache(100 * 1024 * 1024)
	debug.SetGCPercent(20)

	return cache
}

// ConfigCache 配置缓存
type ConfigCache struct {
	cc *freecache.Cache
}

// NewConfigCache 新建config缓存对象
// 须在freecache初始化后才能安全调用.
func NewConfigCache(cc *freecache.Cache) *ConfigCache {
	return &ConfigCache{
		cc,
	}
}

// genTokenKey 生成配置key.
func (t *ConfigCache) genTokenKey(key string) string {
	return fmt.Sprintf("%sconfig:%s", defaults.RedisKeyPrefix, key)
}

// GetConfig 获取配置.
func (t *ConfigCache) GetConfig(key string) (value string, err error) {
	ckey := t.genTokenKey(key)
	tb, err := t.cc.Get([]byte(ckey))
	value = string(tb)

	return
}

// AddConfig 添加配置.
func (t *ConfigCache) AddConfig(key, value string) (err error) {
	err = t.cc.Set([]byte(t.genTokenKey(key)), []byte(value), -1)

	return
}

// DelConfig 删除配置.
func (t *ConfigCache) DelToken(key string) (affected bool) {
	affected = t.cc.Del([]byte(t.genTokenKey(key)))

	return
}
