package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// SelfMonitor periodically collects runtime metrics and writes them to monitoring DB.
type SelfMonitor struct {
	recorder  *Recorder
	interval  time.Duration
	startTime time.Time
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Optional stat providers — set before calling Start().
	WSConnectionCount   func() int
	CaptureSessionCount func() int
	SchedulerJobCount   func() int
	DBStats             func() (open, idle int, waitCount int64, waitDurationMs float64)
	RequestStats        func() (total, active, errors int64)
}

// NewSelfMonitor creates a self-monitoring goroutine that snapshots app health.
// Default interval is 60 seconds per the implementation plan spec.
func NewSelfMonitor(recorder *Recorder, interval time.Duration) *SelfMonitor {
	if interval == 0 {
		interval = 60 * time.Second
	}
	return &SelfMonitor{
		recorder:  recorder,
		interval:  interval,
		startTime: time.Now(),
	}
}

// Start begins the periodic health snapshot collection.
func (sm *SelfMonitor) Start(ctx context.Context) {
	ctx, sm.cancel = context.WithCancel(ctx)
	sm.wg.Add(1)
	go sm.run(ctx)
}

// Stop stops the self-monitor and waits for the goroutine to finish.
func (sm *SelfMonitor) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
	sm.wg.Wait()
}

func (sm *SelfMonitor) run(ctx context.Context) {
	defer sm.wg.Done()
	ticker := time.NewTicker(sm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.collect(ctx)
		}
	}
}

func (sm *SelfMonitor) collect(ctx context.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := &SystemMetrics{
		UptimeSeconds:   int64(time.Since(sm.startTime).Seconds()),
		GoroutineCount:  runtime.NumGoroutine(),
		HeapAllocBytes:  int64(m.HeapAlloc),
		HeapSysBytes:    int64(m.HeapSys),
		StackInUseBytes: int64(m.StackInuse),
		GCPauseTotalNs:  int64(m.PauseTotalNs),
		GCRuns:          int(m.NumGC),
		GCLastPauseNs:   int64(m.PauseNs[(m.NumGC+255)%256]),
		NumCPU:          runtime.NumCPU(),
		Timestamp:       time.Now(),
	}

	// Collect optional stats from injected providers
	if sm.WSConnectionCount != nil {
		metrics.ActiveWSConnections = sm.WSConnectionCount()
	}
	if sm.CaptureSessionCount != nil {
		metrics.ActiveCaptureSessions = sm.CaptureSessionCount()
	}
	if sm.SchedulerJobCount != nil {
		metrics.SchedulerJobsActive = sm.SchedulerJobCount()
	}
	if sm.DBStats != nil {
		open, idle, waitCount, waitDurationMs := sm.DBStats()
		metrics.DBOpenConnections = open
		metrics.DBIdleConnections = idle
		metrics.DBWaitCount = waitCount
		metrics.DBWaitDurationMs = waitDurationMs
	}
	if sm.RequestStats != nil {
		total, active, errors := sm.RequestStats()
		metrics.RequestsTotal = total
		metrics.RequestsActive = active
		metrics.ErrorsTotal = errors
	}

	if err := sm.recorder.RecordSystem(ctx, metrics); err != nil {
		// Log the error but don't crash — monitoring is best-effort
		_ = err
	}
}
