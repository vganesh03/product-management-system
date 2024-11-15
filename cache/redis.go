package cache

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

func Connect() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return client
}
