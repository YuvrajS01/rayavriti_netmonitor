package reports

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduledRunner struct {
	pool      *pgxpool.Pool
	generator *Generator
	interval  time.Duration
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewScheduledRunner(pool *pgxpool.Pool, generator *Generator, checkInterval time.Duration) *ScheduledRunner {
	if checkInterval <= 0 {
		checkInterval = time.Minute
	}
	return &ScheduledRunner{
		pool:      pool,
		generator: generator,
		interval:  checkInterval,
	}
}

func (r *ScheduledRunner) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.run(ctx)
	}()

	slog.Info("Scheduled report runner started", "interval", r.interval)
}

func (r *ScheduledRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	slog.Info("Scheduled report runner stopped")
}

func (r *ScheduledRunner) run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.checkAndRun(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkAndRun(ctx)
		}
	}
}

func (r *ScheduledRunner) checkAndRun(ctx context.Context) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, report_type, format, schedule_cron, timezone, scope_type, scope_value,
		 recipients, include_charts, lookback_period, custom_from, custom_to, last_run_at
		 FROM scheduled_reports WHERE enabled=true`)
	if err != nil {
		slog.Error("Failed to query scheduled reports", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name, reportType, format, cron, tz, scopeType, scopeValue, lookback string
		var recipients []byte
		var includeCharts bool
		var customFrom, customTo, lastRun *time.Time
		if err := rows.Scan(&id, &name, &reportType, &format, &cron, &tz, &scopeType, &scopeValue,
			&recipients, &includeCharts, &lookback, &customFrom, &customTo, &lastRun); err != nil {
			slog.Error("Failed to scan scheduled report", "error", err)
			continue
		}

		if !r.isDue(lastRun, cron) {
			continue
		}

		periodFrom, periodTo := r.computePeriod(customFrom, customTo, lookback)

		slog.Info("Running scheduled report", "id", id, "name", name, "type", reportType)

		result, err := r.generator.Generate(ctx, GenerateRequest{
			ReportType:        reportType,
			Title:             name,
			Format:            format,
			PeriodFrom:        periodFrom,
			PeriodTo:          periodTo,
			ScopeDesc:         scopeType + ":" + scopeValue,
			Recipients:        string(recipients),
			GeneratedBy:       "scheduler",
			ScheduledReportID: &id,
		})
		if err != nil {
			slog.Error("Failed to generate scheduled report", "id", id, "error", err)
			_, _ = r.pool.Exec(ctx,
				`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='failed' WHERE id=$1`, id)
			continue
		}

		_, _ = r.pool.Exec(ctx,
			`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='success' WHERE id=$1`, id)

		slog.Info("Scheduled report completed", "id", id, "result_id", result.ID, "file", result.FilePath)
	}
}

func (r *ScheduledRunner) isDue(lastRun *time.Time, cronExpr string) bool {
	if lastRun == nil {
		return true
	}

	now := time.Now()
	elapsed := now.Sub(*lastRun)

	switch {
	case containsAny(cronExpr, "*/1 *"):
		return elapsed >= time.Minute
	case containsAny(cronExpr, "*/5 *"):
		return elapsed >= 5*time.Minute
	case containsAny(cronExpr, "*/15 *"):
		return elapsed >= 15*time.Minute
	case containsAny(cronExpr, "*/30 *"):
		return elapsed >= 30*time.Minute
	case containsAny(cronExpr, "0 *"):
		return elapsed >= time.Hour
	case containsAny(cronExpr, "0 0 *"):
		return elapsed >= 24*time.Hour
	case containsAny(cronExpr, "0 0 * * 1"):
		return elapsed >= 7*24*time.Hour
	default:
		return elapsed >= time.Hour
	}
}

func (r *ScheduledRunner) computePeriod(customFrom, customTo *time.Time, lookback string) (time.Time, time.Time) {
	to := time.Now()
	from := to.AddDate(0, 0, -7)

	if customFrom != nil && customTo != nil {
		return *customFrom, *customTo
	}

	switch lookback {
	case "1d":
		from = to.AddDate(0, 0, -1)
	case "7d":
		from = to.AddDate(0, 0, -7)
	case "30d":
		from = to.AddDate(0, 0, -30)
	case "90d":
		from = to.AddDate(0, 0, -90)
	case "1y":
		from = to.AddDate(-1, 0, 0)
	}

	return from, to
}

func containsAny(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
