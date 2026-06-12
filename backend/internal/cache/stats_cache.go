package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type StatsCache struct {
	rdb *Redis
	db  database.Database
}

func (c *StatsCache) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	const key = "nm:stats:dashboard"
	var stats map[string]any
	if found, _ := c.rdb.Get(ctx, key, &stats); found {
		return stats, nil
	}
	stats, err := c.db.GetDashboardStats(ctx)
	if err != nil {
		return nil, err
	}
	_ = c.rdb.Set(ctx, key, stats, 15*time.Second)
	return stats, nil
}

func (c *StatsCache) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	const key = "nm:metrics:latest"
	var metrics []models.Metric
	if found, _ := c.rdb.Get(ctx, key, &metrics); found {
		return metrics, nil
	}
	metrics, err := c.db.GetLatestMetrics(ctx)
	if err != nil {
		return nil, err
	}
	_ = c.rdb.Set(ctx, key, metrics, 10*time.Second)
	return metrics, nil
}

func (c *StatsCache) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	key := fmt.Sprintf("nm:metrics:latest:%d", deviceID)
	var metric models.Metric
	if found, _ := c.rdb.Get(ctx, key, &metric); found {
		return &metric, nil
	}
	metricPtr, err := c.db.GetLatestMetricForDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if metricPtr != nil {
		_ = c.rdb.Set(ctx, key, metricPtr, 10*time.Second)
	}
	return metricPtr, nil
}

func (c *StatsCache) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	const key = "nm:alerts:counts"
	var counts models.AlertCounts
	if found, _ := c.rdb.Get(ctx, key, &counts); found {
		return counts, nil
	}
	counts, err := c.db.GetAlertCounts(ctx)
	if err != nil {
		return counts, err
	}
	_ = c.rdb.Set(ctx, key, counts, 15*time.Second)
	return counts, nil
}

func (c *StatsCache) InvalidateMetrics(ctx context.Context) {
	_ = c.rdb.Del(ctx, "nm:metrics:latest")
	slog.Debug("Metrics cache invalidated")
}

func (c *StatsCache) InvalidateMetricForDevice(ctx context.Context, deviceID int64) {
	key := fmt.Sprintf("nm:metrics:latest:%d", deviceID)
	_ = c.rdb.Del(ctx, key)
}

func (c *StatsCache) InvalidateAlertCounts(ctx context.Context) {
	_ = c.rdb.Del(ctx, "nm:alerts:counts")
}
