package logincrypto

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	compjetcache "github.com/egoadmin/egoadmin/internal/component/jetcache"
	jetcache "github.com/mgtv-tech/jetcache-go"
)

type challengeStore interface {
	Set(ctx context.Context, key string, value ChallengeRecord, ttl time.Duration) error
	Consume(ctx context.Context, key string) (ChallengeRecord, bool, error)
}

type jetChallengeStore struct {
	cache compjetcache.Interface
}

func newJetChallengeStore(cache compjetcache.Interface) challengeStore {
	return &jetChallengeStore{cache: cache}
}

func (s *jetChallengeStore) Set(ctx context.Context, key string, value ChallengeRecord, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal login challenge: %w", err)
	}
	if err = s.cache.Cache().Set(ctx, key, jetcache.Value(raw), jetcache.TTL(ttl), jetcache.SkipLocal(true)); err != nil {
		return fmt.Errorf("set login challenge: %w", err)
	}
	return nil
}

func (s *jetChallengeStore) Consume(ctx context.Context, key string) (ChallengeRecord, bool, error) {
	raw, ok, err := s.cache.GetDelBytes(ctx, key)
	if err != nil {
		return ChallengeRecord{}, false, err
	}
	if !ok {
		return ChallengeRecord{}, false, nil
	}
	var record ChallengeRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		return ChallengeRecord{}, false, fmt.Errorf("unmarshal login challenge: %w", err)
	}
	return record, true, nil
}
