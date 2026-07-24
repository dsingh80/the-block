package redisstore

import "github.com/redis/go-redis/v9"

// NewClient wraps redis.NewClient purely so every call site imports redisstore,
// not go-redis directly -- keeps the concrete client library an implementation
// detail of this one package (guidelines/06-backend-architecture.md).
func NewClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}
