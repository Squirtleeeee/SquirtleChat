package database

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func OpenRedis(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

func PingRedis(ctx context.Context, rdb *redis.Client) error {
	return rdb.Ping(ctx).Err()
}
