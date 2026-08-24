package engine

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
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

func (c *BaselineCache) RefreshBaselines(ctx context.Context, db database.Database) {
	devices, err := db.GetEnabledDevices(ctx)
	if err != nil {
		return
	}

	activeIDs := make(map[int64]bool, len(devices))
	since := time.Now().Add(-24 * time.Hour)
	fields := []string{"response_time", "packet_loss", "cpu_usage", "memory_usage", "bandwidth"}

	for i := range devices {
		activeIDs[devices[i].ID] = true
		metrics, err := db.GetMetricsSince(ctx, devices[i].ID, since)
		if err != nil || len(metrics) < 10 {
			continue
		}
		// Cap the number of rows processed to avoid memory spikes on
		// devices with very high polling frequency.
		if len(metrics) > 5000 {
			metrics = metrics[len(metrics)-5000:]
		}
		for _, field := range fields {
			floats := extractField(metrics, field)
			if len(floats) < 10 {
				continue
			}
			mean, stddev := computeStats(floats)
			c.Set(devices[i].ID, field, AnomalyBaseline{
				Mean:        mean,
				StdDev:      stddev,
				SampleCount: len(floats),
			})
		}
	}

	// Remove cache entries for devices that no longer exist.
	c.Prune(activeIDs)
}

func extractField(metrics []models.Metric, field string) []float64 {
	var out []float64
	for _, m := range metrics {
		var val *float64
		switch field {
		case "response_time":
			val = m.ResponseTime
		case "packet_loss":
			val = m.PacketLoss
		case "cpu_usage":
			val = m.CPUUsage
		case "memory_usage":
			val = m.MemoryUsage
		case "bandwidth":
			val = m.Bandwidth
		}
		if val != nil {
			out = append(out, *val)
		}
	}
	return out
}

func computeStats(values []float64) (mean, stddev float64) {
	n := float64(len(values))
	if n == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / n

	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	variance := sumSq / n
	stddev = math.Sqrt(variance)

	if stddev < 1e-9 {
		stddev = 1e-9
	}
	return mean, stddev
}
