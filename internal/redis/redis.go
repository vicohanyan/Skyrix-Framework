package db

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Options struct {
	Host     string
	Port     int
	Password string
	DB       int
}

func InitRedis(cfg Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return rdb, nil
}
