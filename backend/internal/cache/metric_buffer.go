package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	metricspkg "github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/redis/go-redis/v9"
)

const metricsBufferKey = "nm:buffer:metrics"

type MetricBuffer struct {
	rdb           *Redis
	db            database.Database
	batchSize     int
	flushInterval time.Duration
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewMetricBuffer(rdb *Redis, db database.Database, batchSize int, flushInterval time.Duration) *MetricBuffer {
	return &MetricBuffer{
		rdb:           rdb,
		db:            db,
		batchSize:     batchSize,
		flushInterval: flushInterval,
	}
}

func (b *MetricBuffer) Push(ctx context.Context, m *models.Metric) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return b.rdb.Client().RPush(ctx, metricsBufferKey, data).Err()
}

// Len returns the number of metrics waiting in Redis to be persisted.
func (b *MetricBuffer) Len(ctx context.Context) (int64, error) {
	return b.rdb.Client().LLen(ctx, metricsBufferKey).Result()
}

func (b *MetricBuffer) Start(ctx context.Context) {
	ctx, b.cancel = context.WithCancel(ctx)
	b.wg.Add(1)
	go b.flushLoop(ctx) //nolint:gosec // ctx is already derived from Start's parameter
	slog.Info("Metric buffer started", "batch_size", b.batchSize, "flush_interval", b.flushInterval)
}

func (b *MetricBuffer) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	slog.Info("Metric buffer stopped")
}

// maxBatchesPerTick bounds one drain cycle so a large backlog is worked
// down quickly without starving the rest of the process.
const maxBatchesPerTick = 50

// deadLetterKey holds metrics the database rejected permanently, capped at
// deadLetterMax entries, for inspection instead of endless retries.
const (
	deadLetterKey = "nm:metrics:deadletter"
	deadLetterMax = 10000
)

func (b *MetricBuffer) flushLoop(ctx context.Context) {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			drainCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			b.drain(drainCtx)
			cancel()
			return
		case <-ticker.C:
			b.drain(ctx)
		}
	}
}

// drain flushes batches until the buffer is empty, a transient failure
// occurs, or maxBatchesPerTick is reached. A single fixed-size pop per tick
// capped throughput at batchSize/flushInterval (B2).
func (b *MetricBuffer) drain(ctx context.Context) {
	for i := 0; i < maxBatchesPerTick; i++ {
		popped, ok := b.flush(ctx)
		if !ok || popped < b.batchSize {
			return
		}
	}
}

// flush persists one batch. It returns how many items were popped and
// whether the cycle should continue (false after a transient failure).
func (b *MetricBuffer) flush(ctx context.Context) (int, bool) {
	items, err := b.rdb.Client().LPopCount(ctx, metricsBufferKey, b.batchSize).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		slog.Warn("MetricBuffer: failed to pop from Redis", "error", err)
		return 0, false
	}
	if len(items) == 0 {
		return 0, true
	}

	metrics := make([]*models.Metric, 0, len(items))
	for _, raw := range items {
		var m models.Metric
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			slog.Warn("Failed to unmarshal buffered metric", "error", err)
			continue
		}
		metrics = append(metrics, &m)
	}
	if len(metrics) == 0 {
		return len(items), true
	}

	err = b.db.RecordMetricsBatch(ctx, metrics)
	if err == nil {
		metricspkg.MetricBufferFlushed.Add(float64(len(metrics)))
		return len(items), true
	}
	if !isPermanentDBError(err) {
		// Connection or server trouble: put the whole batch back and stop
		// this cycle rather than retrying row by row against a sick DB.
		slog.Warn("Metric batch insert failed, re-queueing", "error", err, "count", len(metrics))
		b.requeue(ctx, metrics)
		return len(items), false
	}

	// A row in the batch is bad (e.g. its device was deleted). Insert
	// individually to isolate it; bad rows are dead-lettered, not retried
	// forever, and transient failures are re-queued.
	var toRequeue, toDeadLetter []*models.Metric
	for _, m := range metrics {
		switch dbErr := b.db.RecordMetric(ctx, m); {
		case dbErr == nil:
			metricspkg.MetricBufferFlushed.Inc()
		case isPermanentDBError(dbErr):
			toDeadLetter = append(toDeadLetter, m)
		default:
			toRequeue = append(toRequeue, m)
		}
	}
	b.requeue(ctx, toRequeue)
	b.deadLetter(ctx, toDeadLetter)
	return len(items), len(toRequeue) == 0
}

// isPermanentDBError reports whether retrying err can never succeed: data
// exceptions (SQLSTATE class 22) and integrity violations (class 23).
func isPermanentDBError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || len(pgErr.Code) < 2 {
		return false
	}
	class := pgErr.Code[:2]
	return class == "22" || class == "23"
}

func (b *MetricBuffer) deadLetter(ctx context.Context, metrics []*models.Metric) {
	if len(metrics) == 0 {
		return
	}
	values := make([]interface{}, 0, len(metrics))
	for _, m := range metrics {
		if raw, err := json.Marshal(m); err == nil {
			values = append(values, string(raw))
		}
	}
	pipe := b.rdb.Client().TxPipeline()
	pipe.LPush(ctx, deadLetterKey, values...)
	pipe.LTrim(ctx, deadLetterKey, 0, deadLetterMax-1)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("Failed to dead-letter metrics", "count", len(values), "error", err)
		return
	}
	metricspkg.MetricBufferDeadLettered.Add(float64(len(values)))
	slog.Warn("Dead-lettered metrics rejected by the database", "count", len(values), "key", deadLetterKey)
}

// requeue pushes the given metrics back to the head of the Redis list so they
// are retried on the next flush cycle instead of being lost.
func (b *MetricBuffer) requeue(ctx context.Context, metrics []*models.Metric) {
	if len(metrics) == 0 {
		return
	}
	failed := make([]string, 0, len(metrics))
	for _, m := range metrics {
		raw, err := json.Marshal(m)
		if err != nil {
			slog.Warn("Failed to marshal metric for re-queue", "device_id", m.DeviceID, "error", err)
			continue
		}
		failed = append(failed, string(raw))
	}
	if len(failed) == 0 {
		return
	}
	values := make([]interface{}, len(failed))
	for i, v := range failed {
		values[i] = v
	}
	if err := b.rdb.Client().LPush(ctx, metricsBufferKey, values...).Err(); err != nil {
		slog.Warn("Failed to re-queue metrics to Redis", "count", len(failed), "error", err)
	}
}
