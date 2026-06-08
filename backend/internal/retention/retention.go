package retention

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
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

	slog.Info("Retention scheduler started",
		"metrics_retention_days", s.metricsRetentionDays,
		"flows_retention_days", s.flowsRetentionDays,
		"alerts_retention_days", s.alertsRetentionDays,
		"interval_hours", 6,
	)

	// Run initial sweep after 30 seconds (give server time to boot)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.prune(context.Background())
		}
	}()

	// Periodic sweep every 6 hours
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
	slog.Info("Retention scheduler stopped")
}

func (s *Scheduler) prune(ctx context.Context) {
	slog.Info("Starting data retention sweep")
	start := time.Now()
	var totalMetrics, totalFlows, totalAlerts, totalSessions int64

	// Prune old metrics
	func() {
		cutoff := time.Now().AddDate(0, 0, -s.metricsRetentionDays)
		count, err := s.db.PruneMetrics(ctx, cutoff)
		if err != nil {
			slog.Error("Failed to prune metrics", "error", err, "cutoff", cutoff)
			return
		}
		totalMetrics = count
		if count > 0 {
			slog.Info("Pruned metrics",
				"deleted", count,
				"cutoff", cutoff,
				"retention_days", s.metricsRetentionDays,
			)
		}
	}()

	// Prune old flow records
	func() {
		cutoff := time.Now().AddDate(0, 0, -s.flowsRetentionDays)
		count, err := s.db.PruneFlows(ctx, cutoff)
		if err != nil {
			slog.Error("Failed to prune flows", "error", err, "cutoff", cutoff)
			return
		}
		totalFlows = count
		if count > 0 {
			slog.Info("Pruned flow records",
				"deleted", count,
				"cutoff", cutoff,
				"retention_days", s.flowsRetentionDays,
			)
		}
	}()

	// Prune resolved alerts
	func() {
		cutoff := time.Now().AddDate(0, 0, -s.alertsRetentionDays)
		count, err := s.db.PruneAlerts(ctx, cutoff)
		if err != nil {
			slog.Error("Failed to prune alerts", "error", err, "cutoff", cutoff)
			return
		}
		totalAlerts = count
		if count > 0 {
			slog.Info("Pruned resolved alerts",
				"deleted", count,
				"cutoff", cutoff,
				"retention_days", s.alertsRetentionDays,
			)
		}
	}()

	// Prune old stopped capture sessions (keep 30 days)
	func() {
		cutoff := time.Now().AddDate(0, 0, -30)
		sessions, err := s.db.GetCaptureSessions(ctx)
		if err != nil {
			slog.Error("Failed to get capture sessions for pruning", "error", err)
			return
		}
		for _, session := range sessions {
			if session.Status != "running" && session.StartedAt.Before(cutoff) {
				stats := models.CaptureSessionStats{
					TotalPackets: session.TotalPackets,
					TotalBytes:   session.TotalBytes,
				}
				if err := s.db.StopCaptureSession(ctx, session.ID, stats); err != nil {
					slog.Warn("Failed to prune capture session", "session_id", session.ID, "error", err)
					continue
				}
				totalSessions++
			}
		}
		if totalSessions > 0 {
			slog.Info("Pruned old capture sessions",
				"deleted", totalSessions,
				"retention_days", 30,
			)
		}
	}()

	duration := time.Since(start)
	slog.Info("Data retention sweep complete",
		"metrics_deleted", totalMetrics,
		"flows_deleted", totalFlows,
		"alerts_deleted", totalAlerts,
		"sessions_deleted", totalSessions,
		"duration_ms", duration.Milliseconds(),
	)
}
