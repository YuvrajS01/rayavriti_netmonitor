package retention

import (
	"context"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

type Scheduler struct {
	db                   database.Database
	metricsRetentionDays int
	flowsRetentionDays   int
	alertsRetentionDays  int
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
}

func New(db database.Database, metricsDays, flowsDays, alertsDays int) *Scheduler {
	return &Scheduler{
		db:                   db,
		metricsRetentionDays: metricsDays,
		flowsRetentionDays:   flowsDays,
		alertsRetentionDays:  alertsDays,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.prune(context.Background())
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) prune(ctx context.Context) {
	now := time.Now()
	_, _ = s.db.PruneMetrics(ctx, now.AddDate(0, 0, -s.metricsRetentionDays))
	_, _ = s.db.PruneFlows(ctx, now.AddDate(0, 0, -s.flowsRetentionDays))
	_, _ = s.db.PruneAlerts(ctx, now.AddDate(0, 0, -s.alertsRetentionDays))
}
