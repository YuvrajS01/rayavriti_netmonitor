package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type PollJob struct {
	Device     models.Device
	Priority   int
	ScheduleAt time.Time
	Attempt    int
}

type WorkerPoolMetrics struct {
	ActiveWorkers  atomic.Int64
	QueuedCritical atomic.Int64
	QueuedNormal   atomic.Int64
	QueuedLow      atomic.Int64
	Completed      atomic.Int64
	Errors         atomic.Int64
	TotalLatencyMs atomic.Int64
	TotalCompleted atomic.Int64
}

type WorkerPool struct {
	workers    int
	maxWorkers int
	criticalQ  chan PollJob
	normalQ    chan PollJob
	lowQ       chan PollJob
	wg         sync.WaitGroup
	metrics    *WorkerPoolMetrics
	execute    func(context.Context, PollJob) PollResult
	resultFn   func(PollResult)
	cancel     context.CancelFunc
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
		workers:    cfg.WorkerCount,
		maxWorkers: cfg.MaxWorkerCount,
		criticalQ:  make(chan PollJob, cfg.CriticalQueueSize),
		normalQ:    make(chan PollJob, cfg.NormalQueueSize),
		lowQ:       make(chan PollJob, cfg.LowQueueSize),
		metrics:    &WorkerPoolMetrics{},
		execute:    executeFn,
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
	slog.Info("worker pool started", "workers", wp.workers, "maxWorkers", wp.maxWorkers)
}

func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()
	slog.Info("worker pool stopped")
}

func (wp *WorkerPool) Enqueue(job PollJob) {
	switch job.Priority {
	case 0:
		wp.metrics.QueuedCritical.Add(1)
		select {
		case wp.criticalQ <- job:
		default:
			slog.Warn("critical queue full, dropping job", "deviceID", job.Device.ID)
			wp.metrics.QueuedCritical.Add(-1)
		}
	case 1:
		wp.metrics.QueuedNormal.Add(1)
		select {
		case wp.normalQ <- job:
		default:
			slog.Warn("normal queue full, dropping job", "deviceID", job.Device.ID)
			wp.metrics.QueuedNormal.Add(-1)
		}
	default:
		wp.metrics.QueuedLow.Add(1)
		select {
		case wp.lowQ <- job:
		default:
			slog.Warn("low queue full, dropping job", "deviceID", job.Device.ID)
			wp.metrics.QueuedLow.Add(-1)
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

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var job PollJob
		var ok bool

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
		}
	}
}

func (wp *WorkerPool) executeJob(ctx context.Context, job PollJob) {
	start := time.Now()
	result := wp.execute(ctx, job)
	duration := time.Since(start)

	wp.metrics.TotalCompleted.Add(1)
	wp.metrics.TotalLatencyMs.Add(duration.Milliseconds())
	wp.metrics.Completed.Add(1)
	if result.Error != nil {
		wp.metrics.Errors.Add(1)
	}

	if wp.resultFn != nil {
		wp.resultFn(result)
	}
}

type WorkerPoolMetricsSnapshot struct {
	ActiveWorkers  int
	QueuedCritical int
	QueuedNormal   int
	QueuedLow      int
	Completed      int64
	Errors         int64
	AvgLatencyMs   int64
}
