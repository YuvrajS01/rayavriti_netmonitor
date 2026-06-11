package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDB struct {
	getEnabledDevicesFn func(ctx context.Context) ([]models.Device, error)
	getDeviceFn         func(ctx context.Context, id int64) (*models.Device, error)
	recordMetricFn      func(ctx context.Context, m *models.Metric) error
	getLatestMetricsFn  func(ctx context.Context) ([]models.Metric, error)
}

func (m *mockDB) Connect(ctx context.Context) error                     { return nil }
func (m *mockDB) Close() error                                          { return nil }
func (m *mockDB) Ping(ctx context.Context) error                        { return nil }
func (m *mockDB) RunMigrations(ctx context.Context) error               { return nil }
func (m *mockDB) GetDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}
	return nil, nil
}
func (m *mockDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) { return nil, nil }
func (m *mockDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) { return nil, nil }
func (m *mockDB) DeleteDevice(ctx context.Context, id int64) error                         { return nil }
func (m *mockDB) UpdateDeviceStatus(ctx context.Context, id int64, status string) error     { return nil }
func (m *mockDB) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	if m.getEnabledDevicesFn != nil {
		return m.getEnabledDevicesFn(ctx)
	}
	return nil, nil
}
func (m *mockDB) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error)       { return nil, nil }
func (m *mockDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error)                 { return nil, nil }
func (m *mockDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error)      { return nil, nil }
func (m *mockDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) DeleteSensor(ctx context.Context, id int64) error                               { return nil }
func (m *mockDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) RecordMetric(ctx context.Context, metric *models.Metric) error {
	if m.recordMetricFn != nil {
		return m.recordMetricFn(ctx, metric)
	}
	return nil
}
func (m *mockDB) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	if m.getLatestMetricsFn != nil {
		return m.getLatestMetricsFn(ctx)
	}
	return nil, nil
}
func (m *mockDB) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockDB) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	return nil, nil
}
func (m *mockDB) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	return nil, nil
}
func (m *mockDB) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockDB) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	return nil, nil
}
func (m *mockDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) { return nil, nil }
func (m *mockDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	return nil, nil
}
func (m *mockDB) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	return nil, 0, nil
}
func (m *mockDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error)            { return nil, nil }
func (m *mockDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error)  { return nil, nil }
func (m *mockDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error { return nil }
func (m *mockDB) DeleteAlert(ctx context.Context, id int64) error                          { return nil }
func (m *mockDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error)           { return models.AlertCounts{}, nil }
func (m *mockDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	return nil, nil
}
func (m *mockDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	return nil, nil
}
func (m *mockDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) { return nil, nil }
func (m *mockDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) { return nil, nil }
func (m *mockDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) DeleteAlertRule(ctx context.Context, id int64) error                  { return nil }
func (m *mockDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error    { return nil }
func (m *mockDB) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockDB) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockDB) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockDB) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *mockDB) DeleteNotificationChannel(ctx context.Context, id int64) error { return nil }
func (m *mockDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error { return nil }
func (m *mockDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	return nil, nil
}
func (m *mockDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	return nil, nil
}
func (m *mockDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error { return nil }
func (m *mockDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) { return nil, nil }
func (m *mockDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) { return nil, nil }
func (m *mockDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) { return nil, nil }
func (m *mockDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) { return nil, nil }
func (m *mockDB) DeleteUser(ctx context.Context, id int64) error                              { return nil }
func (m *mockDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error)       { return nil, nil }
func (m *mockDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error)  { return nil, nil }
func (m *mockDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) { return nil, nil }
func (m *mockDB) DeleteAPIKey(ctx context.Context, id int64) error                            { return nil }
func (m *mockDB) RecordFlows(ctx context.Context, flows []models.Flow) error                  { return nil }
func (m *mockDB) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	return nil, 0, nil
}
func (m *mockDB) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	return nil, nil
}
func (m *mockDB) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}
func (m *mockDB) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	return nil, nil
}
func (m *mockDB) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	return models.FlowSummaryStats{}, nil
}
func (m *mockDB) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) { return nil, nil }
func (m *mockDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error)         { return nil, nil }
func (m *mockDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	return nil
}
func (m *mockDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *mockDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *mockDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
	return nil
}
func (m *mockDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	return nil, nil
}
func (m *mockDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) { return nil, nil }
func (m *mockDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error)       { return nil, nil }
func (m *mockDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockDB) DeleteDashboard(ctx context.Context, id int64) error                       { return nil }
func (m *mockDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error)      { return 0, nil }
func (m *mockDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error)        { return 0, nil }
func (m *mockDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error)       { return 0, nil }
func (m *mockDB) GetDashboardStats(ctx context.Context) (map[string]any, error)             { return nil, nil }

func newTestHub() *websocket.Hub {
	return websocket.NewHub("test-secret", nil)
}

func TestNew(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	require.NotNil(t, s)
	assert.Equal(t, 0, s.JobCount())
}

func TestScheduler_StartStop_NoDevices(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	assert.Equal(t, 0, s.JobCount())

	s.Stop()
}

func TestScheduler_StartStop_WithDevices(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "router-1", Protocol: "ping", Interval: 10},
				{ID: 2, Name: "switch-1", Protocol: "snmp", Interval: 15},
			}, nil
		},
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Enabled: true, Interval: 10}, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, s.JobCount(), 2)

	s.Stop()
}

func TestScheduler_CollectOnce_UnknownProtocol(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)

	device := models.Device{
		ID:       1,
		Name:     "test-device",
		Protocol: "unknown_protocol",
	}

	// Should not panic
	s.collectOnce(context.Background(), device)
}

func TestScheduler_JobCount_Tracking(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
			}, nil
		},
		getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Enabled: true, Interval: 10}, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert.Equal(t, 0, s.JobCount())

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, s.JobCount(), 1)

	s.Stop()
}

func TestScheduler_StopIdempotent(t *testing.T) {
	t.Parallel()

	db := &mockDB{}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	s.Stop()
	s.Stop()
}

func TestScheduler_Reconcile(t *testing.T) {
	t.Parallel()

	callCount := 0
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			callCount++
			if callCount == 1 {
				return []models.Device{
					{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
				}, nil
			}
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	s.reconcile(ctx)
	s.Stop()
}

func TestScheduler_CollectOnce_DBError(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, assert.AnError
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)

	device := models.Device{ID: 1, Name: "d1", Protocol: "unknown"}
	s.collectOnce(context.Background(), device)
}

func TestScheduler_CollectOnce_NilResult(t *testing.T) {
	t.Parallel()

	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, nil
		},
	}
	reg := collectors.NewRegistry()
	hub := newTestHub()

	s := New(db, reg, hub, nil, 30)

	device := models.Device{ID: 1, Name: "d1", Protocol: "nonexistent"}
	s.collectOnce(context.Background(), device)
}
