package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type PollJob struct {
	Device     models.Device
	Priority   int
	ScheduleAt time.Time
}

type WorkerPoolMetrics struct {
	ActiveWorkers  atomic.Int64
	BusyWorkers    atomic.Int64
	QueuedCritical atomic.Int64
	QueuedNormal   atomic.Int64
	QueuedLow      atomic.Int64
	Completed      atomic.Int64
	Errors         atomic.Int64
	TotalLatencyMs atomic.Int64
	TotalCompleted atomic.Int64
}

type WorkerPool struct {
	workers   int
	criticalQ chan PollJob
	normalQ   chan PollJob
	lowQ      chan PollJob
	wg        sync.WaitGroup
	metrics   *WorkerPoolMetrics
	execute   func(context.Context, PollJob) PollResult
	resultFn  func(PollResult)
	cancel    context.CancelFunc
	// stop tells workers to take no new jobs; in-flight polls finish
	// (up to drainTimeout) before the job context is cancelled.
	stop         chan struct{}
	stopOnce     sync.Once
	drainTimeout time.Duration
}

type PollResult struct {
	Device        models.Device
	CollectResult *collectors.Result
	Status        string
	ResponseMs    float64
	Error         error
	StartedAt     time.Time
	FinishedAt    time.Time
}

type WorkerPoolConfig struct {
	WorkerCount       int
	MaxWorkerCount    int
	CriticalQueueSize int
	NormalQueueSize   int
	LowQueueSize      int
}

func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		WorkerCount:       32,
		MaxWorkerCount:    64,
		CriticalQueueSize: 256,
		NormalQueueSize:   1024,
		LowQueueSize:      512,
	}
}

func NewWorkerPool(cfg WorkerPoolConfig, executeFn func(context.Context, PollJob) PollResult) *WorkerPool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 32
	}
	if cfg.MaxWorkerCount <= 0 {
		cfg.MaxWorkerCount = cfg.WorkerCount * 2
	}
	if cfg.CriticalQueueSize <= 0 {
		cfg.CriticalQueueSize = 256
	}
	if cfg.NormalQueueSize <= 0 {
		cfg.NormalQueueSize = 1024
	}
	if cfg.LowQueueSize <= 0 {
		cfg.LowQueueSize = 512
	}

	return &WorkerPool{
		workers:      cfg.WorkerCount,
		criticalQ:    make(chan PollJob, cfg.CriticalQueueSize),
		normalQ:      make(chan PollJob, cfg.NormalQueueSize),
		lowQ:         make(chan PollJob, cfg.LowQueueSize),
		metrics:      &WorkerPoolMetrics{},
		execute:      executeFn,
		stop:         make(chan struct{}),
		drainTimeout: 20 * time.Second,
	}
}

func (wp *WorkerPool) SetResultHandler(fn func(PollResult)) {
	wp.resultFn = fn
}

func (wp *WorkerPool) Start(ctx context.Context) {
	ctx, wp.cancel = context.WithCancel(ctx)
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
	slog.Info("worker pool started", "workers", wp.workers)
}

// Stop lets in-flight polls finish (bounded by drainTimeout) so their
// results are delivered, then cancels anything still running. Queued jobs
// that haven't started are abandoned.
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() { close(wp.stop) })
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(wp.drainTimeout):
		slog.Warn("worker pool drain timed out, cancelling in-flight polls")
		if wp.cancel != nil {
			wp.cancel()
		}
		<-done
	}
	if wp.cancel != nil {
		wp.cancel()
	}
	slog.Info("worker pool stopped")
}

// Enqueue attempts to add a job to the appropriate priority queue.
// Returns false if the queue is full and the job was dropped (M16 —
// previously returned nothing, so the dispatcher kept re-scheduling).
func (wp *WorkerPool) Enqueue(job PollJob) bool {
	switch job.Priority {
	case 0:
		wp.metrics.QueuedCritical.Add(1)
		select {
		case wp.criticalQ <- job:
			return true
		default:
			slog.Debug("critical queue full, dropping job", "deviceID", job.Device.ID)
			metrics.PollsDropped.WithLabelValues("critical").Inc()
			wp.metrics.QueuedCritical.Add(-1)
			return false
		}
	case 1:
		wp.metrics.QueuedNormal.Add(1)
		select {
		case wp.normalQ <- job:
			return true
		default:
			slog.Debug("normal queue full, dropping job", "deviceID", job.Device.ID)
			metrics.PollsDropped.WithLabelValues("normal").Inc()
			wp.metrics.QueuedNormal.Add(-1)
			return false
		}
	default:
		wp.metrics.QueuedLow.Add(1)
		select {
		case wp.lowQ <- job:
			return true
		default:
			slog.Debug("low queue full, dropping job", "deviceID", job.Device.ID)
			metrics.PollsDropped.WithLabelValues("low").Inc()
			wp.metrics.QueuedLow.Add(-1)
			return false
		}
	}
}

func (wp *WorkerPool) Metrics() WorkerPoolMetricsSnapshot {
	m := wp.metrics
	avgLatencyMs := int64(0)
	if tc := m.TotalCompleted.Load(); tc > 0 {
		avgLatencyMs = m.TotalLatencyMs.Load() / tc
	}
	return WorkerPoolMetricsSnapshot{
		ActiveWorkers:  int(m.ActiveWorkers.Load()),
		BusyWorkers:    int(m.BusyWorkers.Load()),
		QueuedCritical: int(m.QueuedCritical.Load()),
		QueuedNormal:   int(m.QueuedNormal.Load()),
		QueuedLow:      int(m.QueuedLow.Load()),
		Completed:      m.Completed.Load(),
		Errors:         m.Errors.Load(),
		AvgLatencyMs:   avgLatencyMs,
	}
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	wp.metrics.ActiveWorkers.Add(1)
	defer wp.metrics.ActiveWorkers.Add(-1)
	defer wp.wg.Done()

	var fairnessCounter int

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stop:
			return
		default:
		}

		var job PollJob
		var ok bool

		// Every 8th iteration, skip the critical-only fast paths so
		// normal/low jobs get a fair chance even under a sustained burst
		// of critical jobs (M15 — previously critical could starve
		// normal/low indefinitely).
		fairnessCounter++
		fairSkip := fairnessCounter%8 == 0

		if !fairSkip {
			select {
			case job, ok = <-wp.criticalQ:
				if !ok {
					return
				}
				wp.metrics.QueuedCritical.Add(-1)
				wp.executeJob(ctx, job)
				continue
			default:
			}

			select {
			case job, ok = <-wp.criticalQ:
				if !ok {
					return
				}
				wp.metrics.QueuedCritical.Add(-1)
				wp.executeJob(ctx, job)
				continue
			case job, ok = <-wp.normalQ:
				if !ok {
					return
				}
				wp.metrics.QueuedNormal.Add(-1)
				wp.executeJob(ctx, job)
				continue
			default:
			}
		}

		select {
		case job, ok = <-wp.criticalQ:
			if !ok {
				return
			}
			wp.metrics.QueuedCritical.Add(-1)
			wp.executeJob(ctx, job)
			continue
		case job, ok = <-wp.normalQ:
			if !ok {
				return
			}
			wp.metrics.QueuedNormal.Add(-1)
			wp.executeJob(ctx, job)
			continue
		case job, ok = <-wp.lowQ:
			if !ok {
				return
			}
			wp.metrics.QueuedLow.Add(-1)
			wp.executeJob(ctx, job)
			continue
		case <-ctx.Done():
			return
		case <-wp.stop:
			return
		}
	}
}

func (wp *WorkerPool) executeJob(ctx context.Context, job PollJob) {
	start := time.Now()
	wp.metrics.BusyWorkers.Add(1)
	result := wp.safeExecute(ctx, job)
	wp.metrics.BusyWorkers.Add(-1)
	duration := time.Since(start)

	wp.metrics.TotalCompleted.Add(1)
	wp.metrics.TotalLatencyMs.Add(duration.Milliseconds())
	wp.metrics.Completed.Add(1)
	if result.Error != nil {
		wp.metrics.Errors.Add(1)
	}

	if wp.resultFn != nil {
		wp.deliverResult(result)
	}
}

// safeExecute runs the poll job while recovering from any panic so that a
// misbehaving collector can never take down the whole process.
func (wp *WorkerPool) safeExecute(ctx context.Context, job PollJob) (result PollResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in poll job",
				"deviceID", job.Device.ID, "device", job.Device.Name,
				"panic", r, "stack", string(debug.Stack()))
			now := time.Now()
			result = PollResult{
				Device:     job.Device,
				Status:     "down",
				Error:      fmt.Errorf("poll job panic: %v", r),
				StartedAt:  now,
				FinishedAt: now,
			}
		}
	}()
	return wp.execute(ctx, job)
}

// deliverResult forwards a poll result to the registered handler, recovering
// from any panic in the handler so it cannot crash a worker goroutine.
func (wp *WorkerPool) deliverResult(result PollResult) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in result handler",
				"deviceID", result.Device.ID, "device", result.Device.Name,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()
	wp.resultFn(result)
}

type WorkerPoolMetricsSnapshot struct {
	ActiveWorkers  int
	BusyWorkers    int
	QueuedCritical int
	QueuedNormal   int
	QueuedLow      int
	Completed      int64
	Errors         int64
	AvgLatencyMs   int64
}
