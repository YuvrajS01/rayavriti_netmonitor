package engine

import (
	"context"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

type AnomalyEngine struct {
	db     database.Database
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewAnomalyEngine(db database.Database) *AnomalyEngine {
	return &AnomalyEngine{db: db}
}

func (e *AnomalyEngine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// TODO: compute health scores
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
