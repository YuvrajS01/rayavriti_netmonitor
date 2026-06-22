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
	"github.com/rayavriti/netmonitor-backend/internal/discovery"
	"github.com/rayavriti/netmonitor-backend/internal/handlers"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
	"github.com/rayavriti/netmonitor-backend/internal/servicetmpl"
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
	phase2 := handlers.NewPhase2Handler(s.db)
	campusH := handlers.NewCampusHandler(s.db)
	contactH := handlers.NewContactHandler(s.db)
	incidentH := handlers.NewIncidentHandler(s.db, s.hub)
	statusPageH := handlers.NewStatusPageHandler(s.db)
	ispH := handlers.NewISPHandler(s.db)
	reportGenH := handlers.NewReportGenHandler(s.db, s.cfg.Phase2.ReportOutputDir)
	discH := discovery.NewDiscoveryHandler(s.db)

	var svcTmplH *servicetmpl.Handler
	if pp, ok := s.db.(database.PoolProvider); ok && pp.Pool() != nil {
		svcTmplH = servicetmpl.NewHandler(servicetmpl.NewService(pp.Pool()))
	}

	// WebSocket scope filter: only deliver events to users with matching scopes
	if pp, ok := s.db.(database.PoolProvider); ok && pp.Pool() != nil {
		pool := pp.Pool()
		s.hub.SetScopeFilter(func(info websocket.ClientInfo, msg websocket.Message) bool {
		if info.Role == "super_admin" || info.Role == "admin" {
			return true
		}
		if len(info.Scopes) == 0 {
			return true
		}
		switch msg.Type {
		case websocket.EventMetricUpdate, websocket.EventAlertTriggered,
			websocket.EventAlertUpdated, websocket.EventAlertResolved,
			websocket.EventDeviceStatus:
			data, ok := msg.Data.(map[string]any)
			if !ok {
				return true
			}
			deviceID, _ := data["device_id"].(float64)
			if deviceID == 0 {
				return true
			}
			var locationID int64
			_ = pool.QueryRow(context.Background(),
				"SELECT COALESCE(location_id, 0) FROM devices WHERE id = $1", int64(deviceID)).
				Scan(&locationID)
			if locationID == 0 {
				return true
			}
			for _, scope := range info.Scopes {
				if scope.ScopeType == "location" && scope.ScopeValue == fmt.Sprintf("%d", locationID) {
					return true
				}
			}
			return false
		default:
			return true
		}
	})
	}

	// Service templates

	// Public routes
	r.Get("/health", health.Health)
	r.Get("/ws", s.hub.ServeWS)
	r.Get("/api/v1/ws", s.hub.ServeWS)
	r.Get("/status", phase2.PublicStatusHTML)
	r.Get("/api/v1/public/status", statusPageH.PublicStatusJSON)
	r.Get("/api/v1/public/incidents", phase2.List("status_page_incidents"))

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
		if pp, ok := s.db.(database.PoolProvider); ok && pp.Pool() != nil {
			r.Use(rbac.RequireScopeContext(pp.Pool()))
		}
		r.Get("/api/auth/me", authH.Me)
		r.Get("/api/stats", health.Stats)

		// Devices (legacy + v1)
		r.Get("/api/devices", device.List)
		r.Post("/api/devices", device.Create)
		r.Get("/api/devices/{id}", device.Get)
		r.Put("/api/devices/{id}", device.Update)
		r.Delete("/api/devices/{id}", device.Delete)
		r.Get("/api/v1/devices", device.List)
		r.Get("/api/v1/devices/{id}", device.Get)

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

		// Phase 2 summary
		r.Get("/api/v1/phase2/summary", phase2.Summary)

		// Location hierarchy and topology (typed campus handlers)
		r.Get("/api/v1/locations", campusH.ListLocations)
		r.Post("/api/v1/locations", campusH.CreateLocation)
		r.Get("/api/v1/locations/{id}", campusH.GetLocation)
		r.Get("/api/v1/locations/{id}/tree", campusH.GetLocationTree)
		r.Get("/api/v1/locations/{id}/status", campusH.LocationStatus)
		r.Get("/api/v1/locations/{id}/devices", campusH.LocationDevices)
		r.Put("/api/v1/locations/{id}", campusH.UpdateLocation)
		r.Delete("/api/v1/locations/{id}", campusH.DeleteLocation)
		r.Post("/api/v1/locations/{id}/move", campusH.MoveLocation)
		r.Get("/api/v1/subnets", phase2.List("subnets"))
		r.Post("/api/v1/subnets", phase2.Create("subnets"))
		r.Get("/api/v1/subnets/{id}", phase2.Get("subnets"))
		r.Put("/api/v1/subnets/{id}", phase2.Update("subnets"))
		r.Delete("/api/v1/subnets/{id}", phase2.Delete("subnets"))
		r.Get("/api/v1/topology", campusH.DependencyTree)
		r.Get("/api/v1/topology/map", campusH.DependencyTree)
		r.Get("/api/v1/topology/dependency-tree", campusH.DependencyTree)
		r.Get("/api/v1/alerts/suppressed", phase2.List("suppressed_alerts"))
		r.Get("/api/v1/outages/root-cause", campusH.RootCauseOutages)
		r.Put("/api/v1/devices/{id}/parent", device.Update)
		r.Get("/api/v1/devices/{id}/dependencies", campusH.DeviceDependencies)

		// Bulk import and discovery (typed import handlers)
		r.Get("/api/v1/import/history", phase2.List("discovery_jobs"))
		r.Get("/api/v1/import/template", campusH.ImportTemplate)
		r.Post("/api/v1/import/devices", campusH.ImportPreview)
		r.Post("/api/v1/import/devices/confirm", campusH.ImportExecute)
		r.Post("/api/v1/discovery/scan", discH.StartScan)
		r.Get("/api/v1/discovery/jobs", discH.ListJobs)
		r.Get("/api/v1/discovery/jobs/{id}", discH.GetJob)
		r.Get("/api/v1/discovery/jobs/{id}/results", discH.GetJobResults)
		r.Post("/api/v1/discovery/results/{id}/approve", discH.ApproveResult)
		r.Post("/api/v1/discovery/results/{id}/reject", discH.RejectResult)
		r.Post("/api/v1/discovery/results/bulk-approve", discH.BulkApprove)

		// Status page administration (typed)
		r.Get("/api/v1/status-page/services", phase2.List("status_page_services"))
		r.Post("/api/v1/status-page/services", phase2.Create("status_page_services"))
		r.Get("/api/v1/status-page/services/{id}", phase2.Get("status_page_services"))
		r.Put("/api/v1/status-page/services/{id}", phase2.Update("status_page_services"))
		r.Delete("/api/v1/status-page/services/{id}", phase2.Delete("status_page_services"))
		r.Post("/api/v1/status-page/services/{id}/devices", statusPageH.AddServiceDevice)
		r.Delete("/api/v1/status-page/services/{id}/devices/{did}", statusPageH.RemoveServiceDevice)
		r.Get("/api/v1/status-page/services/{id}/devices", statusPageH.ListServiceDevices)
		r.Post("/api/v1/status-page/incidents", phase2.Create("status_page_incidents"))
		r.Put("/api/v1/status-page/incidents/{id}", phase2.Update("status_page_incidents"))
		r.Post("/api/v1/status-page/incidents/{id}/update", phase2.Create("status_page_incident_updates"))
		r.Get("/api/v1/status-page/incidents/{id}/updates", statusPageH.ListIncidentUpdates)
		r.Post("/api/v1/status-page/incidents/{id}/services", statusPageH.LinkIncidentServices)

		// Maintenance, contacts, escalation, incidents, RBAC, reports, ISP
		r.Get("/api/v1/maintenance", phase2.List("maintenance_windows"))
		r.Get("/api/v1/maintenance/active", phase2.List("maintenance_windows"))
		r.Get("/api/v1/maintenance/calendar", phase2.List("maintenance_windows"))
		r.Post("/api/v1/maintenance", phase2.Create("maintenance_windows"))
		r.Get("/api/v1/maintenance/{id}", phase2.Get("maintenance_windows"))
		r.Put("/api/v1/maintenance/{id}", phase2.Update("maintenance_windows"))
		r.Delete("/api/v1/maintenance/{id}", phase2.Delete("maintenance_windows"))
		r.Post("/api/v1/maintenance/{id}/toggle", phase2.Update("maintenance_windows"))
		r.Get("/api/v1/contacts", phase2.List("contacts"))
		r.Post("/api/v1/contacts", phase2.Create("contacts"))
		r.Get("/api/v1/contacts/{id}", phase2.Get("contacts"))
		r.Put("/api/v1/contacts/{id}", phase2.Update("contacts"))
		r.Delete("/api/v1/contacts/{id}", phase2.Delete("contacts"))
		r.Get("/api/v1/contacts/{id}/devices", phase2.List("device_contacts"))
		r.Post("/api/v1/devices/{id}/contacts", phase2.Create("device_contacts"))
		r.Delete("/api/v1/devices/{id}/contacts/{cid}", phase2.Delete("device_contacts"))
		r.Post("/api/v1/locations/{id}/contacts", phase2.Create("device_contacts"))
		r.Get("/api/v1/escalation-policies", phase2.List("escalation_policies"))
		r.Post("/api/v1/escalation-policies", phase2.Create("escalation_policies"))
		r.Put("/api/v1/escalation-policies/{id}", phase2.Update("escalation_policies"))
		r.Delete("/api/v1/escalation-policies/{id}", phase2.Delete("escalation_policies"))
		r.Post("/api/v1/escalation-policies/{id}/steps", phase2.Create("escalation_steps"))
		r.Get("/api/v1/escalation-policies/{id}/steps", phase2.List("escalation_steps"))

		// Typed escalation and notification endpoints
		r.Post("/api/v1/devices/{id}/resolve-contacts", contactH.ResolveContacts)
		r.Post("/api/v1/alerts/{id}/escalate", contactH.EscalationStart)
		r.Post("/api/v1/alerts/{id}/cancel-escalation", contactH.EscalationCancel)
		r.Get("/api/v1/alerts/{id}/escalation-status", contactH.EscalationStatus)
		r.Get("/api/v1/notification-log", contactH.NotificationLog)
		r.Get("/api/v1/oncall", phase2.List("oncall_schedules"))
		r.Get("/api/v1/oncall/schedule", phase2.List("oncall_schedules"))
		r.Put("/api/v1/oncall/{id}/override", phase2.Update("oncall_schedules"))
		r.Get("/api/v1/incidents", phase2.List("incidents"))
		r.Post("/api/v1/incidents", incidentH.CreateIncident)
		r.Get("/api/v1/incidents/stats", incidentH.GetIncidentStats)
		r.Get("/api/v1/incidents/sla-report", incidentH.GetSLAReport)
		r.Get("/api/v1/incidents/{id}", phase2.Get("incidents"))
		r.Put("/api/v1/incidents/{id}", phase2.Update("incidents"))
		r.Post("/api/v1/incidents/{id}/note", incidentH.AddTimelineEntry)
		r.Post("/api/v1/incidents/{id}/assign", incidentH.AssignIncident)
		r.Post("/api/v1/incidents/{id}/resolve", incidentH.ResolveIncident)
		r.Post("/api/v1/incidents/{id}/close", incidentH.CloseIncident)
		r.Post("/api/v1/incidents/{id}/acknowledge", incidentH.AcknowledgeIncident)
		r.Get("/api/v1/incidents/{id}/timeline", incidentH.GetTimeline)
		r.Get("/api/v1/incidents/{id}/devices", incidentH.GetIncidentDevices)
		r.Get("/api/v1/sla", phase2.List("sla_definitions"))
		r.Put("/api/v1/sla/{id}", phase2.Update("sla_definitions"))
		r.Get("/api/v1/roles", phase2.List("roles"))
		r.Post("/api/v1/roles", phase2.Create("roles"))
		r.Put("/api/v1/roles/{id}", phase2.Update("roles"))
		r.Delete("/api/v1/roles/{id}", phase2.Delete("roles"))
		r.Get("/api/v1/users", phase2.List("users"))
		r.Get("/api/v1/users/{id}", phase2.Get("users"))
		r.Put("/api/v1/users/{id}", phase2.Update("users"))
		r.Delete("/api/v1/users/{id}", phase2.Delete("users"))
		r.Put("/api/v1/users/{id}/role", phase2.Update("users"))
		r.Get("/api/v1/user-scopes", phase2.List("user_scopes"))
		r.Put("/api/v1/users/{id}/scopes", phase2.Create("user_scopes"))
		r.Delete("/api/v1/users/{id}/scopes/{sid}", phase2.Delete("user_scopes"))

		// RBAC: permissions endpoint (returns the caller's own permissions)
		r.Get("/api/v1/auth/permissions", func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				httputil.SendError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			httputil.SendOK(w, map[string]any{
				"userId":      claims.UserID,
				"role":        claims.Role,
				"permissions": claims.Permissions,
			})
		})

		// Device write routes require devices.write permission
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/devices", device.Create)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/v1/devices/{id}", device.Update)
		r.With(rbac.RequirePermission(models.PermDevicesDelete)).Delete("/api/v1/devices/{id}", device.Delete)
		r.Get("/api/v1/reports/generated", reportGenH.ListGenerated)
		r.Get("/api/v1/reports/generated/{id}/download", reportGenH.DownloadReport)
		r.Get("/api/v1/reports/scheduled", phase2.List("scheduled_reports"))
		r.Post("/api/v1/reports/scheduled", phase2.Create("scheduled_reports"))
		r.Put("/api/v1/reports/scheduled/{id}", phase2.Update("scheduled_reports"))
		r.Delete("/api/v1/reports/scheduled/{id}", phase2.Delete("scheduled_reports"))
		r.Post("/api/v1/reports/scheduled/{id}/run", reportGenH.RunScheduledReport)
		r.Post("/api/v1/reports/generate", reportGenH.GenerateReport)
		r.Get("/api/v1/reports/sla", phase2.List("sla_definitions"))
		r.Get("/api/v1/reports/mttr", phase2.List("incidents"))
		r.Get("/api/v1/reports/availability", report.Devices)
		r.Get("/api/v1/reports/isp", phase2.List("isp_metrics"))
		r.Get("/api/v1/reports/top-offenders", report.Devices)
		r.Get("/api/v1/isp-links", phase2.List("isp_links"))
		r.Post("/api/v1/isp-links", phase2.Create("isp_links"))
		r.Get("/api/v1/isp-links/comparison", ispH.Comparison)
		r.Get("/api/v1/isp-links/{id}", phase2.Get("isp_links"))
		r.Put("/api/v1/isp-links/{id}", phase2.Update("isp_links"))
		r.Delete("/api/v1/isp-links/{id}", phase2.Delete("isp_links"))
		r.Get("/api/v1/isp-links/{id}/metrics", ispH.MetricsSummary)
		r.Get("/api/v1/isp-links/{id}/sla", ispH.LinkSLA)

		// Service Templates
		if svcTmplH != nil {
			r.Get("/api/v1/service-templates", svcTmplH.ListTemplates)
			r.Get("/api/v1/service-templates/{name}", svcTmplH.GetTemplate)
			r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/service-templates/apply", svcTmplH.ApplyTemplate)
		}

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
