package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit checks if the identifier has exceeded the rate limit.
// Uses Redis sorted sets for a sliding window counter.
// Returns (allowed bool, remaining int, resetAt time.Time, error).
func (r *Redis) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
	now := time.Now()
	windowStart := now.Add(-window)

	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMicro()))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMicro()), Member: now.UnixMicro()})
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, limit, now.Add(window), err // fail-open
	}

	count := int(countCmd.Val())
	allowed := count < limit
	remaining := limit - count - 1
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, now.Add(window), nil
}
