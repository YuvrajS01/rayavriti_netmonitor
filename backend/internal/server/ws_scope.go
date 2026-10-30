package server

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

// messageDeviceID extracts the device a WebSocket event refers to. Payloads
// are either maps (with "deviceId" or legacy "device_id", holding Go ints
// locally or float64 after a Redis round-trip) or model structs, so the
// lookup has to tolerate all of those shapes.
func messageDeviceID(data any) int64 {
	if m, ok := data.(map[string]any); ok {
		for _, key := range []string{"deviceId", "device_id"} {
			if id := toInt64(m[key]); id > 0 {
				return id
			}
		}
		return 0
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	var probe struct {
		DeviceID  *int64 `json:"deviceId"`
		DeviceID2 *int64 `json:"device_id"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return 0
	}
	if probe.DeviceID != nil {
		return *probe.DeviceID
	}
	if probe.DeviceID2 != nil {
		return *probe.DeviceID2
	}
	return 0
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case *int64:
		if n != nil {
			return *n
		}
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// deviceLocationCache memoises device→location lookups for the WebSocket
// scope filter, which runs inside the hub loop for every scoped client and
// must not hit the database on each message.
type deviceLocationCache struct {
	pool *pgxpool.Pool
	ttl  time.Duration
	mu   sync.Mutex
	m    map[int64]cachedLocation
}

type cachedLocation struct {
	locationID int64
	expires    time.Time
}

func newDeviceLocationCache(pool *pgxpool.Pool, ttl time.Duration) *deviceLocationCache {
	return &deviceLocationCache{pool: pool, ttl: ttl, m: map[int64]cachedLocation{}}
}

func (c *deviceLocationCache) locationOf(deviceID int64) int64 {
	now := time.Now()
	c.mu.Lock()
	if v, ok := c.m[deviceID]; ok && now.Before(v.expires) {
		c.mu.Unlock()
		return v.locationID
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var locationID int64
	if err := c.pool.QueryRow(ctx,
		"SELECT COALESCE(location_id, 0) FROM devices WHERE id = $1", deviceID).Scan(&locationID); err != nil {
		return 0
	}
	c.mu.Lock()
	c.m[deviceID] = cachedLocation{locationID: locationID, expires: now.Add(c.ttl)}
	c.mu.Unlock()
	return locationID
}

// isDeviceEvent reports whether an event type is scoped by device.
func isDeviceEvent(t websocket.EventType) bool {
	switch t {
	case websocket.EventMetricUpdate, websocket.EventAlertTriggered,
		websocket.EventAlertUpdated, websocket.EventAlertResolved,
		websocket.EventDeviceStatus:
		return true
	}
	return false
}
