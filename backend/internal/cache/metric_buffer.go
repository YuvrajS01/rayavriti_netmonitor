package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
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

func (b *MetricBuffer) flushLoop(ctx context.Context) {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.flush(context.Background())
			return
		case <-ticker.C:
			b.flush(ctx)
		}
	}
}

func (b *MetricBuffer) flush(ctx context.Context) {
	pipe := b.rdb.Client().Pipeline()
	popCmd := pipe.LPopCount(ctx, metricsBufferKey, b.batchSize)
	_, err := pipe.Exec(ctx)
	if err != nil {
		slog.Warn("MetricBuffer: failed to pop from Redis", "error", err)
		return
	}

	items, err := popCmd.Result()
	if err != nil {
		slog.Warn("MetricBuffer: failed to read popped metrics", "error", err)
		return
	}
	if len(items) == 0 {
		return
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

	// Metrics are popped from Redis before persistence; if the DB is down we
	// must re-queue the items that could not be saved so they are retried on
	// the next flush instead of being silently lost.
	if len(metrics) > 0 {
		if err := b.db.RecordMetricsBatch(ctx, metrics); err != nil {
			slog.Warn("Batch insert failed, falling back to individual inserts", "error", err, "count", len(metrics))
			var toRequeue []*models.Metric
			for _, m := range metrics {
				if dbErr := b.db.RecordMetric(ctx, m); dbErr != nil {
					toRequeue = append(toRequeue, m)
				}
			}
			b.requeue(ctx, toRequeue)
		}
	}
	slog.Debug("Flushed metrics batch", "count", len(items))
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
