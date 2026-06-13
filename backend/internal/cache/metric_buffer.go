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
	go b.flushLoop(ctx)
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
		return
	}

	items, _ := popCmd.Result()
	if len(items) == 0 {
		return
	}

	for _, raw := range items {
		var m models.Metric
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			slog.Warn("Failed to unmarshal buffered metric", "error", err)
			continue
		}
		if err := b.db.RecordMetric(ctx, &m); err != nil {
			slog.Warn("Failed to record buffered metric", "device_id", m.DeviceID, "error", err)
		}
	}
	slog.Debug("Flushed metrics batch", "count", len(items))
}
