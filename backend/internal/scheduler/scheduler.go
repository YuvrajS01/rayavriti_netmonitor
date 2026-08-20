package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type SchedulerConfig struct {
	WorkerCount       int
	MaxWorkerCount    int
	CriticalQueueSize int
	NormalQueueSize   int
	LowQueueSize      int
	ReconcileInterval time.Duration
	ResultBatchSize   int
	ResultFlushMs     int
}

func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		WorkerCount:       runtime.NumCPU() * 4,
		MaxWorkerCount:    runtime.NumCPU() * 8,
		CriticalQueueSize: 256,
		NormalQueueSize:   1024,
		LowQueueSize:      512,
		ReconcileInterval: 30 * time.Second,
		ResultBatchSize:   100,
		ResultFlushMs:     2000,
	}
}

type Scheduler struct {
	db       database.Database
	registry *collectors.Registry
	hub      *websocket.Hub
	alertEng *engine.AlertEngine
	buffer   *cache.MetricBuffer
	rdb      *cache.Redis

	pool         *WorkerPool
	dispatcher   *PollDispatcher
	pipeline     *ResultPipeline
	stateTracker *DeviceStateTracker

	cancel context.CancelFunc
	wg     sync.WaitGroup
	config SchedulerConfig

	jobCount atomic.Int64
}

type SchedulerOption func(*Scheduler)

func WithMetricBuffer(buffer *cache.MetricBuffer) SchedulerOption {
	return func(s *Scheduler) { s.buffer = buffer }
}

func WithRedis(rdb *cache.Redis) SchedulerOption {
	return func(s *Scheduler) { s.rdb = rdb }
}

func WithSchedulerConfig(cfg SchedulerConfig) SchedulerOption {
	return func(s *Scheduler) {
		if cfg.WorkerCount > 0 {
			s.config.WorkerCount = cfg.WorkerCount
		}
		if cfg.MaxWorkerCount > 0 {
			s.config.MaxWorkerCount = cfg.MaxWorkerCount
		}
		if cfg.CriticalQueueSize > 0 {
			s.config.CriticalQueueSize = cfg.CriticalQueueSize
		}
		if cfg.NormalQueueSize > 0 {
			s.config.NormalQueueSize = cfg.NormalQueueSize
		}
		if cfg.LowQueueSize > 0 {
			s.config.LowQueueSize = cfg.LowQueueSize
		}
		if cfg.ResultBatchSize > 0 {
			s.config.ResultBatchSize = cfg.ResultBatchSize
		}
		if cfg.ResultFlushMs > 0 {
			s.config.ResultFlushMs = cfg.ResultFlushMs
		}
	}
}

func New(db database.Database, registry *collectors.Registry, hub *websocket.Hub, alertEng *engine.AlertEngine, intervalSec int, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		db:       db,
		registry: registry,
		hub:      hub,
		alertEng: alertEng,
		config:   DefaultSchedulerConfig(),
	}

	// intervalSec controls the default poll interval, NOT the reconcile frequency.
	// ReconcileInterval is set independently via DefaultSchedulerConfig (30s).

	for _, o := range opts {
		o(s)
	}

	// Build components AFTER options have been applied
	wpCfg := WorkerPoolConfig{
		WorkerCount:       s.config.WorkerCount,
		MaxWorkerCount:    s.config.MaxWorkerCount,
		CriticalQueueSize: s.config.CriticalQueueSize,
		NormalQueueSize:   s.config.NormalQueueSize,
		LowQueueSize:      s.config.LowQueueSize,
	}

	s.pool = NewWorkerPool(wpCfg, s.collectAndReturnResult)
	s.dispatcher = NewPollDispatcher(s.pool, time.Duration(intervalSec)*time.Second)
	s.pool.SetResultHandler(s.handlePollResult)

	s.pipeline = NewResultPipeline(ResultPipelineConfig{
		DB:        db,
		Hub:       hub,
		AlertEng:  alertEng,
		Buffer:    s.buffer,
		RDB:       s.rdb,
		BatchSize: s.config.ResultBatchSize,
		FlushMs:   s.config.ResultFlushMs,
	})

	s.stateTracker = NewDeviceStateTracker()

	return s
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	s.pool.Start(ctx)
	s.dispatcher.Start(ctx)
	s.pipeline.Start(ctx)

	devices, err := s.db.GetEnabledDevices(ctx)
	if err != nil {
		slog.Error("failed to fetch enabled devices on start", "error", err)
	} else {
		for _, d := range devices {
			s.scheduleDevice(d)
		}
	}

	s.wg.Add(1)
	go s.reconcileLoop(ctx)

	slog.Info("async scheduler started",
		"workers", s.config.WorkerCount,
		"devices", s.dispatcher.Count(),
		"reconcileInterval", s.config.ReconcileInterval)
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}

	s.pool.Stop()
	s.dispatcher.Stop()
	s.pipeline.Stop()
	s.wg.Wait()

	slog.Info("async scheduler stopped")
}

func (s *Scheduler) JobCount() int {
	return int(s.jobCount.Load())
}

func (s *Scheduler) Config() SchedulerConfig {
	return s.config
}

func (s *Scheduler) scheduleDevice(d models.Device) {
	priority := 1
	if d.Protocol == "snmp" {
		priority = 0
	}
	if d.DeviceCategory == "switch" || d.DeviceCategory == "router" || d.DeviceCategory == "firewall" {
		priority = 0
	}

	interval := time.Duration(d.Interval) * time.Second
	s.dispatcher.Upsert(d, priority, interval)
	s.jobCount.Store(int64(s.dispatcher.Count()))
}

func (s *Scheduler) unscheduleDevice(deviceID int64) {
	s.dispatcher.Remove(deviceID)
	// Clean up state tracker entry so deleted/disabled devices don't
	// accumulate forever in the map (M18).
	s.stateTracker.Remove(deviceID)
	s.jobCount.Store(int64(s.dispatcher.Count()))
}

func (s *Scheduler) reconcileLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcile(ctx)
		}
	}
}

func (s *Scheduler) reconcile(ctx context.Context) {
	devices, err := s.db.GetEnabledDevices(ctx)
	if err != nil {
		slog.Error("reconcile: failed to fetch devices", "error", err)
		return
	}

	currentIDs := make(map[int64]bool, len(devices))
	for _, d := range devices {
		currentIDs[d.ID] = true
		s.scheduleDevice(d)
	}

	// Remove devices that no longer exist or are disabled
	for _, id := range s.dispatcher.DeviceIDs() {
		if !currentIDs[id] {
			slog.Info("device removed or disabled, unscheduling", "device_id", id)
			s.dispatcher.Remove(id)
			s.stateTracker.Remove(id)
		}
	}
	s.jobCount.Store(int64(s.dispatcher.Count()))
}

func (s *Scheduler) collectAndReturnResult(ctx context.Context, job PollJob) PollResult {
	start := time.Now()
	result := PollResult{
		Device:    job.Device,
		StartedAt: start,
	}

	if s.rdb != nil {
		lockKey := fmt.Sprintf("collect:%d", job.Device.ID)
		// Clamp TTL to a sane range so a device with interval=0 doesn't
		// create a lock with no expiry (which would permanently block
		// all other instances from polling it after a crash).
		lockTTL := time.Duration(job.Device.Interval) * time.Second
		if lockTTL < 30*time.Second {
			lockTTL = 30 * time.Second
		}
		if lockTTL > 10*time.Minute {
			lockTTL = 10 * time.Minute
		}
		locked, release, err := s.rdb.TryLock(ctx, lockKey, lockTTL)
		if err != nil {
			// Redis is briefly down: collect anyway rather than skipping
			// all polling. The worst case is a duplicate poll across
			// instances, which is far better than silently halting.
			slog.Warn("distributed lock failed, collecting without lock", "deviceID", job.Device.ID, "error", err)
		} else if !locked {
			result.Error = fmt.Errorf("skipped: could not acquire lock")
			result.FinishedAt = time.Now()
			return result
		} else {
			defer release()
		}
	}

	c, ok := s.registry.Get(job.Device.Protocol)
	if !ok {
		slog.Warn("unknown protocol for device", "deviceID", job.Device.ID, "protocol", job.Device.Protocol)
		result.Error = fmt.Errorf("unknown protocol: %s", job.Device.Protocol)
		result.FinishedAt = time.Now()
		return result
	}

	duration := time.Duration(0)
	timeout := 30 * time.Second
	if meta, ok := c.(collectors.CollectorMeta); ok {
		if t := meta.DefaultTimeout(); t > 0 {
			timeout = t
		}
	}

	collectCtx, collectCancel := context.WithTimeout(ctx, timeout)
	defer collectCancel()

	collectResult, err := c.Collect(collectCtx, &result.Device)

	duration = time.Since(start)

	if err != nil {
		slog.Warn("collection failed", "deviceID", job.Device.ID, "device", job.Device.Name,
			"protocol", job.Device.Protocol, "duration", duration, "error", err)
		result.Error = err
		result.Status = "down"
		result.FinishedAt = time.Now()
		result.ResponseMs = float64(duration.Milliseconds())
		return result
	}

	if collectResult == nil {
		result.Status = "down"
		result.FinishedAt = time.Now()
		result.ResponseMs = float64(duration.Milliseconds())
		slog.Warn("collector returned nil result", "deviceID", job.Device.ID, "device", job.Device.Name)
		return result
	}

	result.Status = collectResult.Status
	if collectResult.ResponseTime != nil {
		result.ResponseMs = *collectResult.ResponseTime
	} else {
		result.ResponseMs = float64(duration.Milliseconds())
	}
	result.FinishedAt = time.Now()

	result.Device.Status = collectResult.Status
	result.CollectResult = collectResult
	if collectResult.Details == nil {
		collectResult.Details = make(map[string]any)
	}
	collectResult.Details["collectDurationMs"] = duration.Milliseconds()

	return result
}

func (s *Scheduler) handlePollResult(pr PollResult) {
	// Skip operational errors that aren't real device failures
	// (e.g., distributed lock contention, unknown protocol)
	if pr.Error != nil && pr.Status == "" {
		return
	}

	if isDownResult(pr) {
		s.dispatcher.RecordFailure(pr.Device.ID)
		s.stateTracker.RecordFailure(pr.Device.ID, pr.Error)
	} else {
		duration := pr.FinishedAt.Sub(pr.StartedAt)
		s.dispatcher.RecordSuccess(pr.Device.ID)
		s.stateTracker.RecordSuccess(pr.Device.ID, duration)
	}

	s.pipeline.Submit(pr)
}

// isDownResult reports whether a poll result represents a device failure.
// A "down" status means the device did not respond, regardless of whether the
// collector surfaced it as an error or returned a plain down result. Treating
// it as a failure drives adaptive backoff and unreachable-state tracking so
// dead devices are polled less aggressively instead of at full rate forever.
func isDownResult(pr PollResult) bool {
	return pr.Error != nil || pr.Status == "down"
}

func (s *Scheduler) StateTracker() *DeviceStateTracker {
	return s.stateTracker
}
