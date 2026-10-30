package flowingest

import (
	"context"
	"log/slog"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// FlowWriter persists flow batches (database.Postgres.RecordFlows).
type FlowWriter interface {
	RecordFlows(ctx context.Context, flows []models.Flow) error
}

// batchWriter buffers flows from a bounded queue and writes them in batches,
// flushing when a batch fills or on every ticker tick. Producers never block:
// enqueue drops (and counts) when the queue is full.
type batchWriter struct {
	store        FlowWriter
	queue        chan models.Flow
	batchSize    int
	interval     time.Duration
	writeTimeout time.Duration
}

func newBatchWriter(store FlowWriter, queueSize, batchSize int, interval, writeTimeout time.Duration) *batchWriter {
	return &batchWriter{
		store:        store,
		queue:        make(chan models.Flow, queueSize),
		batchSize:    batchSize,
		interval:     interval,
		writeTimeout: writeTimeout,
	}
}

// enqueue offers f to the writer without blocking; it returns false (and
// counts a drop) when the queue is full.
func (w *batchWriter) enqueue(f models.Flow) bool {
	select {
	case w.queue <- f:
		return true
	default:
		metrics.FlowRecordsDropped.WithLabelValues("queue_full").Inc()
		return false
	}
}

// run writes batches until the queue is closed, then drains it and does a
// final flush. Every write gets its own bounded context, so the final flush
// is not cut short by the shutdown that closed the queue.
func (w *batchWriter) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	pending := make([]models.Flow, 0, w.batchSize)
	for {
		select {
		case f, ok := <-w.queue:
			if !ok {
				w.flush(pending)
				return
			}
			pending = append(pending, f)
			if len(pending) >= w.batchSize {
				w.flush(pending)
				pending = pending[:0]
			}
		case <-ticker.C:
			w.flush(pending)
			pending = pending[:0]
		}
	}
}

func (w *batchWriter) flush(batch []models.Flow) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.writeTimeout)
	defer cancel()
	start := time.Now()
	err := w.store.RecordFlows(ctx, batch)
	metrics.FlowBatchWriteDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		metrics.FlowBatchWriteErrors.Inc()
		metrics.FlowRecordsDropped.WithLabelValues("write_error").Add(float64(len(batch)))
		slog.Warn("flow ingest: batch write failed", "error", err, "count", len(batch))
	}
}
