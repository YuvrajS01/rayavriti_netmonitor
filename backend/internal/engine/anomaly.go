package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

type AnomalyEngine struct {
	db            database.Database
	logger        *slog.Logger
	scorer        *HealthScorer
	baselineCache *BaselineCache
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewAnomalyEngine(db database.Database, logger *slog.Logger) *AnomalyEngine {
	if logger == nil {
		logger = slog.Default()
	}
	return &AnomalyEngine{
		db:            db,
		logger:        logger,
		scorer:        NewHealthScorer(db, logger),
		baselineCache: NewBaselineCache(15 * time.Minute),
	}
}

func (e *AnomalyEngine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.baselineCache.RefreshBaselines(ctx, e.db)
				if err := e.scorer.ComputeAll(ctx); err != nil {
					e.logger.Error("health score computation failed", "error", err)
				}
			}
		}
	}()
}

func (e *AnomalyEngine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

func (e *AnomalyEngine) GetBaseline(deviceID int64, field string) *AnomalyBaseline {
	return e.baselineCache.Get(deviceID, field)
}
