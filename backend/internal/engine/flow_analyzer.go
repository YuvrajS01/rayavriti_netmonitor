package engine

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// FlowAnalyzer ingests flow records, detects anomalies, and persists them.
type FlowAnalyzer struct {
	db     database.Database
	flowCh chan []models.Flow
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	stopped bool
}

func NewFlowAnalyzer(db database.Database, bufSize int) *FlowAnalyzer {
	if bufSize <= 0 {
		bufSize = 1000
	}
	return &FlowAnalyzer{db: db, flowCh: make(chan []models.Flow, bufSize)}
}

// IngestChannel returns the channel to send flow batches into.
func (fa *FlowAnalyzer) IngestChannel() chan<- []models.Flow { return fa.flowCh }

// Start begins the analysis loop.
func (fa *FlowAnalyzer) Start(ctx context.Context) {
	ctx, fa.cancel = context.WithCancel(ctx)
	fa.wg.Add(1)
	go fa.run(ctx) //nolint:gosec // ctx is the engine's lifecycle context, not a request context
}

func (fa *FlowAnalyzer) Stop() {
	if fa.cancel != nil {
		fa.cancel()
	}
	fa.wg.Wait()
	// Close the channel after the run loop has exited so producers
	// checking the stopped flag can stop sending. Closing before the
	// loop exits would risk a panic on send-to-closed-channel.
	fa.mu.Lock()
	fa.stopped = true
	close(fa.flowCh)
	fa.mu.Unlock()
}

// Submit attempts to send a batch of flows into the analyzer. Returns false
// if the analyzer has been stopped so the caller can discard the batch
// instead of blocking forever on a channel that will never be drained.
func (fa *FlowAnalyzer) Submit(batch []models.Flow) bool {
	fa.mu.Lock()
	if fa.stopped {
		fa.mu.Unlock()
		return false
	}
	fa.mu.Unlock()

	select {
	case fa.flowCh <- batch:
		return true
	default:
		slog.Warn("FlowAnalyzer: flow channel full, dropping batch", "count", len(batch))
		return false
	}
}

func (fa *FlowAnalyzer) run(ctx context.Context) {
	defer fa.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in flow analyzer loop", "panic", r, "stack", string(debug.Stack()))
		}
	}()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var pending []models.Flow

	flush := func(flushCtx context.Context) {
		if len(pending) == 0 {
			return
		}
		if err := fa.db.RecordFlows(flushCtx, pending); err != nil {
			slog.Warn("FlowAnalyzer: failed to persist flows", "error", err, "count", len(pending))
		}
		pending = pending[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// Flush the final batch with a fresh short-timeout context
			// so the insert isn't cancelled by the shutdown context.
			flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			flush(flushCtx)
			cancel()
			return
		case batch, ok := <-fa.flowCh:
			if !ok {
				// Channel closed by Stop(): flush remaining and exit.
				flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				flush(flushCtx)
				cancel()
				return
			}
			pending = append(pending, batch...)
			if len(pending) >= 500 {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}
