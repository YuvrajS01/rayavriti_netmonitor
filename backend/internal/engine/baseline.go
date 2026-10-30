package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

type BaselineCache struct {
	mu      sync.RWMutex
	entries map[baselineKey]*cachedBaseline
	ttl     time.Duration
}

type baselineKey struct {
	deviceID int64
	field    string
}

type cachedBaseline struct {
	baseline   AnomalyBaseline
	computedAt time.Time
}

func NewBaselineCache(ttl time.Duration) *BaselineCache {
	return &BaselineCache{
		entries: make(map[baselineKey]*cachedBaseline),
		ttl:     ttl,
	}
}

func (c *BaselineCache) Get(deviceID int64, field string) *AnomalyBaseline {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := baselineKey{deviceID: deviceID, field: field}
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.computedAt) > c.ttl {
		return nil
	}
	b := entry.baseline
	return &b
}

func (c *BaselineCache) Set(deviceID int64, field string, b AnomalyBaseline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := baselineKey{deviceID: deviceID, field: field}
	c.entries[key] = &cachedBaseline{baseline: b, computedAt: time.Now()}
}

// Prune removes entries that are either expired or belong to devices no
// longer in the given set of active device IDs. Called after each refresh
// so deleted devices don't leak in the map forever.
func (c *BaselineCache) Prune(activeDeviceIDs map[int64]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if !activeDeviceIDs[key.deviceID] {
			delete(c.entries, key)
		}
	}
}

// minBaselineSamples is the fewest observations a baseline needs to be used.
const minBaselineSamples = 10

// RefreshBaselines recomputes 24h baselines for every enabled device with a
// single aggregate query (B18: previously one full-row query per device).
func (c *BaselineCache) RefreshBaselines(ctx context.Context, db database.Database) {
	baselines, err := db.GetMetricBaselines(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		slog.Warn("baseline refresh failed", "error", err)
		return
	}
	activeIDs := make(map[int64]bool)
	for _, b := range baselines {
		activeIDs[b.DeviceID] = true
		if b.Count < minBaselineSamples {
			continue
		}
		stddev := b.StdDev
		if stddev < 1e-9 {
			stddev = 1e-9
		}
		c.Set(b.DeviceID, b.Field, AnomalyBaseline{Mean: b.Mean, StdDev: stddev, SampleCount: b.Count})
	}
	// Remove cache entries for devices that no longer exist or stopped
	// reporting.
	c.Prune(activeIDs)
}
