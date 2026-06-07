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

	// Metrics
	RecordMetric(ctx context.Context, m *models.Metric) error
	GetLatestMetrics(ctx context.Context) ([]models.Metric, error)
	GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error)
	GetMetricsSummary(ctx context.Context, from, to time.Time) (map[string]any, error)

	// Alerts
	GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error)
	GetAlert(ctx context.Context, id int64) (*models.Alert, error)
	CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error)
	UpdateAlertStatus(ctx context.Context, id int64, status, by string) error
	DeleteAlert(ctx context.Context, id int64) error

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
