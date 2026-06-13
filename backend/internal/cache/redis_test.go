package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb, err := NewRedis(RedisConfig{
		URL:          "redis://" + mr.Addr(),
		PoolSize:     5,
		MinIdleConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { rdb.Close() })
	return rdb, mr
}

func TestRedis_GetSetDel(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	type testVal struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	// Set
	val := testVal{Name: "hello", Count: 42}
	err := rdb.Set(ctx, "nm:test:key", val, 10*time.Second)
	require.NoError(t, err)

	// Get
	var got testVal
	found, err := rdb.Get(ctx, "nm:test:key", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "hello", got.Name)
	assert.Equal(t, 42, got.Count)

	// Get missing key
	found, err = rdb.Get(ctx, "nm:test:missing", &got)
	require.NoError(t, err)
	assert.False(t, found)

	// Del
	err = rdb.Del(ctx, "nm:test:key")
	require.NoError(t, err)
	found, err = rdb.Get(ctx, "nm:test:key", &got)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestRedis_Exists(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	_ = rdb.Set(ctx, "nm:exists", "yes", time.Minute)

	exists, err := rdb.Exists(ctx, "nm:exists")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = rdb.Exists(ctx, "nm:notexists")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRedis_Ping(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	err := rdb.Ping(context.Background())
	assert.NoError(t, err)
}

func TestRedis_SetExpired(t *testing.T) {
	rdb, mr := setupTestRedis(t)
	ctx := context.Background()

	_ = rdb.Set(ctx, "nm:ttl", "data", 10*time.Second)
	mr.FastForward(11 * time.Second)

	var got string
	found, err := rdb.Get(ctx, "nm:ttl", &got)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestRedis_DelMultiple(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	_ = rdb.Set(ctx, "k1", "v1", time.Minute)
	_ = rdb.Set(ctx, "k2", "v2", time.Minute)
	_ = rdb.Set(ctx, "k3", "v3", time.Minute)

	err := rdb.Del(ctx, "k1", "k2")
	require.NoError(t, err)

	exists, _ := rdb.Exists(ctx, "k1")
	assert.False(t, exists)
	exists, _ = rdb.Exists(ctx, "k2")
	assert.False(t, exists)
	exists, _ = rdb.Exists(ctx, "k3")
	assert.True(t, exists)
}
