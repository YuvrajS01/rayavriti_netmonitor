package cache

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// CachedDatabase wraps a Database with Redis caching.
// It implements the database.Database interface by embedding the real DB
// and overriding hot-path methods with cached versions.
type CachedDatabase struct {
	database.Database
	rdb         *Redis
	deviceCache *DeviceCache
	statsCache  *StatsCache
}

func NewCachedDatabase(db database.Database, rdb *Redis) *CachedDatabase {
	return &CachedDatabase{
		Database:    db,
		rdb:         rdb,
		deviceCache: &DeviceCache{rdb: rdb, db: db},
		statsCache:  &StatsCache{rdb: rdb, db: db},
	}
}

func (c *CachedDatabase) GetDevices(ctx context.Context) ([]models.Device, error) {
	return c.deviceCache.GetDevices(ctx)
}

func (c *CachedDatabase) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	return c.deviceCache.GetEnabledDevices(ctx)
}

func (c *CachedDatabase) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	return c.deviceCache.GetDevice(ctx, id)
}

func (c *CachedDatabase) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	return c.statsCache.GetDashboardStats(ctx)
}

func (c *CachedDatabase) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	return c.statsCache.GetLatestMetrics(ctx)
}

func (c *CachedDatabase) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	return c.statsCache.GetLatestMetricForDevice(ctx, deviceID)
}

func (c *CachedDatabase) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	return c.statsCache.GetAlertCounts(ctx)
}

func (c *CachedDatabase) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	result, err := c.Database.CreateDevice(ctx, d)
	if err == nil {
		c.deviceCache.InvalidateDevices(ctx)
	}
	return result, err
}

func (c *CachedDatabase) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	result, err := c.Database.UpdateDevice(ctx, id, d)
	if err == nil {
		c.deviceCache.InvalidateDevice(ctx, id)
	}
	return result, err
}

func (c *CachedDatabase) DeleteDevice(ctx context.Context, id int64) error {
	err := c.Database.DeleteDevice(ctx, id)
	if err == nil {
		c.deviceCache.InvalidateDevice(ctx, id)
	}
	return err
}

func (c *CachedDatabase) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	err := c.Database.UpdateDeviceStatus(ctx, id, status)
	if err == nil {
		_ = c.rdb.Del(ctx, fmt.Sprintf("nm:device:%d", id))
	}
	return err
}

func (c *CachedDatabase) RecordMetric(ctx context.Context, m *models.Metric) error {
	err := c.Database.RecordMetric(ctx, m)
	if err == nil {
		c.statsCache.InvalidateMetrics(ctx)
		c.statsCache.InvalidateMetricForDevice(ctx, m.DeviceID)
	}
	return err
}

func (c *CachedDatabase) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	result, err := c.Database.CreateAlert(ctx, a)
	if err == nil {
		c.statsCache.InvalidateAlertCounts(ctx)
	}
	return result, err
}

func (c *CachedDatabase) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	err := c.Database.UpdateAlertStatus(ctx, id, status, by)
	if err == nil {
		c.statsCache.InvalidateAlertCounts(ctx)
	}
	return err
}

func (c *CachedDatabase) DeleteAlert(ctx context.Context, id int64) error {
	err := c.Database.DeleteAlert(ctx, id)
	if err == nil {
		c.statsCache.InvalidateAlertCounts(ctx)
	}
	return err
}

func (c *CachedDatabase) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	if f.Search == "" && f.Status == "" && f.Protocol == "" && f.Enabled == nil {
		devices, err := c.deviceCache.GetDevices(ctx)
		if err != nil {
			slog.Debug("Cache miss for filtered devices, falling through to DB", "error", err)
			return c.Database.GetDevicesFiltered(ctx, f)
		}
		total := len(devices)
		if f.Limit > 0 && f.Offset < total {
			end := f.Offset + f.Limit
			if end > total {
				end = total
			}
			devices = devices[f.Offset:end]
		}
		return devices, total, nil
	}
	return c.Database.GetDevicesFiltered(ctx, f)
}
