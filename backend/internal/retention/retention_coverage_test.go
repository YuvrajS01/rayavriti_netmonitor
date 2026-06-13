package retention

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
)

type mockRetDB2 struct {
	pruneMetricsFn       func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneFlowsFn         func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneAlertsFn        func(ctx context.Context, olderThan time.Time) (int64, error)
	getCaptureSessionsFn func(ctx context.Context) ([]models.CaptureSession, error)
	stopCaptureSessionFn func(ctx context.Context, id int64, stats models.CaptureSessionStats) error
}

func (m *mockRetDB2) Connect(ctx context.Context) error                       { return nil }
func (m *mockRetDB2) Close() error                                            { return nil }
func (m *mockRetDB2) Ping(ctx context.Context) error                          { return nil }
func (m *mockRetDB2) RunMigrations(ctx context.Context) error                 { return nil }
func (m *mockRetDB2) GetDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockRetDB2) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB2) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteDevice(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	return nil
}
func (m *mockRetDB2) GetEnabledDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockRetDB2) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) {
	return nil, nil
}
func (m *mockRetDB2) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB2) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteSensor(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockRetDB2) RecordMetric(ctx context.Context, metric *models.Metric) error { return nil }
func (m *mockRetDB2) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) { return nil, nil }
func (m *mockRetDB2) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB2) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	return nil, nil
}
func (m *mockRetDB2) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	return nil, nil
}
func (m *mockRetDB2) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockRetDB2) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	return nil, nil
}
func (m *mockRetDB2) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB2) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB2) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB2) GetAlert(ctx context.Context, id int64) (*models.Alert, error) { return nil, nil }
func (m *mockRetDB2) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	return nil
}
func (m *mockRetDB2) DeleteAlert(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	return models.AlertCounts{}, nil
}
func (m *mockRetDB2) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB2) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB2) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) { return nil, nil }
func (m *mockRetDB2) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteAlertRule(ctx context.Context, id int64) error               { return nil }
func (m *mockRetDB2) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error { return nil }
func (m *mockRetDB2) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB2) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteNotificationChannel(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	return nil
}
func (m *mockRetDB2) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	return nil, assert.AnError
}
func (m *mockRetDB2) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	return nil
}
func (m *mockRetDB2) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB2) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB2) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteUser(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB2) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB2) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteAPIKey(ctx context.Context, id int64) error           { return nil }
func (m *mockRetDB2) RecordFlows(ctx context.Context, flows []models.Flow) error { return nil }
func (m *mockRetDB2) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	return nil, 0, nil
}
func (m *mockRetDB2) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	return nil, nil
}
func (m *mockRetDB2) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}
func (m *mockRetDB2) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockRetDB2) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	return models.FlowSummaryStats{}, nil
}
func (m *mockRetDB2) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockRetDB2) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockRetDB2) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	if m.getCaptureSessionsFn != nil {
		return m.getCaptureSessionsFn(ctx)
	}
	return nil, nil
}
func (m *mockRetDB2) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	if m.stopCaptureSessionFn != nil {
		return m.stopCaptureSessionFn(ctx, id, stats)
	}
	return nil
}
func (m *mockRetDB2) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *mockRetDB2) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *mockRetDB2) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
	return nil
}
func (m *mockRetDB2) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	return nil, nil
}
func (m *mockRetDB2) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB2) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB2) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteDashboard(ctx context.Context, id int64) error { return nil }
func (m *mockRetDB2) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneMetricsFn != nil {
		return m.pruneMetricsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB2) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneFlowsFn != nil {
		return m.pruneFlowsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB2) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneAlertsFn != nil {
		return m.pruneAlertsFn(ctx, olderThan)
	}
	return 0, nil
}
func (m *mockRetDB2) GetDashboardStats(ctx context.Context) (map[string]any, error) { return nil, nil }
func (m *mockRetDB2) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *mockRetDB2) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	return nil, nil
}
func (m *mockRetDB2) DeleteRefreshToken(ctx context.Context, tokenHash string) error    { return nil }
func (m *mockRetDB2) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *mockRetDB2) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)    { return 0, nil }

// ── prune calls all three prune functions ─────────────────────────────────────

func TestPrune_Coverage_CallsAllPruneFunctions(t *testing.T) {
	t.Parallel()
	metricsCalled := false
	flowsCalled := false
	alertsCalled := false

	db := &mockRetDB2{
		pruneMetricsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			metricsCalled = true
			return 10, nil
		},
		pruneFlowsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			flowsCalled = true
			return 20, nil
		},
		pruneAlertsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			alertsCalled = true
			return 5, nil
		},
	}

	s := New(db, 30, 7, 90)
	s.prune(context.Background())

	assert.True(t, metricsCalled)
	assert.True(t, flowsCalled)
	assert.True(t, alertsCalled)
}

// ── prune with DB errors (doesn't crash) ─────────────────────────────────────

func TestPrune_Coverage_DBErrors(t *testing.T) {
	t.Parallel()
	db := &mockRetDB2{
		pruneMetricsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, fmt.Errorf("metrics prune failed")
		},
		pruneFlowsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, fmt.Errorf("flows prune failed")
		},
		pruneAlertsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, fmt.Errorf("alerts prune failed")
		},
	}

	s := New(db, 30, 7, 90)
	s.prune(context.Background())
}

// ── prune prunes old capture sessions ─────────────────────────────────────────

func TestPrune_Coverage_CaptureSessions(t *testing.T) {
	t.Parallel()
	stopCalled := map[int64]bool{}

	db := &mockRetDB2{
		getCaptureSessionsFn: func(ctx context.Context) ([]models.CaptureSession, error) {
			return []models.CaptureSession{
				{
					ID:           1,
					Status:       "stopped",
					StartedAt:    time.Now().AddDate(0, 0, -60),
					TotalPackets: 100,
					TotalBytes:   5000,
				},
				{
					ID:        2,
					Status:    "running",
					StartedAt: time.Now().AddDate(0, 0, -60),
				},
				{
					ID:           3,
					Status:       "stopped",
					StartedAt:    time.Now().AddDate(0, 0, -10),
					TotalPackets: 50,
					TotalBytes:   2000,
				},
			}, nil
		},
		stopCaptureSessionFn: func(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
			stopCalled[id] = true
			return nil
		},
	}

	s := New(db, 30, 7, 90)
	s.prune(context.Background())

	assert.True(t, stopCalled[1])
	assert.False(t, stopCalled[2])
	assert.False(t, stopCalled[3])
}

// ── prune with sessions error ────────────────────────────────────────────────

func TestPrune_Coverage_SessionsError(t *testing.T) {
	t.Parallel()
	db := &mockRetDB2{
		getCaptureSessionsFn: func(ctx context.Context) ([]models.CaptureSession, error) {
			return nil, fmt.Errorf("sessions query failed")
		},
	}

	s := New(db, 30, 7, 90)
	s.prune(context.Background())
}

// ── prune with session stop error ────────────────────────────────────────────

func TestPrune_Coverage_SessionStopError(t *testing.T) {
	t.Parallel()
	db := &mockRetDB2{
		getCaptureSessionsFn: func(ctx context.Context) ([]models.CaptureSession, error) {
			return []models.CaptureSession{
				{
					ID:           1,
					Status:       "stopped",
					StartedAt:    time.Now().AddDate(0, 0, -60),
					TotalPackets: 100,
					TotalBytes:   5000,
				},
			}, nil
		},
		stopCaptureSessionFn: func(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
			return fmt.Errorf("stop failed")
		},
	}

	s := New(db, 30, 7, 90)
	s.prune(context.Background())
}

// ── New creates scheduler correctly ──────────────────────────────────────────

func TestNew_Coverage_CreatesScheduler(t *testing.T) {
	t.Parallel()
	db := &mockRetDB2{}
	s := New(db, 30, 7, 90)
	assert.NotNil(t, s)
	assert.Equal(t, 30, s.metricsRetentionDays)
	assert.Equal(t, 7, s.flowsRetentionDays)
	assert.Equal(t, 90, s.alertsRetentionDays)
}

// ── Start and Stop ───────────────────────────────────────────────────────────

func TestScheduler_Coverage_StartStop(t *testing.T) {
	t.Parallel()
	db := &mockRetDB2{
		pruneMetricsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, nil
		},
		pruneFlowsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, nil
		},
		pruneAlertsFn: func(ctx context.Context, olderThan time.Time) (int64, error) {
			return 0, nil
		},
	}

	s := New(db, 30, 7, 90)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	cancel()
	s.Stop()
}
