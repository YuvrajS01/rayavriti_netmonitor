package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type Scheduler struct {
	db        database.Database
	registry  *collectors.Registry
	hub       *websocket.Hub
	alertEng  *engine.AlertEngine
	jobs      map[int64]context.CancelFunc
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	jobCount  atomic.Int64
}

func New(db database.Database, reg *collectors.Registry, hub *websocket.Hub, alertEng *engine.AlertEngine, intervalSec int) *Scheduler {
	return &Scheduler{
		db:       db,
		registry: reg,
		hub:      hub,
		alertEng: alertEng,
		jobs:     make(map[int64]context.CancelFunc),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	// Run initial collection for all devices immediately
	devices, err := s.db.GetEnabledDevices(ctx)
	if err != nil {
		slog.Error("Failed to get enabled devices for initial collection", "error", err)
	} else {
		for _, d := range devices {
			s.scheduleDevice(ctx, d)
		}
		slog.Info("Scheduler started", "devices", len(devices))
	}

	// Periodically check for device changes (new devices, interval changes, deleted devices)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reconcile(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Lock()
	for id, cancel := range s.jobs {
		cancel()
		delete(s.jobs, id)
	}
	s.mu.Unlock()
	s.wg.Wait()
	slog.Info("Scheduler stopped")
}

// JobCount returns the number of active scheduled jobs.
func (s *Scheduler) JobCount() int {
	return int(s.jobCount.Load())
}

func (s *Scheduler) scheduleDevice(ctx context.Context, device models.Device) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing job for this device if any
	if cancel, ok := s.jobs[device.ID]; ok {
		cancel()
	}

	interval := time.Duration(device.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	deviceCtx, deviceCancel := context.WithCancel(ctx)
	s.jobs[device.ID] = deviceCancel
	s.jobCount.Store(int64(len(s.jobs)))

	// Run immediately in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.collectOnce(deviceCtx, device)
	}()

	// Then run on interval
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-deviceCtx.Done():
				return
			case <-ticker.C:
				// Re-fetch device to get latest config
				if d, err := s.db.GetDevice(deviceCtx, device.ID); err == nil && d.Enabled {
					s.collectOnce(deviceCtx, *d)
				}
			}
		}
	}()

	slog.Debug("Device scheduled", "device_id", device.ID, "name", device.Name, "interval", interval)
}

func (s *Scheduler) reconcile(ctx context.Context) {
	devices, err := s.db.GetEnabledDevices(ctx)
	if err != nil {
		slog.Warn("Failed to get devices for reconciliation", "error", err)
		return
	}

	// Build set of current device IDs
	activeIDs := make(map[int64]bool, len(devices))
	for _, d := range devices {
		activeIDs[d.ID] = true
		if _, scheduled := s.jobs[d.ID]; !scheduled {
			slog.Info("New device discovered, scheduling", "device_id", d.ID, "name", d.Name)
			s.scheduleDevice(ctx, d)
		}
	}

	// Remove jobs for devices that no longer exist or are disabled
	s.mu.Lock()
	for id, cancel := range s.jobs {
		if !activeIDs[id] {
			slog.Info("Device removed or disabled, unscheduling", "device_id", id)
			cancel()
			delete(s.jobs, id)
		}
	}
	s.jobCount.Store(int64(len(s.jobs)))
	s.mu.Unlock()
}

func (s *Scheduler) collectOnce(ctx context.Context, device models.Device) {
	collector, ok := s.registry.Get(device.Protocol)
	if !ok {
		slog.Warn("No collector for protocol", "protocol", device.Protocol, "device_id", device.ID)
		return
	}

	start := time.Now()
	result, err := collector.Collect(ctx, &device)
	duration := time.Since(start)

	if err != nil {
		slog.Error("Collection failed",
			"device_id", device.ID,
			"device_name", device.Name,
			"protocol", device.Protocol,
			"error", err,
			"duration_ms", duration.Milliseconds(),
		)
		return
	}

	if result == nil {
		result = &collectors.Result{Status: "down"}
	}

	// Determine previous status
	var previousStatus string
	if latest, err := s.db.GetLatestMetrics(ctx); err == nil {
		for _, m := range latest {
			if m.DeviceID == device.ID {
				previousStatus = m.Status
				break
			}
		}
	}

	statusChanged := previousStatus != "" && previousStatus != result.Status

	// Record metric
	metric := &models.Metric{
		DeviceID:     device.ID,
		DeviceName:   device.Name,
		Protocol:     device.Protocol,
		Timestamp:    time.Now(),
		Status:       result.Status,
		ResponseTime: result.ResponseTime,
		PacketLoss:   result.PacketLoss,
		CPUUsage:     result.CPUUsage,
		MemoryUsage:  result.MemoryUsage,
		Bandwidth:    result.Bandwidth,
		Details:      result.Details,
	}

	if err := s.db.RecordMetric(ctx, metric); err != nil {
		slog.Error("Failed to record metric",
			"device_id", device.ID,
			"error", err,
		)
	}

	// Update device status
	if pg, ok := s.db.(interface {
		UpdateDeviceStatus(context.Context, int64, string) error
	}); ok {
		_ = pg.UpdateDeviceStatus(ctx, device.ID, result.Status)
	}

	// Log the collection result
	if statusChanged {
		slog.Info("Device status changed",
			"device_id", device.ID,
			"device_name", device.Name,
			"protocol", device.Protocol,
			"previous_status", previousStatus,
			"new_status", result.Status,
			"duration_ms", duration.Milliseconds(),
		)
	} else {
		slog.Debug("Collection completed",
			"device_id", device.ID,
			"device_name", device.Name,
			"status", result.Status,
			"duration_ms", duration.Milliseconds(),
		)
	}

	// Broadcast metric update via WebSocket
	s.hub.Broadcast(websocket.Message{
		Type: websocket.EventMetricUpdate,
		Data: metric,
	})

	// If status changed, broadcast device status
	if statusChanged {
		s.hub.Broadcast(websocket.Message{
			Type: websocket.EventDeviceStatus,
			Data: map[string]any{
				"device_id":       device.ID,
				"device_name":     device.Name,
				"previous_status": previousStatus,
				"new_status":      result.Status,
			},
		})
	}

	// Evaluate alert rules if alert engine is configured
	if s.alertEng != nil {
		if err := s.alertEng.ProcessMetric(ctx, &device, metric, previousStatus); err != nil {
			slog.Warn("Alert evaluation failed",
				"device_id", device.ID,
				"error", err,
			)
		}
	}
}


