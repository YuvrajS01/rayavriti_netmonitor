package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedis_AcquireLock(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	release, err := rdb.AcquireLock(ctx, "test-lock", 10*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Cleanup
	release()
}

func TestRedis_AcquireLock_Contention(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	// First lock should succeed
	release1, err := rdb.AcquireLock(ctx, "contested", 10*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, release1)

	// Second lock on same key should fail (nil release)
	release2, err := rdb.AcquireLock(ctx, "contested", 10*time.Second)
	require.NoError(t, err)
	assert.Nil(t, release2)

	// Release first lock
	release1()

	// Now should succeed
	release3, err := rdb.AcquireLock(ctx, "contested", 10*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, release3)
	release3()
}

func TestRedis_TryLock(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	// TryLock should succeed
	ok, release, err := rdb.TryLock(ctx, "try-lock", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, release)

	// Second TryLock should fail
	ok, release2, err := rdb.TryLock(ctx, "try-lock", 10*time.Second)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, release2)

	release()
}

func TestRedis_LockExpiry(t *testing.T) {
	rdb, mr := setupTestRedis(t)
	ctx := context.Background()

	// Acquire with short TTL
	release, err := rdb.AcquireLock(ctx, "short-lock", 1*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, release)

	// Fast forward past TTL
	mr.FastForward(2 * time.Second)

	// Should be able to acquire again
	release2, err := rdb.AcquireLock(ctx, "short-lock", 10*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, release2)
	release2()
}
