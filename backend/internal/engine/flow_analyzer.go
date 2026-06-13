package engine

import (
	"context"
	"log/slog"
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
	go fa.run(ctx)
}

func (fa *FlowAnalyzer) Stop() {
	if fa.cancel != nil {
		fa.cancel()
	}
	fa.wg.Wait()
}

func (fa *FlowAnalyzer) run(ctx context.Context) {
	defer fa.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var pending []models.Flow

	flush := func() {
		if len(pending) == 0 {
			return
		}
		if err := fa.db.RecordFlows(ctx, pending); err != nil {
			slog.Warn("FlowAnalyzer: failed to persist flows", "error", err, "count", len(pending))
		}
		pending = pending[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case batch := <-fa.flowCh:
			pending = append(pending, batch...)
			if len(pending) >= 500 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
