package authsession

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient interface {
	Client() redis.Cmdable
}

type indexStore interface {
	SetDeviceSession(ctx context.Context, key string, sessionID string, ttl time.Duration) error
	GetDeviceSession(ctx context.Context, key string) (string, error)
	DeleteDeviceSession(ctx context.Context, key string) error
	AddUserSession(ctx context.Context, key string, sessionID string, score float64, ttl time.Duration) error
	RemoveUserSession(ctx context.Context, key string, sessionID string) error
	ListUserSessions(ctx context.Context, key string) ([]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type redisIndexStore struct {
	client redis.Cmdable
}

func newRedisIndexStore(client redis.Cmdable) indexStore {
	return &redisIndexStore{client: client}
}

func (s *redisIndexStore) SetDeviceSession(ctx context.Context, key string, sessionID string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, sessionID, ttl).Err(); err != nil {
		return fmt.Errorf("set device session: %w", err)
	}
	return nil
}

func (s *redisIndexStore) GetDeviceSession(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", errRecordNotFound
		}
		return "", fmt.Errorf("get device session: %w", err)
	}
	return value, nil
}

func (s *redisIndexStore) DeleteDeviceSession(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete device session: %w", err)
	}
	return nil
}

func (s *redisIndexStore) AddUserSession(ctx context.Context, key string, sessionID string, score float64, ttl time.Duration) error {
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: sessionID})
		pipe.Expire(ctx, key, ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("add user session: %w", err)
	}
	return nil
}

func (s *redisIndexStore) RemoveUserSession(ctx context.Context, key string, sessionID string) error {
	if err := s.client.ZRem(ctx, key, sessionID).Err(); err != nil {
		return fmt.Errorf("remove user session: %w", err)
	}
	return nil
}

func (s *redisIndexStore) ListUserSessions(ctx context.Context, key string) ([]string, error) {
	items, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("list user sessions: %w", err)
	}
	return items, nil
}

func (s *redisIndexStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("expire index: %w", err)
	}
	return nil
}
