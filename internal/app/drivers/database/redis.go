package database

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(driverConfig *config.DriverConfig) (*redis.Client, error) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", driverConfig.Redis.Host, driverConfig.Redis.Port),
		Password: driverConfig.Redis.Password,
	})

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, err
	}
	return rdb, nil
}
