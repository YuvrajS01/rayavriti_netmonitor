package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type atomicCount struct {
	c atomic.Int64
}

func (a *atomicCount) Add(n int64) { a.c.Add(n) }
func (a *atomicCount) Load() int64 { return a.c.Load() }

func TestPollDispatcher_UnreachableCount(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1}, 0, time.Hour)
	d.Upsert(models.Device{ID: 2}, 0, time.Hour)
	assert.Equal(t, 0, d.UnreachableCount())
	d.RecordFailure(1)
	d.RecordFailure(1)
	d.RecordFailure(1)
	assert.Equal(t, 1, d.UnreachableCount())
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_PausedCount(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1}, 0, time.Hour)
	d.Upsert(models.Device{ID: 2}, 0, time.Hour)
	assert.Equal(t, 0, d.PausedCount())
	d.Pause(1)
	d.Pause(2)
	assert.Equal(t, 2, d.PausedCount())
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_DispatchCorrectPriority(t *testing.T) {
	t.Parallel()
	executed := make(chan int, 10)
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		executed <- job.Priority
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1, DeviceCategory: "switch"}, 0, 30*time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	select {
	case p := <-executed:
		assert.Equal(t, 0, p)
	default:
		t.Fatal("expected job")
	}
	d.Stop()
	wp.Stop()
}

func TestScheduleEntry_EffectiveInterval(t *testing.T) {
	t.Parallel()
	e := &ScheduleEntry{Interval: 10 * time.Second}
	assert.Equal(t, 10*time.Second, e.effectiveInterval())
	e.Failures = 2
	assert.Equal(t, 20*time.Second, e.effectiveInterval())
	e.Failures = 3
	assert.Equal(t, 40*time.Second, e.effectiveInterval())
	e.Failures = 10
	assert.Equal(t, 80*time.Second, e.effectiveInterval())
}

func TestNew_PreservesConfig(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	s := New(db, reg, nil, nil, 60)
	cfg := s.Config()
	// intervalSec controls the default poll interval, not the reconcile interval.
	// ReconcileInterval should always be the default (30s).
	assert.Equal(t, 30*time.Second, cfg.ReconcileInterval)
}

func TestScheduler_Config(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	s := New(db, reg, nil, nil, 30)
	require.NotNil(t, s)
	assert.NotNil(t, s.Config())
}

func TestWorkerPool_NilResultHandler(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(DefaultWorkerPoolConfig(), func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	wp.SetResultHandler(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	wp.Enqueue(PollJob{Device: models.Device{ID: 1}, Priority: 1})
	time.Sleep(100 * time.Millisecond)
	wp.Stop()
}

func TestDeviceStateTracker_Concurrent(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	for i := 0; i < 100; i++ {
		go dst.RecordSuccess(1, time.Millisecond)
		go dst.RecordFailure(2, assert.AnError)
	}
	time.Sleep(50 * time.Millisecond)
	assert.Greater(t, dst.Count(), 0)
}

func TestDependencyTree_GetDescendants_NoChildren(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	assert.Empty(t, dt.GetDescendants(1))
}

func TestDependencyTree_Count(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	assert.Equal(t, 0, dt.Count())
	dt.SetParent(2, 1)
	assert.Equal(t, 1, dt.Count())
	dt.SetParent(3, 1)
	assert.Equal(t, 2, dt.Count())
}

func TestResultPipeline_EmptyBatch(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	p := NewResultPipeline(ResultPipelineConfig{DB: db, BatchSize: 100, FlushMs: 50})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	p.Stop()
}

func TestResultPipeline_FallbackOnBatchError(t *testing.T) {
	t.Parallel()
	var individualCount atomicCount
	db := &mockDB{
		recordMetricsBatchFn: func(ctx context.Context, metrics []*models.Metric) error {
			return assert.AnError
		},
		recordMetricFn: func(ctx context.Context, m *models.Metric) error {
			individualCount.Add(1)
			return nil
		},
	}
	p := NewResultPipeline(ResultPipelineConfig{DB: db, BatchSize: 2, FlushMs: 5000})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Submit(PollResult{Device: models.Device{ID: 1}, Status: "up", FinishedAt: time.Now()})
	p.Submit(PollResult{Device: models.Device{ID: 2}, Status: "up", FinishedAt: time.Now()})
	time.Sleep(100 * time.Millisecond)
	assert.Greater(t, individualCount.Load(), int64(0))
	p.Stop()
}
