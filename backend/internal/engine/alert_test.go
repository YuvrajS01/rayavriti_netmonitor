package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDB struct {
	getAlertRulesFn      func(ctx context.Context) ([]models.AlertRule, error)
	getAlertRuleStateFn  func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error)
	upsertAlertRuleStateFn func(ctx context.Context, s *models.AlertRuleState) error
	createAlertFn        func(ctx context.Context, a *models.Alert) (*models.Alert, error)
	getAlertsFn          func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error)
	findActiveAlertByRuleAndDeviceFn func(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error)
	updateAlertStatusFn  func(ctx context.Context, id int64, status, by string) error
	recordAlertHistoryFn func(ctx context.Context, h *models.AlertHistory) error
	getNotificationChannelsFn func(ctx context.Context) ([]models.NotificationChannel, error)
}

func (m *mockDB) Connect(ctx context.Context) error                     { return nil }
func (m *mockDB) Close() error                                          { return nil }
func (m *mockDB) Ping(ctx context.Context) error                        { return nil }
func (m *mockDB) RunMigrations(ctx context.Context) error               { return nil }
func (m *mockDB) GetDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) { return nil, nil }
func (m *mockDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) { return nil, nil }
func (m *mockDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) { return nil, nil }
func (m *mockDB) DeleteDevice(ctx context.Context, id int64) error { return nil }
func (m *mockDB) UpdateDeviceStatus(ctx context.Context, id int64, status string) error { return nil }
func (m *mockDB) GetEnabledDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) { return nil, nil }
func (m *mockDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) { return nil, nil }
func (m *mockDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) { return nil, nil }
func (m *mockDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) { return nil, nil }
func (m *mockDB) DeleteSensor(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) { return nil, nil }
func (m *mockDB) RecordMetric(ctx context.Context, metric *models.Metric) error { return nil }
func (m *mockDB) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) { return nil, nil }
func (m *mockDB) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) { return nil, nil }
func (m *mockDB) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) { return nil, nil }
func (m *mockDB) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) { return nil, nil }
func (m *mockDB) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) { return nil, nil }
func (m *mockDB) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) { return nil, nil }
func (m *mockDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) { return nil, nil }
func (m *mockDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) { return nil, nil }
func (m *mockDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) { return nil, nil }
func (m *mockDB) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	if m.getAlertsFn != nil {
		return m.getAlertsFn(ctx, status, limit, offset)
	}
	return nil, 0, nil
}
func (m *mockDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error) { return nil, nil }
func (m *mockDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	if m.createAlertFn != nil {
		return m.createAlertFn(ctx, a)
	}
	return a, nil
}
func (m *mockDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	if m.updateAlertStatusFn != nil {
		return m.updateAlertStatusFn(ctx, id, status, by)
	}
	return nil
}
func (m *mockDB) DeleteAlert(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) { return models.AlertCounts{}, nil }
func (m *mockDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) { return nil, nil }
func (m *mockDB) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	if m.findActiveAlertByRuleAndDeviceFn != nil {
		return m.findActiveAlertByRuleAndDeviceFn(ctx, ruleID, deviceID)
	}
	return nil, nil
}
func (m *mockDB) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) { return nil, nil }
func (m *mockDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) { return nil, nil }
func (m *mockDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	if m.getAlertRulesFn != nil {
		return m.getAlertRulesFn(ctx)
	}
	return nil, nil
}
func (m *mockDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) { return nil, nil }
func (m *mockDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) { return nil, nil }
func (m *mockDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) { return nil, nil }
func (m *mockDB) DeleteAlertRule(ctx context.Context, id int64) error { return nil }
func (m *mockDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error { return nil }
func (m *mockDB) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	if m.getNotificationChannelsFn != nil {
		return m.getNotificationChannelsFn(ctx)
	}
	return nil, nil
}
func (m *mockDB) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) { return nil, nil }
func (m *mockDB) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) { return nil, nil }
func (m *mockDB) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) { return nil, nil }
func (m *mockDB) DeleteNotificationChannel(ctx context.Context, id int64) error { return nil }
func (m *mockDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	if m.recordAlertHistoryFn != nil {
		return m.recordAlertHistoryFn(ctx, h)
	}
	return nil
}
func (m *mockDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) { return nil, nil }
func (m *mockDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	if m.getAlertRuleStateFn != nil {
		return m.getAlertRuleStateFn(ctx, ruleID, deviceID)
	}
	return nil, assert.AnError
}
func (m *mockDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	if m.upsertAlertRuleStateFn != nil {
		return m.upsertAlertRuleStateFn(ctx, s)
	}
	return nil
}
func (m *mockDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) { return nil, nil }
func (m *mockDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) { return nil, nil }
func (m *mockDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) { return nil, nil }
func (m *mockDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) { return nil, nil }
func (m *mockDB) DeleteUser(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) { return nil, nil }
func (m *mockDB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) { return nil, nil }
func (m *mockDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) { return nil, nil }
func (m *mockDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) { return nil, nil }
func (m *mockDB) DeleteAPIKey(ctx context.Context, id int64) error { return nil }
func (m *mockDB) RecordFlows(ctx context.Context, flows []models.Flow) error { return nil }
func (m *mockDB) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) { return nil, 0, nil }
func (m *mockDB) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) { return nil, nil }
func (m *mockDB) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) { return nil, nil }
func (m *mockDB) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) { return nil, nil }
func (m *mockDB) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) { return models.FlowSummaryStats{}, nil }
func (m *mockDB) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) { return nil, nil }
func (m *mockDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) { return nil, nil }
func (m *mockDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) { return nil, nil }
func (m *mockDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error { return nil }
func (m *mockDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error { return nil }
func (m *mockDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) { return nil, nil }
func (m *mockDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error { return nil }
func (m *mockDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) { return nil, nil }
func (m *mockDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) { return nil, nil }
func (m *mockDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) { return nil, nil }
func (m *mockDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) { return nil, nil }
func (m *mockDB) DeleteDashboard(ctx context.Context, id int64) error { return nil }
func (m *mockDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) { return 0, nil }
func (m *mockDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) { return 0, nil }
func (m *mockDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) { return 0, nil }
func (m *mockDB) GetDashboardStats(ctx context.Context) (map[string]any, error) { return nil, nil }
func (m *mockDB) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *mockDB) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) { return nil, nil }
func (m *mockDB) DeleteRefreshToken(ctx context.Context, tokenHash string) error                       { return nil }
func (m *mockDB) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error                    { return nil }
func (m *mockDB) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)                       { return 0, nil }

func int64Ptr(v int64) *int64 { return &v }

func TestRuleAppliesToDevice_Global(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "global"}
	device := &models.Device{ID: 1}
	assert.True(t, RuleAppliesToDevice(rule, device))
}

func TestRuleAppliesToDevice_EmptyScope(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: ""}
	device := &models.Device{ID: 1}
	assert.True(t, RuleAppliesToDevice(rule, device))
}

func TestRuleAppliesToDevice_Device_Matching(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "device", DeviceID: int64Ptr(5)}
	device := &models.Device{ID: 5}
	assert.True(t, RuleAppliesToDevice(rule, device))
}

func TestRuleAppliesToDevice_Device_NonMatching(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "device", DeviceID: int64Ptr(5)}
	device := &models.Device{ID: 10}
	assert.False(t, RuleAppliesToDevice(rule, device))
}

func TestRuleAppliesToDevice_Device_NilDeviceID(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "device", DeviceID: nil}
	device := &models.Device{ID: 1}
	assert.False(t, RuleAppliesToDevice(rule, device))
}

func TestAlertEngine_ProcessMetric_DBError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return nil, assert.AnError
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	err := engine.ProcessMetric(context.Background(), &models.Device{ID: 1}, &models.Metric{DeviceID: 1, Status: "down"}, "")
	require.Error(t, err)
}

func TestAlertEngine_ProcessMetric_NoRules(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return nil, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	err := engine.ProcessMetric(context.Background(), &models.Device{ID: 1}, &models.Metric{DeviceID: 1, Status: "down"}, "")
	require.NoError(t, err)
}

func TestAlertEngine_ProcessMetric_DisabledRule(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{Enabled: false}}, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	err := engine.ProcessMetric(context.Background(), &models.Device{ID: 1}, &models.Metric{DeviceID: 1, Status: "down"}, "")
	require.NoError(t, err)
}

func TestAlertEngine_ProcessMetric_RuleWithNoConditions(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{Enabled: true, Conditions: nil}}, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	err := engine.ProcessMetric(context.Background(), &models.Device{ID: 1}, &models.Metric{DeviceID: 1, Status: "down"}, "")
	require.NoError(t, err)
}

func TestAlertEngine_StartStop(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAlertEngine(db, nil, nil)
	engine.Start(context.Background())
	engine.Stop()
}

func TestAlertEngine_ReloadRules(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	engine := NewAlertEngine(db, nil, nil)
	err := engine.ReloadRules(context.Background())
	require.NoError(t, err)
}

func TestSnapshotFromResults(t *testing.T) {
	t.Parallel()
	results := []ConditionResult{
		{ConditionID: 1, Type: "threshold", Field: "response_time", Result: true},
		{ConditionID: 2, Type: "threshold", Field: "packet_loss", Result: false},
	}
	snap := snapshotFromResults(results)
	assert.NotNil(t, snap)
	assert.NotNil(t, snap["results"])
}
