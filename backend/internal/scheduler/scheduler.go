package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type Scheduler struct {
	db        database.Database
	registry  *collectors.Registry
	hub       *websocket.Hub
	interval  time.Duration
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func New(db database.Database, reg *collectors.Registry, hub *websocket.Hub, intervalSec int) *Scheduler {
	return &Scheduler{db: db, registry: reg, hub: hub, interval: time.Duration(intervalSec) * time.Second}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runOnce(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) runOnce(ctx context.Context) {
	devices, err := s.db.GetDevices(ctx)
	if err != nil {
		return
	}
	for _, d := range devices {
		if !d.Enabled {
			continue
		}
		go s.collectOne(context.Background(), d)
	}
}

func (s *Scheduler) collectOne(ctx context.Context, device models.Device) {
	collector, ok := s.registry.Get(device.Protocol)
	if !ok {
		return
	}
	result, err := collector.Collect(ctx, &device)
	if err != nil || result == nil {
		return
	}
	metric := &models.Metric{
		DeviceID:     device.ID,
		Timestamp:    time.Now(),
		Status:       result.Status,
		ResponseTime: result.ResponseTime,
		PacketLoss:   result.PacketLoss,
		CPUUsage:     result.CPUUsage,
		MemoryUsage:  result.MemoryUsage,
		Bandwidth:    result.Bandwidth,
		Details:      result.Details,
	}
	_ = s.db.RecordMetric(ctx, metric)
	if pg, ok := s.db.(interface{ UpdateDeviceStatus(context.Context, int64, string) error }); ok {
		_ = pg.UpdateDeviceStatus(ctx, device.ID, result.Status)
	}
	s.hub.Broadcast(websocket.Message{Type: websocket.EventMetricUpdate, Data: metric})
}
