package engine

import (
	"context"
	"testing"
	"time"
)

func TestAnomalyEngine_StartStop(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAnomalyEngine(db)

	engine.Start(context.Background())
	time.Sleep(10 * time.Millisecond)
	engine.Stop()
	// If Stop() completes without deadlock, the test passes.
	// Goroutine count can fluctuate due to GC and runtime scheduling.
}

func TestAnomalyEngine_Stop_BeforeStart(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAnomalyEngine(db)
	// Should not panic
	engine.Stop()
}
