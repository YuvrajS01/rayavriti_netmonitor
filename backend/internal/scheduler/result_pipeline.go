package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type ResultPipeline struct {
	resultCh  chan PollResult
	db        database.Database
	hub       *websocket.Hub
	alertEng  *engine.AlertEngine
	buffer    *cache.MetricBuffer
	rdb       *cache.Redis
	batchSize int
	flushMs   int
	wg        sync.WaitGroup
	cancel    context.CancelFunc
}

type ResultPipelineConfig struct {
	DB        database.Database
	Hub       *websocket.Hub
	AlertEng  *engine.AlertEngine
	Buffer    *cache.MetricBuffer
	RDB       *cache.Redis
	BatchSize int
	FlushMs   int
}

func NewResultPipeline(cfg ResultPipelineConfig) *ResultPipeline {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushMs <= 0 {
		cfg.FlushMs = 2000
	}

	return &ResultPipeline{
		resultCh:  make(chan PollResult, 2048),
		db:        cfg.DB,
		hub:       cfg.Hub,
		alertEng:  cfg.AlertEng,
		buffer:    cfg.Buffer,
		rdb:       cfg.RDB,
		batchSize: cfg.BatchSize,
		flushMs:   cfg.FlushMs,
	}
}

func (rp *ResultPipeline) Start(ctx context.Context) {
	ctx, rp.cancel = context.WithCancel(ctx)
	rp.wg.Add(1)
	go rp.run(ctx)
	slog.Info("result pipeline started", "batchSize", rp.batchSize, "flushMs", rp.flushMs)
}

func (rp *ResultPipeline) Stop() {
	if rp.cancel != nil {
		rp.cancel()
	}
	rp.wg.Wait()
	rp.flush(context.Background())
	slog.Info("result pipeline stopped")
}

func (rp *ResultPipeline) Submit(pr PollResult) {
	select {
	case rp.resultCh <- pr:
	default:
		slog.Warn("result pipeline channel full, dropping result", "deviceID", pr.Device.ID)
	}
}

func (rp *ResultPipeline) run(ctx context.Context) {
	defer rp.wg.Done()

	batch := make([]PollResult, 0, rp.batchSize)
	timer := time.NewTimer(time.Duration(rp.flushMs) * time.Millisecond)
	defer timer.Stop()

	flushBatch := func() {
		if len(batch) > 0 {
			rp.processBatch(ctx, batch)
			batch = batch[:0]
			timer.Reset(time.Duration(rp.flushMs) * time.Millisecond)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch()
			return
		case result := <-rp.resultCh:
			batch = append(batch, result)
			if len(batch) >= rp.batchSize {
				flushBatch()
			}
		case <-timer.C:
			flushBatch()
		}
	}
}

func (rp *ResultPipeline) processBatch(ctx context.Context, batch []PollResult) {
	metrics := make([]*models.Metric, 0, len(batch))
	now := time.Now()

	for _, pr := range batch {
		metric := rp.buildMetric(pr, now)
		metrics = append(metrics, metric)

		if rp.hub != nil {
			rp.broadcast(pr, metric)
		}
	}

	rp.writeMetrics(ctx, metrics)

	if rp.alertEng != nil {
		for i, pr := range batch {
			prevStatus := pr.Device.Status
			_ = rp.alertEng.ProcessMetric(ctx, &pr.Device, metrics[i], prevStatus)
		}
	}
}

func (rp *ResultPipeline) buildMetric(pr PollResult, now time.Time) *models.Metric {
	responseTime := pr.ResponseMs
	status := pr.Status
	if status == "" {
		status = pr.Device.Status
	}

	metric := &models.Metric{
		DeviceID:     pr.Device.ID,
		DeviceName:   pr.Device.Name,
		Protocol:     pr.Device.Protocol,
		Timestamp:    now,
		Status:       status,
		ResponseTime: &responseTime,
	}

	return metric
}

func (rp *ResultPipeline) writeMetrics(ctx context.Context, metrics []*models.Metric) {
	if len(metrics) == 0 {
		return
	}

	if rp.buffer != nil {
		for _, m := range metrics {
			if err := rp.buffer.Push(ctx, m); err != nil {
				_ = rp.db.RecordMetric(ctx, m)
			}
		}
		return
	}

	if bdb, ok := rp.db.(interface {
		RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error
	}); ok {
		if err := bdb.RecordMetricsBatch(ctx, metrics); err != nil {
			slog.Warn("batch insert failed, falling back to individual inserts", "error", err)
			for _, m := range metrics {
				_ = rp.db.RecordMetric(ctx, m)
			}
		}
		return
	}

	for _, m := range metrics {
		_ = rp.db.RecordMetric(ctx, m)
	}
}

func (rp *ResultPipeline) broadcast(pr PollResult, metric *models.Metric) {
	if rp.hub == nil {
		return
	}

	rp.hub.Broadcast(websocket.Message{
		Type: websocket.EventMetricUpdate,
		Data: metric,
	})

	if pr.Device.Status != pr.Status && pr.Status != "" {
		rp.hub.Broadcast(websocket.Message{
			Type: websocket.EventDeviceStatus,
			Data: map[string]any{
				"deviceId": pr.Device.ID,
				"status":   pr.Status,
			},
		})
	}
}

func (rp *ResultPipeline) flush(ctx context.Context) {
	batch := make([]PollResult, 0)
	drain := true
	for drain {
		select {
		case pr := <-rp.resultCh:
			batch = append(batch, pr)
		default:
			drain = false
		}
	}
	if len(batch) > 0 {
		rp.processBatch(ctx, batch)
	}
}