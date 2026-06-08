package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/handlers"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type Server struct {
	cfg        *config.Config
	db         database.Database
	hub        *websocket.Hub
	logger     *logging.Logger
	httpServer *http.Server
}

func New(cfg *config.Config, db database.Database, hub *websocket.Hub, logger *logging.Logger) *Server {
	return &Server{cfg: cfg, db: db, hub: hub, logger: logger}
}

func (s *Server) Start() error {
	r := chi.NewRouter()

	// CORS (before logging to avoid logging preflight OPTIONS requests)
	corsHandler := cors.New(cors.Options{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	r.Use(corsHandler.Handler)

	// Middleware
	r.Use(Recovery)
	r.Use(SecurityHeaders)
	r.Use(logging.RequestLogger(s.logger, s.cfg.Logging.SlowRequestMs))
	r.Use(RequestSize(1 << 20)) // 1MB
	if s.cfg.App.NodeEnv == "production" {
		r.Use(RateLimiter(100, 200))
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
	capture := handlers.NewCaptureHandler(s.db)
	ports := handlers.NewPortsHandler()
	dashboard := handlers.NewDashboardHandler(s.db)
	simulator := handlers.NewSimulatorHandler(s.db)
	sensor := handlers.NewSensorHandler(s.db)

	// Public routes
	r.Get("/health", health.Health)
	r.Get("/ws", s.hub.ServeWS)

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
		r.Use(auth.UserRateLimiter())
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
		r.Get("/api/v1/devices/{id}/metrics", metric.ForDevice)

		// Alerts
		r.Get("/api/alerts", alert.List)
		r.Post("/api/alerts", alert.Create)
		r.Get("/api/alerts/{id}", alert.Get)
		r.Post("/api/alerts/{id}/acknowledge", alert.Acknowledge)
		r.Post("/api/alerts/{id}/resolve", alert.Resolve)
		r.Delete("/api/alerts/{id}", alert.Delete)
		r.Get("/api/alerts/counts", alert.Counts)

		// Reports
		r.Get("/api/reports/summary", report.Summary)
		r.Get("/api/reports/timeseries", report.Timeseries)
		r.Get("/api/reports/devices", report.Devices)
		r.Get("/api/reports/alerts", report.Alerts)
		r.Get("/api/reports/export", report.Export)

		// Insights
		r.Get("/api/insights", insight.Current)
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

		// Dashboards
		r.Get("/api/v1/dashboards", dashboard.List)
		r.Post("/api/v1/dashboards", dashboard.Save)
		r.Get("/api/v1/dashboards/{id}", dashboard.Get)
		r.Put("/api/v1/dashboards/{id}", dashboard.Save)
		r.Delete("/api/v1/dashboards/{id}", dashboard.Delete)

		// API Keys
		r.Get("/api/v1/auth/apikeys", authH.ListAPIKeys)
		r.Post("/api/v1/auth/apikeys", authH.CreateAPIKey)
		r.Delete("/api/v1/auth/apikeys/{id}", authH.DeleteAPIKey)

		// Simulator
		r.Post("/api/simulator/metrics", simulator.Metrics)
		r.Post("/api/simulator/flows", simulator.Flows)
		r.Post("/api/simulator/alert", simulator.Alert)
	})

	// 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		SendError(w, 404, "not found")
	})

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.App.Port),
		Handler: r,
	}

	s.logger.Info("HTTP server starting", "port", s.cfg.App.Port)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
