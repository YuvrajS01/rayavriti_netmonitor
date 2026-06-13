package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedis_RateLimit_Allowed(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	allowed, remaining, resetAt, err := rdb.RateLimit(ctx, "nm:rl:test", 5, time.Second)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 4, remaining)
	assert.True(t, resetAt.After(time.Now()))
}

func TestRedis_RateLimit_ExceedsLimit(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	// Exhaust the limit
	for i := 0; i < 5; i++ {
		allowed, _, _, err := rdb.RateLimit(ctx, "nm:rl:exhaust", 5, time.Second)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	// Next request should be denied
	allowed, remaining, _, err := rdb.RateLimit(ctx, "nm:rl:exhaust", 5, time.Second)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, remaining)
}

func TestRedis_RateLimit_WindowExpiry(t *testing.T) {
	rdb, mr := setupTestRedis(t)
	ctx := context.Background()

	// Exhaust limit
	for i := 0; i < 5; i++ {
		_, _, _, err := rdb.RateLimit(ctx, "nm:rl:window", 5, time.Second)
		require.NoError(t, err)
	}

	// Should be denied
	allowed, _, _, _ := rdb.RateLimit(ctx, "nm:rl:window", 5, time.Second)
	assert.False(t, allowed)

	// Fast forward past the window
	mr.FastForward(2 * time.Second)

	// Should be allowed again
	allowed, remaining, _, err := rdb.RateLimit(ctx, "nm:rl:window", 5, time.Second)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 4, remaining)
}

func TestRedis_RateLimit_DifferentKeys(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	// Exhaust key1
	for i := 0; i < 5; i++ {
		_, _, _, _ = rdb.RateLimit(ctx, "nm:rl:key1", 5, time.Second)
	}

	// key2 should still be allowed
	allowed, _, _, err := rdb.RateLimit(ctx, "nm:rl:key2", 5, time.Second)
	require.NoError(t, err)
	assert.True(t, allowed)
}
