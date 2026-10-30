package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// A result submitted after an idle flush interval must still be written on
// the next tick, not held until batchSize results accumulate.
func TestResultPipeline_FlushesAfterIdleInterval(t *testing.T) {
	var mu sync.Mutex
	written := 0
	db := &mockDB{recordMetricsBatchFn: func(_ context.Context, m []*models.Metric) error {
		mu.Lock()
		written += len(m)
		mu.Unlock()
		return nil
	}}
	p := NewResultPipeline(ResultPipelineConfig{DB: db, BatchSize: 100, FlushMs: 50})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	time.Sleep(200 * time.Millisecond) // several empty ticks
	p.Submit(PollResult{Device: models.Device{ID: 1}, Status: "up", FinishedAt: time.Now()})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := written
		mu.Unlock()
		if n == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("result submitted after an idle period was not flushed")
}
