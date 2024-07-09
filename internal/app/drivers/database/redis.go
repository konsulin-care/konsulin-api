package database

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func NewRedisClient(driverConfig *config.DriverConfig, log *logrus.Logger) *redis.Client {
	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%s", driverConfig.Redis.Port),
		Password: driverConfig.Redis.Password,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	return rdb
}
