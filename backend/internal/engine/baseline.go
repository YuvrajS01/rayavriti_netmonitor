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

func (c *BaselineCache) RefreshBaselines(ctx context.Context, db database.Database) {
	devices, err := db.GetEnabledDevices(ctx)
	if err != nil {
		return
	}

	since := time.Now().Add(-24 * time.Hour)
	fields := []string{"response_time", "packet_loss", "cpu_usage", "memory_usage", "bandwidth"}

	for i := range devices {
		for _, field := range fields {
			metrics, err := db.GetMetricsSince(ctx, devices[i].ID, since)
			if err != nil || len(metrics) < 10 {
				continue
			}
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
