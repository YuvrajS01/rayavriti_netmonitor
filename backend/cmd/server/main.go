package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/campus"
	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"github.com/rayavriti/netmonitor-backend/internal/metrics"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/monitoring"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
	"github.com/rayavriti/netmonitor-backend/internal/reports"
	"github.com/rayavriti/netmonitor-backend/internal/retention"
	"github.com/rayavriti/netmonitor-backend/internal/scheduler"
	"github.com/rayavriti/netmonitor-backend/internal/server"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Initialize logger
	logger := logging.New(cfg)
	logger.Info("Rayavriti NetMonitor starting", "version", cfg.App.Version)

	// 3. Connect to database
	dbCfg := &database.DatabaseConfig{
		MaxConns:          int32(cfg.Database.MaxConns), //nolint:gosec // MaxConns is validated in config parsing
		MinConns:          int32(cfg.Database.MinConns), //nolint:gosec // MinConns is validated in config parsing
		MaxConnLifetime:   cfg.Database.MaxConnLifetime,
		HealthCheckPeriod: cfg.Database.HealthCheckPeriod,
	}
	db := database.NewPostgres(cfg.Database.DSN, dbCfg)
	if err := db.Connect(context.Background()); err != nil {
		return fmt.Errorf("database connect: %w", err)
	}
	defer func() { _ = db.Close() }()
	logger.Info("Database connected")

	// 3.5 Connect to Redis (optional)
	var rdb *cache.Redis
	if cfg.Redis.Enabled {
		rdb, err = cache.NewRedis(cache.RedisConfig{
			URL:          cfg.Redis.URL,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
		})
		if err != nil {
			logger.Warn("Redis connection failed, running without cache", "error", err)
		} else {
			defer func() { _ = rdb.Close() }()
			logger.Info("Redis connected")
			auth.SetDefaultStore(auth.NewRedisSessionStore(rdb))
		}
	}

	// 3.6 Wrap DB with cache layer if Redis is available
	var appDB database.Database = db
	if rdb != nil {
		appDB = cache.NewCachedDatabase(db, rdb)
		logger.Info("Database cache layer enabled")
	}

	// 4. Run migrations
	if err := db.RunMigrations(context.Background()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("Migrations complete")

	if err := db.ApplyRetentionPolicies(context.Background(), retentionPolicies(cfg, logger)); err != nil {
		return fmt.Errorf("apply retention policies: %w", err)
	}

	// 5. Seed defaults
	adminPass := cfg.Auth.AdminPassword
	if adminPass == "" {
		if cfg.App.AppEnv == "production" {
			return fmt.Errorf("ADMIN_PASSWORD is required in production mode")
		}
		adminPass = "admin123"
		logger.Warn("Using default admin password - set ADMIN_PASSWORD env var")
	}
	hash, err := auth.HashPassword(adminPass)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	if err := database.SeedDefaults(context.Background(), db, cfg.Auth.AdminUsername, hash); err != nil {
		return fmt.Errorf("seed defaults: %w", err)
	}
	logger.Info("Defaults seeded", "admin_user", cfg.Auth.AdminUsername)

	// 6. Initialize WebSocket hub with bootstrap function
	bootstrapFn := func(ctx context.Context, userID int64, username, role string) (map[string]any, error) {
		user := map[string]any{"id": userID, "username": username, "role": role}
		if !rbac.IsUnscopedRole(role) {
			// Scoped users only receive their own devices' state (S7).
			return scopedBootstrap(ctx, appDB, db, userID, user)
		}
		stats, err := appDB.GetDashboardStats(ctx)
		if err != nil {
			stats = map[string]any{}
		}
		latestMetrics, err := appDB.GetLatestMetrics(ctx)
		if err != nil {
			latestMetrics = nil
		}
		alerts, _, err := appDB.GetAlerts(ctx, "active", 50, 0, nil)
		if err != nil {
			alerts = nil
		}
		return map[string]any{
			"stats":         stats,
			"latestMetrics": latestMetrics,
			"alerts":        alerts,
			"user":          user,
		}, nil
	}
	hub := websocket.NewHub(
		cfg.Auth.JWTSecret,
		bootstrapFn,
		websocket.DevAwareAllowedOrigins(cfg.App.CORSOrigins, cfg.App.AppEnv == "production"),
	)
	if pp, ok := any(db).(database.PoolProvider); ok && pp.Pool() != nil {
		hub.SetDB(pp.Pool())
	}
	go hub.Run()
	logger.Info("WebSocket hub started")

	// 7. Initialize collector registry
	registry := collectors.NewRegistry()
	registry.Register(collectors.PingCollector{})
	registry.Register(collectors.HTTPCollector{})
	registry.Register(collectors.HTTPSCollector{})
	registry.Register(collectors.PortCollector{})
	registry.Register(collectors.SNMPCollector{})
	registry.Register(collectors.SystemCollector{})
	registry.Register(collectors.CameraCollector{})
	registry.Register(collectors.BiometricCollector{})
	logger.Info("Collectors registered", "count", 8)

	// 8. Initialize alert engine (used by scheduler for rule evaluation).
	// The anomaly engine below owns the baseline cache and shares it with the
	// alert engine so anomaly-condition rules read refreshed baselines.
	baselineCache := engine.NewBaselineCache(15 * time.Minute)
	notifier := engine.NewNotifier()
	alertOpts := []engine.AlertEngineOption{
		engine.WithBaselineCache(baselineCache),
	}
	if rdb != nil {
		alertOpts = append(alertOpts, engine.WithAlertStateCache(cache.NewAlertStateCache(rdb, db)))
	}
	// Maintenance windows and parent-down (topology) suppression. These
	// checks only run when a rule is about to fire, so they add no per-poll
	// queries.
	if pp, ok := any(db).(database.PoolProvider); ok && pp.Pool() != nil {
		alertOpts = append(alertOpts,
			engine.WithSuppressionChecker(campus.NewTopologyService(pp.Pool())),
			engine.WithMaintenanceChecker(campus.NewMaintenanceService(pp.Pool())),
		)
	}
	alertOpts = append(alertOpts, engine.WithSuppressedAlertRecorder(appDB))
	alertEng := engine.NewAlertEngine(appDB, hub, notifier, alertOpts...)
	alertEng.Start(context.Background())
	logger.Info("Alert engine started")

	// 8.5 Initialize metric buffer and Pub/Sub bridge (if Redis available)
	var metricBuf *cache.MetricBuffer
	var pubSubBridge *cache.PubSubBridge
	if rdb != nil {
		metricBuf = cache.NewMetricBuffer(rdb, appDB, 100, 2*time.Second)
		metricBuf.Start(context.Background())
		logger.Info("Metric buffer started")

		pubSubBridge = cache.NewPubSubBridge(rdb, func(msg cache.WSMessage) {
			hub.BroadcastLocal(websocket.Message{
				Type:      websocket.EventType(msg.Type),
				RequestID: msg.RequestID,
				Data:      msg.Data,
			})
		})
		go pubSubBridge.Subscribe(context.Background())
		hub.SetPublisher(func(ctx context.Context, msg websocket.Message) {
			_ = pubSubBridge.Publish(ctx, cache.WSMessage{
				Type:      string(msg.Type),
				RequestID: msg.RequestID,
				Data:      msg.Data,
			})
		})
		logger.Info("Redis Pub/Sub bridge initialized")
	}

	// 9. Initialize scheduler
	schedOpts := []scheduler.SchedulerOption{}
	if metricBuf != nil {
		schedOpts = append(schedOpts, scheduler.WithMetricBuffer(metricBuf))
	}
	if rdb != nil {
		schedOpts = append(schedOpts, scheduler.WithRedis(rdb))
	}
	schedOpts = append(schedOpts, scheduler.WithSchedulerConfig(scheduler.SchedulerConfig{
		WorkerCount:       cfg.Collector.PollerWorkerCount,
		MaxWorkerCount:    cfg.Collector.PollerMaxWorkerCount,
		CriticalQueueSize: cfg.Collector.PollerCriticalQueueSize,
		NormalQueueSize:   cfg.Collector.PollerNormalQueueSize,
		LowQueueSize:      cfg.Collector.PollerLowQueueSize,
		ResultBatchSize:   cfg.Collector.PollerResultBatchSize,
		ResultFlushMs:     cfg.Collector.PollerResultFlushMs,
	}))
	sched := scheduler.New(appDB, registry, hub, alertEng, cfg.Collector.CollectorIntervalSec, schedOpts...)
	sched.Start(context.Background())
	logger.Info("Scheduler started")

	// Prometheus metrics (served by the admin listener below).
	registerRuntimeMetrics(cfg.App.Version, db, sched, hub, metricBuf)

	// 10. Initialize anomaly engine (shares its baseline cache with the alert engine)
	anomalyEng := engine.NewAnomalyEngine(db, slog.Default())
	anomalyEng.SetBaselineCache(baselineCache)
	anomalyEng.Start(context.Background())
	logger.Info("Anomaly engine started")

	// 11. Initialize retention scheduler
	retSched := retention.New(db, retention.Config{
		AlertsDays:        cfg.Collector.AlertsRetentionDays,
		HealthHistoryDays: cfg.Collector.MonHealthRetentionDays,
		CaptureDays:       cfg.Collector.CaptureRetentionDays,
	})
	retSched.Start(context.Background())
	logger.Info("Retention scheduler started")

	// 11.5 Initialize ISP collector and scheduled report runner
	ispCollector := reports.NewISPCollector(db.Pool(), cfg.Integrations.ISPMonitorInterval)
	ispCollector.Start(context.Background())
	logger.Info("ISP collector started", "interval_sec", cfg.Integrations.ISPMonitorInterval)

	reportGen := reports.NewGenerator(db.Pool(), cfg.Integrations.ReportOutputDir)
	reportScheduler := reports.NewScheduledRunner(db.Pool(), reportGen, time.Minute)
	reportScheduler.Start(context.Background())
	logger.Info("Scheduled report runner started")

	// 12. Initialize explicit service-mode synchronization.
	var serviceMode atomic.Value
	serviceMode.Store("active")
	sysConfig := monitoring.NewSysConfigStore(db.Pool())
	if storedMode, err := sysConfig.GetSysConfig(context.Background(), "service_mode"); err == nil && (storedMode == "active" || storedMode == "maintenance" || storedMode == "readonly") {
		serviceMode.Store(storedMode)
	}
	if cfg.Telemetry.Endpoint != "" {
		cfgSync := monitoring.NewConfigSyncService(cfg.Telemetry.Endpoint, sysConfig,
			monitoring.WithSyncInterval(cfg.Telemetry.SyncInterval),
			monitoring.WithGracePeriod(cfg.Telemetry.GraceDays),
			monitoring.WithOnModeChange(func(mode string) { serviceMode.Store(mode); logger.Info("service mode updated", "mode", mode) }),
		)
		cfgSync.Start(context.Background())
		defer cfgSync.Stop()
		logger.Info("configuration sync started")
	}

	// 13. Initialize HTTP server
	srv := server.New(cfg, appDB, hub, logger, server.WithRedis(rdb), server.WithAlertEngine(alertEng), server.WithServiceMode(&serviceMode))

	adminSrv := server.StartAdmin(cfg.App.AdminAddr, cfg.App.PprofEnabled)

	// 13. Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// 14. Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		logger.Info("Shutdown signal received", "signal", sig)
	}

	// 15. Graceful shutdown. Order matters: stop intake first, then the
	// producers (scheduler, collectors), then what they feed (alert engine,
	// metric buffer), and the WebSocket hub last so every component can still
	// broadcast while it drains.
	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}
	sched.Stop()
	ispCollector.Stop()
	reportScheduler.Stop()
	anomalyEng.Stop()
	alertEng.Stop()
	if metricBuf != nil {
		metricBuf.Stop()
	}
	retSched.Stop()
	hub.Stop()
	_ = adminSrv.Shutdown(ctx)
	logger.Info("Shutdown complete")
	return nil
}

// scopedBootstrap builds the initial WebSocket payload for a location- or
// subnet-scoped user from their devices only.
func scopedBootstrap(ctx context.Context, appDB database.Database, db *database.Postgres, userID int64, user map[string]any) (map[string]any, error) {
	scopes, err := rbac.LoadScopes(ctx, db.Pool(), userID)
	if err != nil {
		return nil, err
	}
	sf := (&rbac.ScopeContext{IsScoped: true, Scopes: scopes}).DatabaseScopeFilter()
	devices, _, err := appDB.GetDevicesFiltered(ctx, database.DeviceFilter{Scope: sf, Limit: 100000})
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]bool, len(devices))
	var online, offline int
	for _, d := range devices {
		allowed[d.ID] = true
		switch d.Status {
		case "up":
			online++
		case "down":
			offline++
		}
	}
	var latest []models.Metric
	if all, err := appDB.GetLatestMetrics(ctx); err == nil {
		for _, m := range all {
			if allowed[m.DeviceID] {
				latest = append(latest, m)
			}
		}
	}
	alerts, activeAlerts, err := appDB.GetAlerts(ctx, "active", 50, 0, sf)
	if err != nil {
		alerts, activeAlerts = nil, 0
	}
	return map[string]any{
		"stats": map[string]any{
			"totalDevices":   len(devices),
			"onlineDevices":  online,
			"offlineDevices": offline,
			"activeAlerts":   activeAlerts,
		},
		"latestMetrics": latest,
		"alerts":        alerts,
		"user":          user,
	}, nil
}

// minRawMetricsDays keeps raw metrics longer than the metrics_hourly refresh
// window (3 days). If raw data were dropped inside that window, the next
// refresh would see the hours as empty and erase their rollups.
const minRawMetricsDays = 4

// retentionPolicies maps config to TimescaleDB retention policies.
func retentionPolicies(cfg *config.Config, logger *logging.Logger) []database.RetentionPolicy {
	c := cfg.Collector
	metricsDays := c.MetricsRetentionDays
	if metricsDays > 0 && metricsDays < minRawMetricsDays {
		logger.Warn("METRICS_RETENTION_DAYS is below the rollup refresh window; raising it",
			"configured", metricsDays, "effective", minRawMetricsDays)
		metricsDays = minRawMetricsDays
	}
	return []database.RetentionPolicy{
		{Relation: "metrics", Days: metricsDays},
		{Relation: "metrics_hourly", Days: c.MetricsHourlyRetentionDays},
		{Relation: "isp_metrics", Days: metricsDays},
		{Relation: "flows", Days: c.FlowRetentionDays},
		{Relation: "system_log_events", Days: c.LogRetentionDays},
		{Relation: "capture_packets", Days: c.CaptureRetentionDays},
		{Relation: "alert_history", Days: c.AlertsRetentionDays},
		{Relation: "notification_log", Days: c.AlertsRetentionDays},
		{Relation: "suppressed_alerts", Days: c.AlertsRetentionDays},
		{Relation: "remote_snapshots", Days: cfg.Remote.SnapshotRetentionDays},
		{Relation: "monitoring_http_requests", Days: c.MonHTTPRetentionDays},
		{Relation: "monitoring_db_queries", Days: c.MonDBRetentionDays},
		{Relation: "monitoring_collector_runs", Days: c.MonCollectorRetentionDays},
		{Relation: "monitoring_system_metrics", Days: c.MonHealthRetentionDays},
		{Relation: "monitoring_alerts", Days: c.MonAlertRetentionDays},
		{Relation: "monitoring_alert_activity", Days: c.MonAlertRetentionDays},
		// incident_timeline and metrics_daily are kept indefinitely.
	}
}

// registerRuntimeMetrics exposes state other components already track as
// Prometheus gauges read at scrape time.
func registerRuntimeMetrics(version string, db *database.Postgres, sched *scheduler.Scheduler, hub *websocket.Hub, metricBuf *cache.MetricBuffer) {
	metrics.SetBuildInfo(version)
	if db != nil && db.Pool() != nil {
		metrics.RegisterPool(db.Pool())
	}
	metrics.RegisterGaugeFunc("poller", "scheduled_devices", "Devices currently scheduled for polling.",
		func() float64 { return float64(sched.JobCount()) })
	metrics.RegisterGaugeFunc("poller", "unreachable_devices", "Scheduled devices in unreachable backoff.",
		func() float64 { return float64(sched.UnreachableCount()) })
	metrics.RegisterGaugeFunc("poller", "busy_workers", "Workers currently running a poll.",
		func() float64 { return float64(sched.PoolMetrics().BusyWorkers) })
	for _, q := range []struct {
		name string
		get  func(scheduler.WorkerPoolMetricsSnapshot) int
	}{
		{"critical", func(m scheduler.WorkerPoolMetricsSnapshot) int { return m.QueuedCritical }},
		{"normal", func(m scheduler.WorkerPoolMetricsSnapshot) int { return m.QueuedNormal }},
		{"low", func(m scheduler.WorkerPoolMetricsSnapshot) int { return m.QueuedLow }},
	} {
		get := q.get
		metrics.RegisterGaugeFunc("poller", "queue_depth_"+q.name, "Poll jobs waiting in the "+q.name+" priority queue.",
			func() float64 { return float64(get(sched.PoolMetrics())) })
	}
	metrics.RegisterGaugeFunc("websocket", "clients", "Connected WebSocket clients.",
		func() float64 { return float64(hub.ConnectionCount()) })
	if metricBuf != nil {
		metrics.RegisterGaugeFunc("metric_buffer", "length", "Metrics waiting in the Redis buffer to be persisted.",
			func() float64 {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				n, err := metricBuf.Len(ctx)
				if err != nil {
					return -1
				}
				return float64(n)
			})
	}
}
