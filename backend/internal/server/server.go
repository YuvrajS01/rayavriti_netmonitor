package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/handlers"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/rs/cors"
)

type Server struct {
	cfg        *config.Config
	db         database.Database
	hub        *websocket.Hub
	rdb        *cache.Redis
	logger     *logging.Logger
	httpServer *http.Server
	cancel     context.CancelFunc
}

func New(cfg *config.Config, db database.Database, hub *websocket.Hub, logger *logging.Logger, opts ...ServerOption) *Server {
	s := &Server{cfg: cfg, db: db, hub: hub, logger: logger}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type ServerOption func(*Server)

func WithRedis(rdb *cache.Redis) ServerOption {
	return func(s *Server) { s.rdb = rdb }
}

func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	r := chi.NewRouter()

	// CORS (before logging to avoid logging preflight OPTIONS requests)
	allowAll := s.cfg.App.AppEnv != "production"
	if allowAll && len(s.cfg.App.CORSOrigins) > 0 {
		allowAll = false
	}
	if s.cfg.App.AppEnv == "production" && len(s.cfg.App.CORSOrigins) == 0 {
		s.logger.Warn("CORS_ORIGINS is empty in production — all origins will be blocked")
	}
	corsHandler := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			if allowAll {
				return true
			}
			for _, o := range s.cfg.App.CORSOrigins {
				if o == origin {
					return true
				}
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	r.Use(corsHandler.Handler)

	// Middleware
	r.Use(RequestID)
	r.Use(Recovery)
	r.Use(SecurityHeaders)
	r.Use(logging.RequestLogger(s.logger, s.cfg.Logging.SlowRequestMs))
	r.Use(RequestSize(1 << 20)) // 1MB
	if s.cfg.App.AppEnv == "production" {
		r.Use(RateLimiter(ctx, 100, 200, s.rdb))
	}

	// Auth helper
	requireAuth := auth.RequireAuth(s.cfg.Auth.JWTSecret, func(ctx context.Context, hash string) (*auth.Claims, error) {
		key, err := s.db.GetAPIKey(ctx, hash)
		if err != nil {
			return nil, err
		}
		user, err := s.db.GetUserByID(ctx, key.UserID)
		if err != nil {
			return nil, err
		}
		return &auth.Claims{UserID: user.ID, Username: user.Username, Role: user.Role}, nil
	})

	// Handlers
	health := handlers.NewHealthHandler(s.db)
	authH := handlers.NewAuthHandler(s.db, s.cfg)
	device := handlers.NewDeviceHandler(s.db)
	metric := handlers.NewMetricHandler(s.db)
	alert := handlers.NewAlertHandler(s.db)
	flow := handlers.NewFlowHandler(s.db)
	report := handlers.NewReportHandler(s.db)
	insight := handlers.NewInsightHandler(s.db)
	capture := handlers.NewCaptureHandler(s.db, s.hub)
	ports := handlers.NewPortsHandler(s.db)
	dashboard := handlers.NewDashboardHandler(s.db)
	simulator := handlers.NewSimulatorHandler(s.db)
	sensor := handlers.NewSensorHandler(s.db)
	alertRule := handlers.NewAlertRuleHandler(s.db)
	notifChannel := handlers.NewNotificationChannelHandler(s.db)
	system := handlers.NewSystemHandler()

	// Public routes
	r.Get("/health", health.Health)
	r.Get("/ws", s.hub.ServeWS)
	r.Get("/api/v1/ws", s.hub.ServeWS)

	// Auth routes
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/logout", authH.Logout)
	r.Post("/api/auth/refresh", authH.Refresh)

	// V1 Auth (public)
	r.Post("/api/v1/auth/login", authH.V1Login)
	r.Post("/api/v1/auth/refresh", authH.Refresh)
	r.Post("/api/v1/auth/2fa/verify", authH.Verify2FA)
	r.Post("/api/v1/auth/logout", authH.V1Logout)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(requireAuth)
		r.Use(auth.UserRateLimiter(ctx, s.rdb))
		r.Get("/api/auth/me", authH.Me)
		r.Get("/api/stats", health.Stats)

		// Devices (legacy + v1)
		r.Get("/api/devices", device.List)
		r.Post("/api/devices", device.Create)
		r.Get("/api/devices/{id}", device.Get)
		r.Put("/api/devices/{id}", device.Update)
		r.Delete("/api/devices/{id}", device.Delete)
		r.Get("/api/v1/devices", device.List)
		r.Post("/api/v1/devices", device.Create)
		r.Get("/api/v1/devices/{id}", device.Get)
		r.Put("/api/v1/devices/{id}", device.Update)
		r.Delete("/api/v1/devices/{id}", device.Delete)

		// Metrics
		r.Get("/api/metrics/latest", metric.Latest)
		r.Get("/api/metrics/{deviceId}", metric.ForDevice)
		r.Get("/api/v1/metrics/query", metric.Query)

		// Devices
		r.Get("/api/devices/{id}/ports", ports.ForDevice)
		r.Post("/api/devices/{id}/scan-ports", device.ScanPorts)
		r.Get("/api/v1/devices/{deviceId}/metrics", metric.ForDevice)

		// Alerts
		r.Get("/api/alerts", alert.List)
		r.Post("/api/alerts", alert.Create)
		r.Get("/api/alerts/counts", alert.Counts)
		r.Get("/api/alerts/grouped", alert.Grouped)
		r.Get("/api/alerts/{id}", alert.Get)
		r.Post("/api/alerts/{id}/acknowledge", alert.Acknowledge)
		r.Post("/api/alerts/{id}/resolve", alert.Resolve)
		r.Delete("/api/alerts/{id}", alert.Delete)

		// V1 Alerts
		r.Get("/api/v1/alerts", alert.List)
		r.Post("/api/v1/alerts", alert.Create)
		r.Get("/api/v1/alerts/grouped", alert.Grouped)
		r.Get("/api/v1/alerts/{id}", alert.Get)
		r.Put("/api/v1/alerts/{id}", alert.Update)
		r.Delete("/api/v1/alerts/{id}", alert.Delete)
		r.Post("/api/v1/alerts/{id}/acknowledge", alert.Acknowledge)
		r.Post("/api/v1/alerts/{id}/resolve", alert.Resolve)
		r.Get("/api/v1/alerts/{id}/history", alert.History)
		r.Get("/api/v1/alert-stats", alert.AlertStats)

		// Reports
		r.Get("/api/reports/summary", report.Summary)
		r.Get("/api/reports/timeseries", report.Timeseries)
		r.Get("/api/reports/devices", report.Devices)
		r.Get("/api/reports/alerts", report.Alerts)
		r.Get("/api/reports/export", report.Export)

		// V1 Reports
		r.Get("/api/v1/reports", report.List)

		// Insights
		r.Get("/api/insights", insight.Current)
		r.Get("/api/insights/current", insight.Current)
		r.Get("/api/insights/history", insight.History)

		// Flows
		r.Get("/api/v1/flows", flow.List)
		r.Get("/api/v1/flows/top-talkers", flow.TopTalkers)
		r.Get("/api/v1/flows/protocols", flow.Protocols)
		r.Get("/api/v1/flows/timeseries", flow.Timeseries)
		r.Get("/api/v1/flows/stats", flow.Stats)

		// Capture
		r.Get("/api/v1/capture/interfaces", capture.Interfaces)
		r.Post("/api/v1/capture/start", capture.Start)
		r.Post("/api/v1/capture/{id}/stop", capture.Stop)
		r.Get("/api/v1/capture/{id}", capture.GetSession)
		r.Get("/api/v1/capture/{id}/packets", capture.GetPackets)
		r.Get("/api/v1/capture/sessions", capture.ListSessions)

		// Ports
		r.Get("/api/v1/devices/{id}/ports", ports.ForDevice)

		// Sensors
		r.Get("/api/v1/sensors", sensor.List)
		r.Get("/api/v1/sensors/{id}", sensor.Get)
		r.Post("/api/v1/sensors", sensor.Create)
		r.Put("/api/v1/sensors/{id}", sensor.Update)
		r.Delete("/api/v1/sensors/{id}", sensor.Delete)

		// Dashboards
		r.Get("/api/v1/dashboards", dashboard.List)
		r.Post("/api/v1/dashboards", dashboard.Save)
		r.Get("/api/v1/dashboards/{id}", dashboard.Get)
		r.Put("/api/v1/dashboards/{id}", dashboard.Save)
		r.Delete("/api/v1/dashboards/{id}", dashboard.Delete)

		// Alert Rules
		r.Get("/api/v1/alert-rules", alertRule.List)
		r.Post("/api/v1/alert-rules", alertRule.Create)
		r.Get("/api/v1/alert-rules/{id}", alertRule.Get)
		r.Put("/api/v1/alert-rules/{id}", alertRule.Update)
		r.Delete("/api/v1/alert-rules/{id}", alertRule.Delete)
		r.Post("/api/v1/alert-rules/{id}/toggle", alertRule.Toggle)
		r.Post("/api/v1/alert-rules/{id}/test", alertRule.Test)

		// Notification Channels
		r.Get("/api/v1/notification-channels", notifChannel.List)
		r.Post("/api/v1/notification-channels", notifChannel.Create)
		r.Get("/api/v1/notification-channels/{id}", notifChannel.Get)
		r.Put("/api/v1/notification-channels/{id}", notifChannel.Update)
		r.Delete("/api/v1/notification-channels/{id}", notifChannel.Delete)
		r.Post("/api/v1/notification-channels/{id}/test", notifChannel.Test)

		// API Keys
		r.Get("/api/v1/auth/apikeys", authH.ListAPIKeys)
		r.Post("/api/v1/auth/apikeys", authH.CreateAPIKey)
		r.Delete("/api/v1/auth/apikeys/{id}", authH.DeleteAPIKey)

		// System Info
		r.Get("/api/v1/system/info", system.Info)

		// Simulator (admin only)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))
			r.Post("/api/simulator/metrics", simulator.Metrics)
			r.Post("/api/simulator/flows", simulator.Flows)
			r.Post("/api/simulator/alert", simulator.Alert)
		})
	})

	// SPA static file serving (production)
	publicDir := s.cfg.App.PublicDir
	if dir, err := os.Stat(publicDir); err == nil && dir.IsDir() {
		spa := spaHandler{staticDir: publicDir}
		r.NotFound(spa.ServeHTTP)
		s.logger.Info("SPA static files serving", "dir", publicDir)
	} else {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			SendError(w, 404, "not found")
		})
		s.logger.Info("No static files directory found, API-only mode", "dir", publicDir)
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.cfg.App.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("HTTP server starting", "port", s.cfg.App.Port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// spaHandler serves static files and falls back to index.html for SPA routing.
type spaHandler struct {
	staticDir string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticDir, filepath.Clean(r.URL.Path))

	// If file exists, serve it
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	// Otherwise serve index.html (SPA fallback)
	http.ServeFile(w, r, filepath.Join(h.staticDir, "index.html"))
}
