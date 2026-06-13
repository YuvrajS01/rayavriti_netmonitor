package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type DeviceCache struct {
	rdb *Redis
	db  database.Database
}

func (c *DeviceCache) GetDevices(ctx context.Context) ([]models.Device, error) {
	var devices []models.Device
	if found, _ := c.rdb.Get(ctx, "nm:devices:all", &devices); found {
		return devices, nil
	}
	var err error
	devices, err = c.db.GetDevices(ctx)
	if err != nil {
		return nil, err
	}
	_ = c.rdb.Set(ctx, "nm:devices:all", devices, 30*time.Second)
	return devices, nil
}

func (c *DeviceCache) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	var devices []models.Device
	if found, _ := c.rdb.Get(ctx, "nm:devices:enabled", &devices); found {
		return devices, nil
	}
	devices, err := c.db.GetEnabledDevices(ctx)
	if err != nil {
		return nil, err
	}
	_ = c.rdb.Set(ctx, "nm:devices:enabled", devices, 30*time.Second)
	return devices, nil
}

func (c *DeviceCache) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	key := fmt.Sprintf("nm:device:%d", id)
	var device models.Device
	if found, _ := c.rdb.Get(ctx, key, &device); found {
		return &device, nil
	}
	devicePtr, err := c.db.GetDevice(ctx, id)
	if err != nil {
		return nil, err
	}
	if devicePtr != nil {
		_ = c.rdb.Set(ctx, key, devicePtr, 60*time.Second)
	}
	return devicePtr, nil
}

func (c *DeviceCache) InvalidateDevices(ctx context.Context) {
	_ = c.rdb.Del(ctx, "nm:devices:all", "nm:devices:enabled")
	slog.Debug("Device cache invalidated")
}

func (c *DeviceCache) InvalidateDevice(ctx context.Context, id int64) {
	key := fmt.Sprintf("nm:device:%d", id)
	_ = c.rdb.Del(ctx, key)
	c.InvalidateDevices(ctx)
}
