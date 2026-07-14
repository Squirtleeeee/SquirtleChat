package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

func New(addr string) *goredis.Client {
	return goredis.NewClient(&goredis.Options{Addr: addr})
}

func Ping(ctx context.Context, c *goredis.Client) error {
	return c.Ping(ctx).Err()
}
