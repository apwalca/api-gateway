package ratelimit

import (
	"context"

	"github.com/redis/go-redis/v9"
	"myproject/api-gateway/internal/logger"
)

type TokenBucket struct {
	client   *redis.Client
	logger   *logger.Logger
	failOpen bool
}

func NewTokenBucket(client *redis.Client, log *logger.Logger, failOpen bool) *TokenBucket {
	return &TokenBucket{
		client:   client,
		logger:   log,
		failOpen: failOpen,
	}
}

func (l *TokenBucket) Allow(ctx context.Context, key string, rate int, windowSec int) (bool, error) {
	bucketKey := "ratelimit:" + key

	script := redis.NewScript(`
		local current = redis.call('GET', KEYS[1])
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])

		if not current then
			redis.call('SET', KEYS[1], limit-1, 'EX', window)
			return 1
		end

		current = tonumber(current)
		if current > 0 then
			redis.call('DECR', KEYS[1])
			return 1
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.client, []string{bucketKey}, rate, windowSec).Int()
	if err != nil {
		l.logger.Error("ошибка лимитера", "key", key, "error", err)
		if l.failOpen {
			return true, err
		}
		return false, err
	}

	return result == 1, nil
}
