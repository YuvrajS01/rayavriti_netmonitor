package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// PoolProvider is satisfied by *Postgres and any wrapper (e.g. *CachedDatabase).
type PoolProvider interface {
	Pool() *pgxpool.Pool
}

type DeviceFilter struct {
	Status     string
	Protocol   string
	Enabled    *bool
	Search     string
	SortBy     string
	SortDir    string
	Limit      int
	Offset     int
	LocationID *int64
}

type RefreshToken struct {
	ID        int64
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Phase2Summary struct {
	Locations          int `json:"locations"`
	Subnets            int `json:"subnets"`
	Contacts           int `json:"contacts"`
	Incidents          int `json:"incidents"`
	MaintenanceWindows int `json:"maintenanceWindows"`
	StatusServices     int `json:"statusServices"`
	DiscoveryJobs      int `json:"discoveryJobs"`
	ISPLinks           int `json:"ispLinks"`
	ScheduledReports   int `json:"scheduledReports"`
}

type Phase2Store interface {
	ListPhase2(ctx context.Context, resource string, filters map[string]string) ([]map[string]any, error)
	GetPhase2(ctx context.Context, resource string, id int64) (map[string]any, error)
	CreatePhase2(ctx context.Context, resource string, values map[string]any) (map[string]any, error)
	UpdatePhase2(ctx context.Context, resource string, id int64, values map[string]any) (map[string]any, error)
	DeletePhase2(ctx context.Context, resource string, id int64) error
	Phase2Summary(ctx context.Context) (Phase2Summary, error)
}

type Database interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	RunMigrations(ctx context.Context) error

	// Devices
	GetDevices(ctx context.Context) ([]models.Device, error)
	GetDevice(ctx context.Context, id int64) (*models.Device, error)
	GetDevicesFiltered(ctx context.Context, f DeviceFilter) ([]models.Device, int, error)
	CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error)
	UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error)
	DeleteDevice(ctx context.Context, id int64) error
	UpdateDeviceStatus(ctx context.Context, id int64, status string) error
	GetEnabledDevices(ctx context.Context) ([]models.Device, error)
	GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error)

	// Sensors
	GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error)
	GetSensor(ctx context.Context, id int64) (*models.Sensor, error)
	CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error)
	UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error)
	DeleteSensor(ctx context.Context, id int64) error
	GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error)

	// Metrics
	RecordMetric(ctx context.Context, m *models.Metric) error
	GetLatestMetrics(ctx context.Context) ([]models.Metric, error)
	GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error)
	GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error)
	GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error)
	GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error)
	GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error)
	GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error)
	QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error)
	ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error)
	GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error)

	// Alerts
	GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error)
	GetAlert(ctx context.Context, id int64) (*models.Alert, error)
	CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error)
	UpdateAlertStatus(ctx context.Context, id int64, status, by string) error
	DeleteAlert(ctx context.Context, id int64) error
	GetAlertCounts(ctx context.Context) (models.AlertCounts, error)
	FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error)
	FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error)
	GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error)

	// Alert Rules
	GetAlertRules(ctx context.Context) ([]models.AlertRule, error)
	GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error)
	CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error)
	UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error)
	DeleteAlertRule(ctx context.Context, id int64) error
	ToggleAlertRule(ctx context.Context, id int64, enabled bool) error

	// Notification Channels
	GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error)
	GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error)
	CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error)
	UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error)
	DeleteNotificationChannel(ctx context.Context, id int64) error

	// Alert History
	RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error
	GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error)

	// Alert Rule State
	GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error)
	UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error

	// Users
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	CreateUser(ctx context.Context, u *models.User) (*models.User, error)
	UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error)
	DeleteUser(ctx context.Context, id int64) error
	GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error)
	CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error)
	GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error

	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteRefreshTokensByUser(ctx context.Context, userID int64) error
	CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)

	// Flows
	RecordFlows(ctx context.Context, flows []models.Flow) error
	GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error)
	GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error)
	GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error)
	GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error)
	GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error)

	// Captures
	CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error)
	GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error)
	GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error)
	StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error
	InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error
	GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error)

	// Port Scans
	UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) (int, error)
	GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error)

	// Dashboards
	GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error)
	GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error)
	SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error)
	DeleteDashboard(ctx context.Context, id int64) error

	// Retention
	PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error)
	PruneFlows(ctx context.Context, olderThan time.Time) (int64, error)
	PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error)

	// Health Scores
	UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error
	GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error)
	GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error)
	GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error)
	InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error
	GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error)
	GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error)
	GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error)
	GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error)

	// Stats
	GetDashboardStats(ctx context.Context) (map[string]any, error)

	// Suppressed Alerts
	RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error

	// RBAC
	GetRolePermissions(ctx context.Context, roleID int64) ([]string, error)
}
