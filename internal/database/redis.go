package database

import (
	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

func NewRedis(cfg *redis.Options) *redis.Client {
	client := redis.NewClient(cfg)

	if _, err := client.Ping().Result(); err != nil {
		zap.L().Fatal("Connection to Redis failed", zap.Error(err), zap.Any("options", cfg))
	}

	return client
}
