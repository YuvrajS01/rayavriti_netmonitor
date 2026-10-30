// Package retention sweeps plain (non-hypertable) tables that grow without
// bound. Hypertables — metrics, flows, logs, alert/notification history,
// capture packets and the rest — are pruned by TimescaleDB retention
// policies (database.ApplyRetentionPolicies), which drop whole chunks.
package retention

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

// Config sets how many days each plain table keeps. Zero or negative keeps
// rows forever.
type Config struct {
	AlertsDays        int
	HealthHistoryDays int
	CaptureDays       int
}

type Scheduler struct {
	db     database.Database
	cfg    Config
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(db database.Database, cfg Config) *Scheduler {
	return &Scheduler{db: db, cfg: cfg}
}

func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	slog.Info("Retention scheduler started",
		"alerts_retention_days", s.cfg.AlertsDays,
		"health_history_retention_days", s.cfg.HealthHistoryDays,
		"capture_retention_days", s.cfg.CaptureDays,
		"interval_hours", 6,
	)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.prune(ctx)
			case <-ticker.C:
				s.prune(ctx)
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	slog.Info("Retention scheduler stopped")
}

type pruneJob struct {
	name string
	days int
	fn   func(context.Context, time.Time) (int64, error)
}

func (s *Scheduler) prune(ctx context.Context) {
	start := time.Now()
	jobs := []pruneJob{
		{"resolved_alerts", s.cfg.AlertsDays, s.db.PruneAlerts},
		{"health_score_history", s.cfg.HealthHistoryDays, s.db.PruneHealthScoreHistory},
		{"capture_sessions", s.cfg.CaptureDays, s.db.PruneCaptureSessions},
	}
	deleted := map[string]int64{}
	for _, job := range jobs {
		if job.days <= 0 {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -job.days)
		n, err := job.fn(ctx, cutoff)
		if err != nil {
			slog.Error("Retention prune failed", "table", job.name, "cutoff", cutoff, "error", err)
			continue
		}
		deleted[job.name] = n
	}
	slog.Info("Data retention sweep complete", "deleted", deleted, "duration_ms", time.Since(start).Milliseconds())
}
