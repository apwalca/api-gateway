package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"myproject/api-gateway/internal/logger"
)

type RedisCache struct {
	client  *redis.Client
	logger  *logger.Logger
	maxSize int
}

func NewRedisClient(addr, password string, db, timeout int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("редис не отвечает: %w", err)
	}

	return client, nil
}

func NewRedisCache(client *redis.Client, log *logger.Logger, maxSize int) *RedisCache {
	return &RedisCache{
		client:  client,
		logger:  log,
		maxSize: maxSize,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) []byte {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		c.logger.Error("ошибка получения кеша", "key", key, "error", err)
		return nil
	}
	return val
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		c.logger.Error("ошибка сохранения в кеш", "key", key, "error", err)
	}
}

func (c *RedisCache) Delete(ctx context.Context, key string) {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Error("ошибка удаления кеша", "key", key, "error", err)
	}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
