package database

import (
	"context"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type Database interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	RunMigrations(ctx context.Context) error

	// Devices
	GetDevices(ctx context.Context) ([]models.Device, error)
	GetDevice(ctx context.Context, id int64) (*models.Device, error)
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
	GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error)
	GetMetricsSummary(ctx context.Context, from, to time.Time) (map[string]any, error)
	GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error)
	QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error)
	GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error)

	// Alerts
	GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error)
	GetAlert(ctx context.Context, id int64) (*models.Alert, error)
	CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error)
	UpdateAlertStatus(ctx context.Context, id int64, status, by string) error
	DeleteAlert(ctx context.Context, id int64) error
	GetAlertCounts(ctx context.Context) (models.AlertCounts, error)
	FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error)

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
	GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error)
	CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error)
	GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error

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

	// Port Scans
	UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error
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

	// Stats
	GetDashboardStats(ctx context.Context) (map[string]any, error)
}
