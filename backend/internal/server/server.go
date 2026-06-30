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
	alertEng   handlers.AlertProcessor
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

func WithAlertEngine(ap handlers.AlertProcessor) ServerOption {
	return func(s *Server) { s.alertEng = ap }
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
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID", "X-Api-Key"},
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
	auditLog := logging.NewAuditLogger(s.logger)

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
		// Load permissions for API key users, same as login
		var permissions []string
		if user.RoleID != nil {
			if perms, err := s.db.GetRolePermissions(ctx, *user.RoleID); err == nil {
				permissions = perms
			}
		}
		return &auth.Claims{UserID: user.ID, Username: user.Username, Role: user.Role, Permissions: permissions}, nil
	})

	// Handlers
	health := handlers.NewHealthHandler(s.db)
	authH := handlers.NewAuthHandler(s.db, s.cfg)
	device := handlers.NewDeviceHandler(s.db).WithAlertEngine(s.alertEng)
	metric := handlers.NewMetricHandler(s.db)
	alert := handlers.NewAlertHandler(s.db)
	flow := handlers.NewFlowHandler(s.db)
	report := handlers.NewReportHandler(s.db)
	insight := handlers.NewInsightHandler(s.db)
	capture := handlers.NewCaptureHandler(s.db, s.hub, handlers.CaptureConfig{
		Enabled:        s.cfg.Collector.CaptureEnabled,
		MaxDurationSec: s.cfg.Collector.CaptureMaxDurationSec,
		MaxPackets:     s.cfg.Collector.CaptureMaxPackets,
		MaxBytes:       s.cfg.Collector.CaptureMaxBytes,
	})
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
				return false
			}
			switch msg.Type {
			case websocket.EventMetricUpdate, websocket.EventAlertTriggered,
				websocket.EventAlertUpdated, websocket.EventAlertResolved,
				websocket.EventDeviceStatus:
				data, ok := msg.Data.(map[string]any)
				if !ok {
					return false
				}
				deviceID, _ := data["device_id"].(float64)
				if deviceID == 0 {
					return false
				}
				var locationID int64
				_ = pool.QueryRow(context.Background(),
					"SELECT COALESCE(location_id, 0) FROM devices WHERE id = $1", int64(deviceID)).
					Scan(&locationID)
				if locationID == 0 {
					return false
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
		r.Use(AuditLog(auditLog))
		if pp, ok := s.db.(database.PoolProvider); ok && pp.Pool() != nil {
			r.Use(rbac.RequireScopeContext(pp.Pool()))
		}

		// --- Auth / Self-service (no special permission, just authenticated) ---
		r.Get("/api/auth/me", authH.Me)
		r.Get("/api/auth/permissions", func(w http.ResponseWriter, r *http.Request) {
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
		r.Get("/api/v1/auth/apikeys", authH.ListAPIKeys)
		r.Post("/api/v1/auth/apikeys", authH.CreateAPIKey)
		r.Delete("/api/v1/auth/apikeys/{id}", authH.DeleteAPIKey)

		// --- System (system.monitoring) ---
		r.With(rbac.RequirePermission(models.PermSystemMonitoring)).Get("/api/stats", health.Stats)
		r.With(rbac.RequirePermission(models.PermSystemMonitoring)).Get("/api/v1/system/info", system.Info)
		r.With(rbac.RequirePermission(models.PermSystemMonitoring)).Get("/api/v1/phase2/summary", phase2.Summary)

		// --- Devices (devices.read / devices.write / devices.delete) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/devices", device.List)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/devices/{id}", device.Get)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/devices", device.Create)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/v1/devices/{id}", device.Update)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/v1/devices/{id}/parent", device.Update)
		r.With(rbac.RequirePermission(models.PermDevicesDelete)).Delete("/api/v1/devices/{id}", device.Delete)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/devices/{id}/ports", ports.ForDevice)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/devices/{deviceId}/metrics", metric.ForDevice)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/devices/{id}/scan-ports", device.ScanPorts)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/devices/{id}/dependencies", campusH.DeviceDependencies)

		// --- Devices (legacy aliases) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/devices", device.List)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/devices/{id}", device.Get)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/devices", device.Create)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/devices/{id}", device.Update)
		r.With(rbac.RequirePermission(models.PermDevicesDelete)).Delete("/api/devices/{id}", device.Delete)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/devices/{id}/ports", ports.ForDevice)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/devices/{id}/scan-ports", device.ScanPorts)

		// --- Metrics (devices.read) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/metrics/query", metric.Query)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/metrics/latest", metric.Latest)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/metrics/{deviceId}", metric.ForDevice)

		// --- Alerts (alerts.read / alerts.create / alerts.acknowledge / alerts.resolve) ---
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alerts", alert.List)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alerts/grouped", alert.Grouped)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alerts/{id}", alert.Get)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alerts/{id}/history", alert.History)
		r.With(rbac.RequirePermission(models.PermAlertsCreate)).Post("/api/v1/alerts", alert.Create)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Put("/api/v1/alerts/{id}", alert.Update)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Delete("/api/v1/alerts/{id}", alert.Delete)
		r.With(rbac.RequirePermission(models.PermAlertsAcknowledge)).Post("/api/v1/alerts/{id}/acknowledge", alert.Acknowledge)
		r.With(rbac.RequirePermission(models.PermAlertsResolve)).Post("/api/v1/alerts/{id}/resolve", alert.Resolve)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alert-stats", alert.AlertStats)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/v1/alerts/suppressed", phase2.List("suppressed_alerts"))

		// --- Alerts (legacy aliases) ---
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/alerts", alert.List)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/alerts/counts", alert.Counts)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/alerts/grouped", alert.Grouped)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Get("/api/alerts/{id}", alert.Get)
		r.With(rbac.RequirePermission(models.PermAlertsCreate)).Post("/api/alerts", alert.Create)
		r.With(rbac.RequirePermission(models.PermAlertsRead)).Delete("/api/alerts/{id}", alert.Delete)
		r.With(rbac.RequirePermission(models.PermAlertsAcknowledge)).Post("/api/alerts/{id}/acknowledge", alert.Acknowledge)
		r.With(rbac.RequirePermission(models.PermAlertsResolve)).Post("/api/alerts/{id}/resolve", alert.Resolve)

		// --- Insights (devices.read) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/insights/current", insight.Current)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/insights/history", insight.History)

		// --- Insights (legacy aliases) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/insights", insight.Current)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/insights/current", insight.Current)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/insights/history", insight.History)

		// --- Flows (devices.read) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/flows", flow.List)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/flows/top-talkers", flow.TopTalkers)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/flows/protocols", flow.Protocols)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/flows/timeseries", flow.Timeseries)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/flows/stats", flow.Stats)

		// --- Capture (devices.read / capture.execute) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/capture/interfaces", capture.Interfaces)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/capture/{id}", capture.GetSession)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/capture/{id}/packets", capture.GetPackets)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/capture/sessions", capture.ListSessions)
		r.With(rbac.RequirePermission(models.PermCaptureExecute)).Post("/api/v1/capture/start", capture.Start)
		r.With(rbac.RequirePermission(models.PermCaptureExecute)).Post("/api/v1/capture/{id}/stop", capture.Stop)

		// --- Sensors (devices.read / devices.write) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/sensors", sensor.List)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/sensors/{id}", sensor.Get)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/sensors", sensor.Create)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/v1/sensors/{id}", sensor.Update)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Delete("/api/v1/sensors/{id}", sensor.Delete)

		// --- Dashboards (devices.read / devices.write) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/dashboards", dashboard.List)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/dashboards/{id}", dashboard.Get)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/dashboards", dashboard.Save)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Put("/api/v1/dashboards/{id}", dashboard.Save)
		r.With(rbac.RequirePermission(models.PermDevicesWrite)).Delete("/api/v1/dashboards/{id}", dashboard.Delete)

		// --- Alert Rules (alert_rules.write) ---
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Get("/api/v1/alert-rules", alertRule.List)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Get("/api/v1/alert-rules/{id}", alertRule.Get)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Post("/api/v1/alert-rules", alertRule.Create)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Put("/api/v1/alert-rules/{id}", alertRule.Update)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Delete("/api/v1/alert-rules/{id}", alertRule.Delete)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Post("/api/v1/alert-rules/{id}/toggle", alertRule.Toggle)
		r.With(rbac.RequirePermission(models.PermAlertRulesWrite)).Post("/api/v1/alert-rules/{id}/test", alertRule.Test)

		// --- Notification Channels (notifications.manage) ---
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Get("/api/v1/notification-channels", notifChannel.List)
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Get("/api/v1/notification-channels/{id}", notifChannel.Get)
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Post("/api/v1/notification-channels", notifChannel.Create)
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Put("/api/v1/notification-channels/{id}", notifChannel.Update)
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Delete("/api/v1/notification-channels/{id}", notifChannel.Delete)
		r.With(rbac.RequirePermission(models.PermNotificationsManage)).Post("/api/v1/notification-channels/{id}/test", notifChannel.Test)

		// --- Reports (reports.read / reports.write) ---
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports", report.List)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/generated", reportGenH.ListGenerated)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/generated/{id}/download", reportGenH.DownloadReport)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/sla", phase2.List("sla_definitions"))
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/mttr", phase2.List("incidents"))
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/availability", report.Devices)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/isp", phase2.List("isp_metrics"))
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/reports/top-offenders", report.Devices)
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Get("/api/v1/reports/scheduled", phase2.List("scheduled_reports"))
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Post("/api/v1/reports/scheduled", phase2.Create("scheduled_reports"))
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Put("/api/v1/reports/scheduled/{id}", phase2.Update("scheduled_reports"))
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Delete("/api/v1/reports/scheduled/{id}", phase2.Delete("scheduled_reports"))
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Post("/api/v1/reports/scheduled/{id}/run", reportGenH.RunScheduledReport)
		r.With(rbac.RequirePermission(models.PermReportsWrite)).Post("/api/v1/reports/generate", reportGenH.GenerateReport)

		// --- Reports (legacy aliases) ---
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/reports/summary", report.Summary)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/reports/timeseries", report.Timeseries)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/reports/devices", report.Devices)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/reports/alerts", report.Alerts)
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/reports/export", report.Export)

		// --- Locations / Topology (settings.write / devices.read) ---
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/locations", campusH.ListLocations)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/locations/{id}", campusH.GetLocation)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/locations/{id}/tree", campusH.GetLocationTree)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/locations/{id}/status", campusH.LocationStatus)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/locations/{id}/devices", campusH.LocationDevices)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Post("/api/v1/locations", campusH.CreateLocation)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Put("/api/v1/locations/{id}", campusH.UpdateLocation)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Delete("/api/v1/locations/{id}", campusH.DeleteLocation)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Post("/api/v1/locations/{id}/move", campusH.MoveLocation)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/topology", campusH.DependencyTree)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/topology/map", campusH.DependencyTree)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/topology/dependency-tree", campusH.DependencyTree)
		r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/outages/root-cause", campusH.RootCauseOutages)

		// --- Subnets (settings.write) ---
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/subnets", phase2.List("subnets"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Post("/api/v1/subnets", phase2.Create("subnets"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/subnets/{id}", phase2.Get("subnets"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Put("/api/v1/subnets/{id}", phase2.Update("subnets"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Delete("/api/v1/subnets/{id}", phase2.Delete("subnets"))

		// --- Import (reports.read / import.execute) ---
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/import/history", phase2.List("discovery_jobs"))
		r.With(rbac.RequirePermission(models.PermReportsRead)).Get("/api/v1/import/template", campusH.ImportTemplate)
		r.With(rbac.RequirePermission(models.PermImportExecute)).Post("/api/v1/import/devices", campusH.ImportPreview)
		r.With(rbac.RequirePermission(models.PermImportExecute)).Post("/api/v1/import/devices/confirm", campusH.ImportExecute)

		// --- Discovery (discovery.execute) ---
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Post("/api/v1/discovery/scan", discH.StartScan)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Get("/api/v1/discovery/jobs", discH.ListJobs)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Get("/api/v1/discovery/jobs/{id}", discH.GetJob)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Get("/api/v1/discovery/jobs/{id}/results", discH.GetJobResults)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Post("/api/v1/discovery/results/{id}/approve", discH.ApproveResult)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Post("/api/v1/discovery/results/{id}/reject", discH.RejectResult)
		r.With(rbac.RequirePermission(models.PermDiscoveryExecute)).Post("/api/v1/discovery/results/bulk-approve", discH.BulkApprove)

		// --- Status Page (status_page.manage) ---
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Get("/api/v1/status-page/services", phase2.List("status_page_services"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Get("/api/v1/status-page/services/{id}", phase2.Get("status_page_services"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Post("/api/v1/status-page/services", phase2.Create("status_page_services"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Put("/api/v1/status-page/services/{id}", phase2.Update("status_page_services"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Delete("/api/v1/status-page/services/{id}", phase2.Delete("status_page_services"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Post("/api/v1/status-page/services/{id}/devices", statusPageH.AddServiceDevice)
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Delete("/api/v1/status-page/services/{id}/devices/{did}", statusPageH.RemoveServiceDevice)
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Get("/api/v1/status-page/services/{id}/devices", statusPageH.ListServiceDevices)
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Post("/api/v1/status-page/incidents", phase2.Create("status_page_incidents"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Put("/api/v1/status-page/incidents/{id}", phase2.Update("status_page_incidents"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Post("/api/v1/status-page/incidents/{id}/update", phase2.Create("status_page_incident_updates"))
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Get("/api/v1/status-page/incidents/{id}/updates", statusPageH.ListIncidentUpdates)
		r.With(rbac.RequirePermission(models.PermStatusPageManage)).Post("/api/v1/status-page/incidents/{id}/services", statusPageH.LinkIncidentServices)

		// --- Maintenance (maintenance.write) ---
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Get("/api/v1/maintenance", phase2.List("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Get("/api/v1/maintenance/active", phase2.List("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Get("/api/v1/maintenance/calendar", phase2.List("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Get("/api/v1/maintenance/{id}", phase2.Get("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Post("/api/v1/maintenance", phase2.Create("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Put("/api/v1/maintenance/{id}", phase2.Update("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Delete("/api/v1/maintenance/{id}", phase2.Delete("maintenance_windows"))
		r.With(rbac.RequirePermission(models.PermMaintenanceWrite)).Post("/api/v1/maintenance/{id}/toggle", phase2.Update("maintenance_windows"))

		// --- Contacts & Escalation (contacts.write) ---
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/contacts", phase2.List("contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/contacts/{id}", phase2.Get("contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/contacts", phase2.Create("contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Put("/api/v1/contacts/{id}", phase2.Update("contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Delete("/api/v1/contacts/{id}", phase2.Delete("contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/contacts/{id}/devices", phase2.List("device_contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/devices/{id}/contacts", phase2.Create("device_contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Delete("/api/v1/devices/{id}/contacts/{cid}", phase2.Delete("device_contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/locations/{id}/contacts", phase2.Create("device_contacts"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/escalation-policies", phase2.List("escalation_policies"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/escalation-policies/{id}/steps", phase2.List("escalation_steps"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/escalation-policies", phase2.Create("escalation_policies"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Put("/api/v1/escalation-policies/{id}", phase2.Update("escalation_policies"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Delete("/api/v1/escalation-policies/{id}", phase2.Delete("escalation_policies"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/escalation-policies/{id}/steps", phase2.Create("escalation_steps"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/devices/{id}/resolve-contacts", contactH.ResolveContacts)
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/alerts/{id}/escalate", contactH.EscalationStart)
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Post("/api/v1/alerts/{id}/cancel-escalation", contactH.EscalationCancel)
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/alerts/{id}/escalation-status", contactH.EscalationStatus)
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/notification-log", contactH.NotificationLog)
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/oncall", phase2.List("oncall_schedules"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Get("/api/v1/oncall/schedule", phase2.List("oncall_schedules"))
		r.With(rbac.RequirePermission(models.PermContactsWrite)).Put("/api/v1/oncall/{id}/override", phase2.Update("oncall_schedules"))

		// --- Incidents (incidents.write) ---
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents", phase2.List("incidents"))
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents/stats", incidentH.GetIncidentStats)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents/sla-report", incidentH.GetSLAReport)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents/{id}", phase2.Get("incidents"))
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents", incidentH.CreateIncident)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Put("/api/v1/incidents/{id}", phase2.Update("incidents"))
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents/{id}/note", incidentH.AddTimelineEntry)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents/{id}/assign", incidentH.AssignIncident)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents/{id}/resolve", incidentH.ResolveIncident)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents/{id}/close", incidentH.CloseIncident)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Post("/api/v1/incidents/{id}/acknowledge", incidentH.AcknowledgeIncident)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents/{id}/timeline", incidentH.GetTimeline)
		r.With(rbac.RequirePermission(models.PermIncidentsWrite)).Get("/api/v1/incidents/{id}/devices", incidentH.GetIncidentDevices)

		// --- Roles & Users (users.manage) ---
		r.With(rbac.RequirePermission(models.PermUsersManage)).Get("/api/v1/roles", phase2.List("roles"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Post("/api/v1/roles", phase2.Create("roles"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Put("/api/v1/roles/{id}", phase2.Update("roles"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Delete("/api/v1/roles/{id}", phase2.Delete("roles"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Get("/api/v1/users", phase2.List("users"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Get("/api/v1/users/{id}", phase2.Get("users"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Post("/api/v1/users", authH.CreateUser)
		r.With(rbac.RequirePermission(models.PermUsersManage)).Put("/api/v1/users/{id}", phase2.Update("users"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Delete("/api/v1/users/{id}", authH.DeleteUser)
		r.With(rbac.RequirePermission(models.PermUsersManage)).Put("/api/v1/users/{id}/role", phase2.Update("users"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Get("/api/v1/user-scopes", phase2.List("user_scopes"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Put("/api/v1/users/{id}/scopes", phase2.Create("user_scopes"))
		r.With(rbac.RequirePermission(models.PermUsersManage)).Delete("/api/v1/users/{id}/scopes/{sid}", phase2.Delete("user_scopes"))

		// --- ISP Links (settings.write) ---
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links", phase2.List("isp_links"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links/comparison", ispH.Comparison)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links/{id}", phase2.Get("isp_links"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links/{id}/metrics", ispH.MetricsSummary)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links/{id}/metrics/timeseries", ispH.MetricsTimeSeries)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Get("/api/v1/isp-links/{id}/sla", ispH.LinkSLA)
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Post("/api/v1/isp-links", phase2.Create("isp_links"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Put("/api/v1/isp-links/{id}", phase2.Update("isp_links"))
		r.With(rbac.RequirePermission(models.PermSettingsWrite)).Delete("/api/v1/isp-links/{id}", phase2.Delete("isp_links"))

		// --- SLA (sla.manage) ---
		r.With(rbac.RequirePermission(models.PermSLAManage)).Get("/api/v1/sla", phase2.List("sla_definitions"))
		r.With(rbac.RequirePermission(models.PermSLAManage)).Put("/api/v1/sla/{id}", phase2.Update("sla_definitions"))

		// --- Service Templates (devices.read / devices.write) ---
		if svcTmplH != nil {
			r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/service-templates", svcTmplH.ListTemplates)
			r.With(rbac.RequirePermission(models.PermDevicesRead)).Get("/api/v1/service-templates/{name}", svcTmplH.GetTemplate)
			r.With(rbac.RequirePermission(models.PermDevicesWrite)).Post("/api/v1/service-templates/apply", svcTmplH.ApplyTemplate)
		}

		// --- Simulator (admin only) ---
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
