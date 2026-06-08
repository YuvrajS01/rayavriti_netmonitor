package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
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
	db := database.NewPostgres(cfg.Database.DSN)
	if err := db.Connect(context.Background()); err != nil {
		return fmt.Errorf("database connect: %w", err)
	}
	defer db.Close()
	logger.Info("Database connected")

	// 4. Run migrations
	if err := db.RunMigrations(context.Background()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("Migrations complete")

	// 5. Seed defaults
	adminPass := cfg.Auth.AdminPassword
	if adminPass == "" {
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
		stats, err := db.GetDashboardStats(ctx)
		if err != nil {
			stats = map[string]any{}
		}
		latestMetrics, err := db.GetLatestMetrics(ctx)
		if err != nil {
			latestMetrics = nil
		}
		alerts, _, err := db.GetAlerts(ctx, "active", 50, 0)
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
	hub := websocket.NewHub(cfg.Auth.JWTSecret, bootstrapFn)
	go hub.Run()
	logger.Info("WebSocket hub started")

	// 7. Initialize collector registry
	registry := collectors.NewRegistry()
	registry.Register(collectors.PingCollector{})
	registry.Register(collectors.HTTPCollector{})
	registry.Register(collectors.PortCollector{})
	registry.Register(collectors.SNMPCollector{})
	registry.Register(collectors.SystemCollector{})
	logger.Info("Collectors registered", "count", 5)

	// 8. Initialize alert engine (used by scheduler for rule evaluation)
	alertEng := engine.NewAlertEngine(db)

	// 9. Initialize scheduler
	sched := scheduler.New(db, registry, hub, alertEng, cfg.Collector.CollectorIntervalSec)
	sched.Start(context.Background())
	logger.Info("Scheduler started")

	// 10. Initialize anomaly engine
	anomalyEng := engine.NewAnomalyEngine(db)
	anomalyEng.Start(context.Background())
	logger.Info("Anomaly engine started")

	// 11. Initialize retention scheduler
	retSched := retention.New(db,
		cfg.Collector.MetricsRetentionDays,
		cfg.Collector.FlowRetentionDays,
		cfg.Collector.AlertsRetentionDays)
	retSched.Start(context.Background())
	logger.Info("Retention scheduler started")

	// 12. Initialize HTTP server
	srv := server.New(cfg, db, hub, logger)

	// 13. Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
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
