package retention

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/require"
)

type mockRetDB struct {
	pruneMetricsFn func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneFlowsFn   func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneAlertsFn  func(ctx context.Context, olderThan time.Time) (int64, error)
}

func (m *mockRetDB) Connect(ctx context.Context) error                       { return nil }
func (m *mockRetDB) Close() error                                            { return nil }
func (m *mockRetDB) Ping(ctx context.Context) error                          { return nil }
func (m *mockRetDB) RunMigrations(ctx context.Context) error                 { return nil }
func (m *mockRetDB) GetDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockRetDB) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) { return nil, nil }
func (m *mockRetDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteDevice(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	return nil
}
func (m *mockRetDB) GetEnabledDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockRetDB) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) {
	return nil, nil
}
func (m *mockRetDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) { return nil, nil }
func (m *mockRetDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteSensor(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB) RecordMetric(ctx context.Context, metric *models.Metric) error { return nil }
func (m *mockRetDB) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) { return nil, nil }
func (m *mockRetDB) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	return nil, nil
}
func (m *mockRetDB) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	return nil, nil
}
func (m *mockRetDB) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockRetDB) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	return nil, nil
}
func (m *mockRetDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	return nil, nil
}
func (m *mockRetDB) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error) { return nil, nil }
func (m *mockRetDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	return nil
}
func (m *mockRetDB) DeleteAlert(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	return models.AlertCounts{}, nil
}
func (m *mockRetDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) { return nil, nil }
func (m *mockRetDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteAlertRule(ctx context.Context, id int64) error               { return nil }
func (m *mockRetDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error { return nil }
func (m *mockRetDB) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteNotificationChannel(ctx context.Context, id int64) error        { return nil }
func (m *mockRetDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error { return nil }
func (m *mockRetDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	return nil, nil
}
func (m *mockRetDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	return nil, nil
}
func (m *mockRetDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	return nil
}
func (m *mockRetDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) { return nil, nil }
func (m *mockRetDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteUser(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteAPIKey(ctx context.Context, id int64) error           { return nil }
func (m *mockRetDB) RecordFlows(ctx context.Context, flows []models.Flow) error { return nil }
func (m *mockRetDB) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	return nil, nil
}
func (m *mockRetDB) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}
func (m *mockRetDB) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockRetDB) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	return models.FlowSummaryStats{}, nil
}
func (m *mockRetDB) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockRetDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockRetDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	return nil, nil
}
func (m *mockRetDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	return nil
}
func (m *mockRetDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *mockRetDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *mockRetDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
	return nil
}
func (m *mockRetDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	return nil, nil
}
func (m *mockRetDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteDashboard(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneMetricsFn != nil {
		return m.pruneMetricsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneFlowsFn != nil {
		return m.pruneFlowsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneAlertsFn != nil {
		return m.pruneAlertsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB) GetDashboardStats(ctx context.Context) (map[string]any, error) { return nil, nil }
func (m *mockRetDB) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *mockRetDB) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	return nil, nil
}
func (m *mockRetDB) DeleteRefreshToken(ctx context.Context, tokenHash string) error    { return nil }
func (m *mockRetDB) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *mockRetDB) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)    { return 0, nil }
func (m *mockRetDB) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	return nil
}
func (m *mockRetDB) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	return nil, nil
}
func (m *mockRetDB) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *mockRetDB) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *mockRetDB) InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error {
	return nil
}
func (m *mockRetDB) GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB) GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB) GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB) GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB) RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error {
	return nil
}
func (m *mockRetDB) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	return nil, nil
}

func TestNew(t *testing.T) {
	t.Parallel()
	db := &mockRetDB{}
	s := New(db, 30, 7, 90)
	require.NotNil(t, s)
}

func TestScheduler_StartStop(t *testing.T) {
	t.Parallel()
	db := &mockRetDB{}
	s := New(db, 30, 7, 90)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	cancel()
	s.Stop()
}

func TestScheduler_Stop_BeforeStart(t *testing.T) {
	t.Parallel()
	db := &mockRetDB{}
	s := New(db, 30, 7, 90)
	s.Stop()
}
