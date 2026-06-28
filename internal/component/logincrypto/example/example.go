//go:build ignore

// 该文件仅作为 logincrypto 组件使用示例，默认不参与构建。
// 如需运行示例，请按项目配置准备 client.redis/client.jetcache，并移除第一行构建标签。

package main

import (
	"context"
	"log"
	"sync"

	"github.com/egoadmin/egoadmin/internal/component/eredis"
	"github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
)

type memoryKeyStore struct {
	mu     sync.Mutex
	active *logincrypto.KeyRecord
}

func (s *memoryKeyStore) GetActive(context.Context) (*logincrypto.KeyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil, false, nil
	}
	return s.active, true, nil
}

func (s *memoryKeyStore) GetByKeyID(_ context.Context, keyID string) (*logincrypto.KeyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil || s.active.KeyID != keyID {
		return nil, false, nil
	}
	return s.active, true, nil
}

func (s *memoryKeyStore) Create(_ context.Context, key *logincrypto.KeyRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = key
	return nil
}

func main() {
	redis := eredis.Load("client.redis").Build()
	cache := jetcache.Load("client.jetcache").Build(jetcache.WithEredis(redis))
	defer func() { _ = cache.Close() }()

	comp := logincrypto.Load("component.logincrypto").Build(
		logincrypto.WithJetCache(cache),
		logincrypto.WithKeyStore(&memoryKeyStore{}),
	)

	challenge, err := comp.ChallengeFor(context.Background(), "admin", "browser-ua", logincrypto.ActionLogin)
	if err != nil {
		log.Fatalf("challenge error: %v", err)
	}
	log.Printf("key=%s challenge=%s algorithm=%s\n", challenge.KeyID, challenge.ChallengeID, challenge.Algorithm)
}
