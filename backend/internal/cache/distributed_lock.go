package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// AcquireLock tries to acquire a named lock with TTL.
// Returns a release function, or nil if lock not acquired.
func (r *Redis) AcquireLock(ctx context.Context, name string, ttl time.Duration) (func(), error) {
	lockKey := "nm:lock:" + name
	lockVal := uuid.New().String()

	ok, err := r.client.SetNX(ctx, lockKey, lockVal, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	release := func() {
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			end
			return 0
		`)
		relCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		script.Run(relCtx, r.client, []string{lockKey}, lockVal)
	}
	return release, nil
}

// TryLock attempts to acquire a lock and returns true if acquired.
func (r *Redis) TryLock(ctx context.Context, name string, ttl time.Duration) (bool, func(), error) {
	lockKey := fmt.Sprintf("nm:lock:%s", name)
	lockVal := uuid.New().String()

	ok, err := r.client.SetNX(ctx, lockKey, lockVal, ttl).Result()
	if err != nil {
		return false, nil, err
	}
	if !ok {
		return false, nil, nil
	}

	release := func() {
		script := redis.NewScript(`
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			end
			return 0
		`)
		relCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		script.Run(relCtx, r.client, []string{lockKey}, lockVal)
	}
	return true, release, nil
}
