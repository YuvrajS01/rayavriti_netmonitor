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
	pruneAlertsFn  func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneHealthFn  func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneCaptureFn func(ctx context.Context, olderThan time.Time) (int64, error)
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
func (m *mockRetDB2) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	return nil
}
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
func (m *mockRetDB2) GetAlerts(ctx context.Context, status string, limit, offset int, _ *database.ScopeFilter) ([]models.Alert, int, error) {
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
func (m *mockRetDB2) RevokeAPIKey(ctx context.Context, id int64) error           { return nil }
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
	return nil, nil
}
func (m *mockRetDB2) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	return nil
}
func (m *mockRetDB2) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *mockRetDB2) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *mockRetDB2) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) (int, error) {
	return 0, nil
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
func (m *mockRetDB2) DeleteRefreshToken(ctx context.Context, tokenHash string) error { return nil }
func (m *mockRetDB2) ClaimRefreshToken(ctx context.Context, tokenHash string) (bool, error) {
	return true, nil
}
func (m *mockRetDB2) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *mockRetDB2) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)    { return 0, nil }
func (m *mockRetDB2) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	return nil
}
func (m *mockRetDB2) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	return nil, nil
}
func (m *mockRetDB2) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *mockRetDB2) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *mockRetDB2) InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error {
	return nil
}
func (m *mockRetDB2) GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockRetDB2) GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB2) GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB2) GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockRetDB2) RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error {
	return nil
}
func (m *mockRetDB2) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	return nil, nil
}

// ── prune calls each configured plain-table prune ─────────────────────────────

func TestPrune_Coverage_PrunesConfiguredTables(t *testing.T) {
	t.Parallel()
	cutoffs := map[string]time.Time{}
	record := func(name string) func(context.Context, time.Time) (int64, error) {
		return func(_ context.Context, olderThan time.Time) (int64, error) {
			cutoffs[name] = olderThan
			return 1, nil
		}
	}
	db := &mockRetDB2{
		pruneAlertsFn:  record("alerts"),
		pruneHealthFn:  record("health"),
		pruneCaptureFn: record("capture"),
	}

	s := New(db, Config{AlertsDays: 90, HealthHistoryDays: 30, CaptureDays: 7})
	s.prune(context.Background())

	for name, days := range map[string]int{"alerts": 90, "health": 30, "capture": 7} {
		got, ok := cutoffs[name]
		if !ok {
			t.Fatalf("%s was not pruned", name)
		}
		want := time.Now().AddDate(0, 0, -days)
		assert.WithinDuration(t, want, got, time.Minute, name)
	}
}

// ── zero days keeps rows forever ─────────────────────────────────────────────

func TestPrune_Coverage_ZeroDaysSkips(t *testing.T) {
	t.Parallel()
	called := false
	db := &mockRetDB2{pruneCaptureFn: func(context.Context, time.Time) (int64, error) {
		called = true
		return 0, nil
	}}
	New(db, Config{AlertsDays: 90}).prune(context.Background())
	assert.False(t, called)
}

// ── prune with DB errors (doesn't crash, continues) ──────────────────────────

func TestPrune_Coverage_DBErrors(t *testing.T) {
	t.Parallel()
	captureCalled := false
	db := &mockRetDB2{
		pruneAlertsFn: func(context.Context, time.Time) (int64, error) { return 0, fmt.Errorf("alerts prune failed") },
		pruneHealthFn: func(context.Context, time.Time) (int64, error) { return 0, fmt.Errorf("health prune failed") },
		pruneCaptureFn: func(context.Context, time.Time) (int64, error) {
			captureCalled = true
			return 0, nil
		},
	}
	New(db, Config{AlertsDays: 90, HealthHistoryDays: 30, CaptureDays: 7}).prune(context.Background())
	assert.True(t, captureCalled, "a failing prune must not stop the others")
}

// ── Start and Stop ───────────────────────────────────────────────────────────

func TestScheduler_Coverage_StartStop(t *testing.T) {
	t.Parallel()
	s := New(&mockRetDB2{}, Config{AlertsDays: 90})
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Stop()
}

func (m *mockRetDB2) PruneHealthScoreHistory(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneHealthFn != nil {
		return m.pruneHealthFn(ctx, olderThan)
	}
	return 0, nil
}

func (m *mockRetDB2) PruneCaptureSessions(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneCaptureFn != nil {
		return m.pruneCaptureFn(ctx, olderThan)
	}
	return 0, nil
}

func (m *mockRetDB2) GetMetricBaselines(ctx context.Context, since time.Time) ([]database.MetricBaseline, error) {
	return nil, nil
}

func (m *mockRetDB2) UpdateDeviceStatuses(ctx context.Context, ids []int64, statuses []string) ([]int64, error) {
	return nil, nil
}

func (m *mockRetDB2) RecordInterfaceSamples(ctx context.Context, samples []models.InterfaceSample) error {
	return nil
}

func (m *mockRetDB2) ReplaceDeviceNeighbors(ctx context.Context, deviceID int64, neighbors []models.LLDPNeighbor) error {
	return nil
}
