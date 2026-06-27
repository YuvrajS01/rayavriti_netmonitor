package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
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
		stats, err := appDB.GetDashboardStats(ctx)
		if err != nil {
			stats = map[string]any{}
		}
		latestMetrics, err := appDB.GetLatestMetrics(ctx)
		if err != nil {
			latestMetrics = nil
		}
		alerts, _, err := appDB.GetAlerts(ctx, "active", 50, 0)
		if err != nil {
			alerts = nil
		}
		return map[string]any{
			"stats":         stats,
			"latestMetrics": latestMetrics,
			"alerts":        alerts,
			"user": map[string]any{
				"id":       userID,
				"username": username,
				"role":     role,
			},
		}, nil
	}
	hub := websocket.NewHub(cfg.Auth.JWTSecret, bootstrapFn, cfg.App.CORSOrigins)
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
	logger.Info("Collectors registered", "count", 6)

	// 8. Initialize alert engine (used by scheduler for rule evaluation)
	notifier := engine.NewNotifier()
	alertOpts := []engine.AlertEngineOption{}
	if rdb != nil {
		alertOpts = append(alertOpts, engine.WithAlertStateCache(cache.NewAlertStateCache(rdb, db)))
	}
	alertEng := engine.NewAlertEngine(appDB, hub, notifier, alertOpts...)

	// 8.5 Initialize metric buffer and Pub/Sub bridge (if Redis available)
	var metricBuf *cache.MetricBuffer
	var pubSubBridge *cache.PubSubBridge
	if rdb != nil {
		metricBuf = cache.NewMetricBuffer(rdb, db, 100, 2*time.Second)
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
	sched := scheduler.New(appDB, registry, hub, alertEng, cfg.Collector.CollectorIntervalSec, schedOpts...)
	sched.Start(context.Background())
	logger.Info("Scheduler started")

	// 10. Initialize anomaly engine
	anomalyEng := engine.NewAnomalyEngine(db, slog.Default())
	anomalyEng.Start(context.Background())
	logger.Info("Anomaly engine started")

	// 11. Initialize retention scheduler
	retSched := retention.New(db,
		cfg.Collector.MetricsRetentionDays,
		cfg.Collector.FlowRetentionDays,
		cfg.Collector.AlertsRetentionDays)
	retSched.Start(context.Background())
	logger.Info("Retention scheduler started")

	// 11.5 Initialize ISP collector and scheduled report runner
	ispCollector := reports.NewISPCollector(db.Pool(), cfg.Phase2.ISPMonitorInterval)
	ispCollector.Start(context.Background())
	logger.Info("ISP collector started", "interval_sec", cfg.Phase2.ISPMonitorInterval)

	reportGen := reports.NewGenerator(db.Pool(), cfg.Phase2.ReportOutputDir)
	reportScheduler := reports.NewScheduledRunner(db.Pool(), reportGen, time.Minute)
	reportScheduler.Start(context.Background())
	logger.Info("Scheduled report runner started")

	// 12. Initialize HTTP server
	srv := server.New(cfg, appDB, hub, logger, server.WithRedis(rdb), server.WithAlertEngine(alertEng))

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

	// 15. Graceful shutdown
	logger.Info("Shutting down...")
	sched.Stop()
	ispCollector.Stop()
	reportScheduler.Stop()
	if metricBuf != nil {
		metricBuf.Stop()
	}
	anomalyEng.Stop()
	retSched.Stop()
	hub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}
	logger.Info("Shutdown complete")
	return nil
}
