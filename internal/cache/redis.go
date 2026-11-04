//osmi-gateway/internal/cache/redis

package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) AddToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	return r.client.Set(ctx, "blacklist:"+token, "true", ttl).Err()
}

func (r *RedisClient) IsBlacklisted(ctx context.Context, token string) (bool, error) {
	val, err := r.client.Get(ctx, "blacklist:"+token).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
