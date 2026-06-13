package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceCache_GetDevices_CacheHit(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	// Pre-populate cache
	devices := []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}{
		{ID: 1, Name: "Router-1"},
		{ID: 2, Name: "Switch-1"},
	}
	err := rdb.Set(ctx, "nm:devices:all", devices, 30*time.Second)
	require.NoError(t, err)

	// Read from cache
	var got []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	found, err := rdb.Get(ctx, "nm:devices:all", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Len(t, got, 2)
	assert.Equal(t, "Router-1", got[0].Name)
}

func TestDeviceCache_Invalidation(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	_ = rdb.Set(ctx, "nm:devices:all", "data", 30*time.Second)
	_ = rdb.Set(ctx, "nm:devices:enabled", "data", 30*time.Second)

	exists, _ := rdb.Exists(ctx, "nm:devices:all")
	assert.True(t, exists)

	_ = rdb.Del(ctx, "nm:devices:all", "nm:devices:enabled")

	exists, _ = rdb.Exists(ctx, "nm:devices:all")
	assert.False(t, exists)
	exists, _ = rdb.Exists(ctx, "nm:devices:enabled")
	assert.False(t, exists)
}

func TestStatsCache_DashboardStats(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	stats := map[string]any{
		"total_devices":   10,
		"online_devices":  8,
		"offline_devices": 2,
		"active_alerts":   3,
		"total_metrics":   1500,
	}

	// Cache miss
	var got map[string]any
	found, _ := rdb.Get(ctx, "nm:stats:dashboard", &got)
	assert.False(t, found)

	// Set cache
	err := rdb.Set(ctx, "nm:stats:dashboard", stats, 15*time.Second)
	require.NoError(t, err)

	// Cache hit
	found, err = rdb.Get(ctx, "nm:stats:dashboard", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, float64(10), got["total_devices"])
	assert.Equal(t, float64(8), got["online_devices"])
}

func TestStatsCache_AlertCounts(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	type AlertCounts struct {
		Active       int `json:"active"`
		Acknowledged int `json:"acknowledged"`
		Resolved     int `json:"resolved"`
	}

	counts := AlertCounts{Active: 3, Acknowledged: 1, Resolved: 5}
	_ = rdb.Set(ctx, "nm:alerts:counts", counts, 15*time.Second)

	var got AlertCounts
	found, err := rdb.Get(ctx, "nm:alerts:counts", &got)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, 3, got.Active)
	assert.Equal(t, 1, got.Acknowledged)
	assert.Equal(t, 5, got.Resolved)
}

func TestPubSubBridge_PublishSubscribe(t *testing.T) {
	rdb, _ := setupTestRedis(t)
	ctx := context.Background()

	var received WSMessage
	done := make(chan struct{})

	bridge := NewPubSubBridge(rdb, func(msg WSMessage) {
		received = msg
		close(done)
	})

	go bridge.Subscribe(ctx)

	// Wait for subscriber to connect
	time.Sleep(50 * time.Millisecond)

	msg := WSMessage{Type: "metric:update", Data: map[string]any{"device_id": 1}}
	err := bridge.Publish(ctx, msg)
	require.NoError(t, err)

	select {
	case <-done:
		assert.Equal(t, "metric:update", received.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pub/sub message")
	}
}
