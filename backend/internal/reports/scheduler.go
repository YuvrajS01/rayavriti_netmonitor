package reports

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field expressions and descriptors such as
// @daily or @weekly.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

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

type dueReport struct {
	id                       int64
	name, reportType, format string
	scopeType, scopeValue    string
	lookback                 string
	recipients               []byte
	customFrom, customTo     *time.Time
}

func (r *ScheduledRunner) checkAndRun(ctx context.Context) {
	due, err := r.dueReports(ctx, time.Now())
	if err != nil {
		slog.Error("Failed to query scheduled reports", "error", err)
		return
	}

	for _, d := range due {
		periodFrom, periodTo := r.computePeriod(d.customFrom, d.customTo, d.lookback)

		slog.Info("Running scheduled report", "id", d.id, "name", d.name, "type", d.reportType)

		id := d.id
		result, err := r.generator.Generate(ctx, GenerateRequest{
			ReportType:        d.reportType,
			Title:             d.name,
			Format:            d.format,
			PeriodFrom:        periodFrom,
			PeriodTo:          periodTo,
			ScopeDesc:         d.scopeType + ":" + d.scopeValue,
			Recipients:        string(d.recipients),
			GeneratedBy:       "scheduler",
			ScheduledReportID: &id,
		})
		if err != nil {
			slog.Error("Failed to generate scheduled report", "id", d.id, "error", err)
			_, _ = r.pool.Exec(ctx,
				`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='failed' WHERE id=$1`, d.id)
			continue
		}

		_, _ = r.pool.Exec(ctx,
			`UPDATE scheduled_reports SET last_run_at=NOW(), last_run_status='success' WHERE id=$1`, d.id)

		slog.Info("Scheduled report completed", "id", d.id, "result_id", result.ID, "file", result.FilePath)
	}
}

// dueReports reads every enabled schedule and returns the ones due at now.
// The cursor is closed before any report is generated, so a slow report
// doesn't pin a pool connection.
func (r *ScheduledRunner) dueReports(ctx context.Context, now time.Time) ([]dueReport, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, report_type, format, schedule_cron, timezone, scope_type, scope_value,
		 recipients, lookback_period, custom_from, custom_to, last_run_at
		 FROM scheduled_reports WHERE enabled=true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var due []dueReport
	for rows.Next() {
		var d dueReport
		var cronExpr, tz string
		var lastRun *time.Time
		if err := rows.Scan(&d.id, &d.name, &d.reportType, &d.format, &cronExpr, &tz, &d.scopeType, &d.scopeValue,
			&d.recipients, &d.lookback, &d.customFrom, &d.customTo, &lastRun); err != nil {
			slog.Error("Failed to scan scheduled report", "error", err)
			continue
		}
		ok, err := isDue(lastRun, cronExpr, tz, now)
		if err != nil {
			slog.Warn("Invalid report schedule", "id", d.id, "cron", cronExpr, "error", err)
			continue
		}
		if ok {
			due = append(due, d)
		}
	}
	return due, rows.Err()
}

// isDue reports whether a schedule has a fire time in (lastRun, now],
// evaluated in the report's timezone. A schedule that has never run is due
// immediately.
func isDue(lastRun *time.Time, cronExpr, tz string, now time.Time) (bool, error) {
	schedule, err := cronParser.Parse(cronExpr)
	if err != nil {
		return false, err
	}
	if lastRun == nil {
		return true, nil
	}
	loc := time.Local
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	next := schedule.Next(lastRun.In(loc))
	return !next.After(now), nil
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
