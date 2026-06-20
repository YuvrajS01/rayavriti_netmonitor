package handlers

import (
	"context"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type mockDB struct {
	connectFn                   func(ctx context.Context) error
	closeFn                     func() error
	pingFn                      func(ctx context.Context) error
	runMigrationsFn             func(ctx context.Context) error
	getDevicesFn                func(ctx context.Context) ([]models.Device, error)
	getDevicesFilteredFn        func(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error)
	getDeviceFn                 func(ctx context.Context, id int64) (*models.Device, error)
	createDeviceFn              func(ctx context.Context, d *models.Device) (*models.Device, error)
	updateDeviceFn              func(ctx context.Context, id int64, d *models.Device) (*models.Device, error)
	deleteDeviceFn              func(ctx context.Context, id int64) error
	updateDeviceStatusFn        func(ctx context.Context, id int64, status string) error
	getEnabledDevicesFn         func(ctx context.Context) ([]models.Device, error)
	getDevicesByStatusFn        func(ctx context.Context, status string) ([]models.Device, error)
	getSensorsFn                func(ctx context.Context, deviceID *int64) ([]models.Sensor, error)
	getSensorFn                 func(ctx context.Context, id int64) (*models.Sensor, error)
	createSensorFn              func(ctx context.Context, s *models.Sensor) (*models.Sensor, error)
	updateSensorFn              func(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error)
	deleteSensorFn              func(ctx context.Context, id int64) error
	getSensorsByDeviceIDFn      func(ctx context.Context, deviceID int64) ([]models.Sensor, error)
	recordMetricFn              func(ctx context.Context, m *models.Metric) error
	getLatestMetricsFn          func(ctx context.Context) ([]models.Metric, error)
	getDeviceMetricsFn          func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error)
	getMetricsSummaryFn         func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error)
	getMetricsForReportFn       func(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error)
	getReportTimeseriesFn       func(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error)
	getReportDeviceBreakdownFn  func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error)
	queryMetricsFn              func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error)
	exportMetricsFn             func(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error)
	getMetricsInWindowFn        func(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error)
	getHealthScoresFn           func(ctx context.Context) ([]models.DeviceHealthScoreRow, error)
	getHealthScoreHistoryFn     func(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error)
	getNetworkHealthHistoryFn   func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error)
	insertHealthScoreHistoryFn  func(ctx context.Context, entries []models.HealthHistoryEntry) error
	getMetricsSinceFn           func(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error)
	getStatusFlapsFn            func(ctx context.Context, deviceID int64, since time.Time) (int, error)
	getPortChangesFn            func(ctx context.Context, deviceID int64, since time.Time) (int, error)
	getAlertsByRuleSinceFn      func(ctx context.Context, ruleID int64, since time.Time) (int, error)
	getAlertsFn                 func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error)
	getAlertFn                  func(ctx context.Context, id int64) (*models.Alert, error)
	createAlertFn               func(ctx context.Context, a *models.Alert) (*models.Alert, error)
	updateAlertStatusFn         func(ctx context.Context, id int64, status, by string) error
	deleteAlertFn               func(ctx context.Context, id int64) error
	getAlertCountsFn            func(ctx context.Context) (models.AlertCounts, error)
	findActiveAlertFn           func(ctx context.Context, deviceID int64, message string) (*models.Alert, error)
	getAlertsForReportFn        func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error)
	getAlertRulesFn             func(ctx context.Context) ([]models.AlertRule, error)
	getAlertRuleFn              func(ctx context.Context, id int64) (*models.AlertRule, error)
	createAlertRuleFn           func(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error)
	updateAlertRuleFn           func(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error)
	deleteAlertRuleFn           func(ctx context.Context, id int64) error
	toggleAlertRuleFn           func(ctx context.Context, id int64, enabled bool) error
	getNotificationChannelsFn   func(ctx context.Context) ([]models.NotificationChannel, error)
	getNotificationChannelFn    func(ctx context.Context, id int64) (*models.NotificationChannel, error)
	createNotificationChannelFn func(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error)
	updateNotificationChannelFn func(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error)
	deleteNotificationChannelFn func(ctx context.Context, id int64) error
	recordAlertHistoryFn        func(ctx context.Context, h *models.AlertHistory) error
	getAlertHistoryFn           func(ctx context.Context, alertID int64) ([]models.AlertHistory, error)
	getAlertRuleStateFn         func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error)
	upsertAlertRuleStateFn      func(ctx context.Context, s *models.AlertRuleState) error
	getUserByUsernameFn         func(ctx context.Context, username string) (*models.User, error)
	getUserByIDFn               func(ctx context.Context, id int64) (*models.User, error)
	createUserFn                func(ctx context.Context, u *models.User) (*models.User, error)
	updateUserFn                func(ctx context.Context, id int64, u *models.User) (*models.User, error)
	deleteUserFn                func(ctx context.Context, id int64) error
	getAPIKeyFn                 func(ctx context.Context, keyHash string) (*models.APIKey, error)
	getAPIKeyByIDFn             func(ctx context.Context, id int64) (*models.APIKey, error)
	createAPIKeyFn              func(ctx context.Context, k *models.APIKey) (*models.APIKey, error)
	getAPIKeysByUserFn          func(ctx context.Context, userID int64) ([]models.APIKey, error)
	deleteAPIKeyFn              func(ctx context.Context, id int64) error
	recordFlowsFn               func(ctx context.Context, flows []models.Flow) error
	getFlowsFn                  func(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error)
	getTopTalkersFn             func(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error)
	getProtocolStatsFn          func(ctx context.Context, from, to time.Time) (map[string]int64, error)
	getFlowTimeseriesFn         func(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error)
	getFlowStatsFn              func(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error)
	createCaptureSessionFn      func(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error)
	getCaptureSessionFn         func(ctx context.Context, id int64) (*models.CaptureSession, error)
	getCaptureSessionsFn        func(ctx context.Context) ([]models.CaptureSession, error)
	stopCaptureSessionFn        func(ctx context.Context, id int64, stats models.CaptureSessionStats) error
	insertCapturePacketFn       func(ctx context.Context, sessionID int64, p *models.CapturePacket) error
	getCapturePacketsFn         func(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error)
	upsertPortScanResultsFn     func(ctx context.Context, deviceID int64, results []models.PortScanResult) error
	getPortScanResultsFn        func(ctx context.Context, deviceID int64) ([]models.PortScanResult, error)
	getDashboardsFn             func(ctx context.Context, userID int64) ([]models.Dashboard, error)
	getDashboardFn              func(ctx context.Context, id int64) (*models.Dashboard, error)
	saveDashboardFn             func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error)
	deleteDashboardFn           func(ctx context.Context, id int64) error
	pruneMetricsFn              func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneFlowsFn                func(ctx context.Context, olderThan time.Time) (int64, error)
	pruneAlertsFn               func(ctx context.Context, olderThan time.Time) (int64, error)
	getDashboardStatsFn         func(ctx context.Context) (map[string]any, error)
	getRefreshTokenFn           func(ctx context.Context, tokenHash string) (*database.RefreshToken, error)
}

func (m *mockDB) Connect(ctx context.Context) error {
	if m.connectFn != nil {
		return m.connectFn(ctx)
	}
	return nil
}

func (m *mockDB) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockDB) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockDB) RunMigrations(ctx context.Context) error {
	if m.runMigrationsFn != nil {
		return m.runMigrationsFn(ctx)
	}
	return nil
}

func (m *mockDB) GetDevices(ctx context.Context) ([]models.Device, error) {
	if m.getDevicesFn != nil {
		return m.getDevicesFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	if m.getDevicesFilteredFn != nil {
		return m.getDevicesFilteredFn(ctx, f)
	}
	return nil, 0, nil
}

func (m *mockDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, d)
	}
	return nil, nil
}

func (m *mockDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	if m.updateDeviceFn != nil {
		return m.updateDeviceFn(ctx, id, d)
	}
	return nil, nil
}

func (m *mockDB) DeleteDevice(ctx context.Context, id int64) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id)
	}
	return nil
}

func (m *mockDB) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	if m.updateDeviceStatusFn != nil {
		return m.updateDeviceStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockDB) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	if m.getEnabledDevicesFn != nil {
		return m.getEnabledDevicesFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) {
	if m.getDevicesByStatusFn != nil {
		return m.getDevicesByStatusFn(ctx, status)
	}
	return nil, nil
}

func (m *mockDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	if m.getSensorsFn != nil {
		return m.getSensorsFn(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) {
	if m.getSensorFn != nil {
		return m.getSensorFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	if m.createSensorFn != nil {
		return m.createSensorFn(ctx, s)
	}
	return nil, nil
}

func (m *mockDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	if m.updateSensorFn != nil {
		return m.updateSensorFn(ctx, id, s)
	}
	return nil, nil
}

func (m *mockDB) DeleteSensor(ctx context.Context, id int64) error {
	if m.deleteSensorFn != nil {
		return m.deleteSensorFn(ctx, id)
	}
	return nil
}

func (m *mockDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	if m.getSensorsByDeviceIDFn != nil {
		return m.getSensorsByDeviceIDFn(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockDB) RecordMetric(ctx context.Context, mt *models.Metric) error {
	if m.recordMetricFn != nil {
		return m.recordMetricFn(ctx, mt)
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
	if m.getDeviceMetricsFn != nil {
		return m.getDeviceMetricsFn(ctx, deviceID, from, to, limit)
	}
	return nil, nil
}

func (m *mockDB) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	if m.getMetricsSummaryFn != nil {
		return m.getMetricsSummaryFn(ctx, from, to, deviceID)
	}
	return nil, nil
}

func (m *mockDB) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	if m.getMetricsForReportFn != nil {
		return m.getMetricsForReportFn(ctx, from, to, deviceID, interval)
	}
	return nil, nil
}

func (m *mockDB) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	if m.getReportTimeseriesFn != nil {
		return m.getReportTimeseriesFn(ctx, from, to, bucketMinutes, deviceID)
	}
	return nil, nil
}

func (m *mockDB) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	if m.getReportDeviceBreakdownFn != nil {
		return m.getReportDeviceBreakdownFn(ctx, from, to, deviceID)
	}
	return nil, nil
}

func (m *mockDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	if m.queryMetricsFn != nil {
		return m.queryMetricsFn(ctx, q)
	}
	return nil, nil
}

func (m *mockDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	if m.exportMetricsFn != nil {
		return m.exportMetricsFn(ctx, from, to, deviceID, limit)
	}
	return nil, nil
}

func (m *mockDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	if m.getMetricsInWindowFn != nil {
		return m.getMetricsInWindowFn(ctx, deviceID, field, from, to)
	}
	return nil, nil
}

func (m *mockDB) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	return nil
}
func (m *mockDB) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	if m.getHealthScoresFn != nil {
		return m.getHealthScoresFn(ctx)
	}
	return nil, nil
}
func (m *mockDB) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	if m.getHealthScoreHistoryFn != nil {
		return m.getHealthScoreHistoryFn(ctx, deviceID, hours)
	}
	return nil, nil
}
func (m *mockDB) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
	if m.getNetworkHealthHistoryFn != nil {
		return m.getNetworkHealthHistoryFn(ctx, hours)
	}
	return nil, nil
}
func (m *mockDB) InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error {
	return nil
}
func (m *mockDB) GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockDB) GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockDB) GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockDB) GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *mockDB) RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error {
	return nil
}
func (m *mockDB) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	return nil, nil
}

func (m *mockDB) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	if m.getAlertsFn != nil {
		return m.getAlertsFn(ctx, status, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error) {
	if m.getAlertFn != nil {
		return m.getAlertFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	if m.createAlertFn != nil {
		return m.createAlertFn(ctx, a)
	}
	return nil, nil
}

func (m *mockDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	if m.updateAlertStatusFn != nil {
		return m.updateAlertStatusFn(ctx, id, status, by)
	}
	return nil
}

func (m *mockDB) DeleteAlert(ctx context.Context, id int64) error {
	if m.deleteAlertFn != nil {
		return m.deleteAlertFn(ctx, id)
	}
	return nil
}

func (m *mockDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	if m.getAlertCountsFn != nil {
		return m.getAlertCountsFn(ctx)
	}
	return models.AlertCounts{}, nil
}

func (m *mockDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	if m.findActiveAlertFn != nil {
		return m.findActiveAlertFn(ctx, deviceID, message)
	}
	return nil, nil
}

func (m *mockDB) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	return nil, nil
}

func (m *mockDB) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	return nil, nil
}

func (m *mockDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	if m.getAlertsForReportFn != nil {
		return m.getAlertsForReportFn(ctx, from, to, deviceID)
	}
	return nil, nil
}

func (m *mockDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	if m.getAlertRulesFn != nil {
		return m.getAlertRulesFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	if m.getAlertRuleFn != nil {
		return m.getAlertRuleFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	if m.createAlertRuleFn != nil {
		return m.createAlertRuleFn(ctx, r)
	}
	return nil, nil
}

func (m *mockDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	if m.updateAlertRuleFn != nil {
		return m.updateAlertRuleFn(ctx, id, r)
	}
	return nil, nil
}

func (m *mockDB) DeleteAlertRule(ctx context.Context, id int64) error {
	if m.deleteAlertRuleFn != nil {
		return m.deleteAlertRuleFn(ctx, id)
	}
	return nil
}

func (m *mockDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error {
	if m.toggleAlertRuleFn != nil {
		return m.toggleAlertRuleFn(ctx, id, enabled)
	}
	return nil
}

func (m *mockDB) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	if m.getNotificationChannelsFn != nil {
		return m.getNotificationChannelsFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	if m.getNotificationChannelFn != nil {
		return m.getNotificationChannelFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	if m.createNotificationChannelFn != nil {
		return m.createNotificationChannelFn(ctx, ch)
	}
	return nil, nil
}

func (m *mockDB) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	if m.updateNotificationChannelFn != nil {
		return m.updateNotificationChannelFn(ctx, id, ch)
	}
	return nil, nil
}

func (m *mockDB) DeleteNotificationChannel(ctx context.Context, id int64) error {
	if m.deleteNotificationChannelFn != nil {
		return m.deleteNotificationChannelFn(ctx, id)
	}
	return nil
}

func (m *mockDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	if m.recordAlertHistoryFn != nil {
		return m.recordAlertHistoryFn(ctx, h)
	}
	return nil
}

func (m *mockDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	if m.getAlertHistoryFn != nil {
		return m.getAlertHistoryFn(ctx, alertID)
	}
	return nil, nil
}

func (m *mockDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	if m.getAlertRuleStateFn != nil {
		return m.getAlertRuleStateFn(ctx, ruleID, deviceID)
	}
	return nil, nil
}

func (m *mockDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	if m.upsertAlertRuleStateFn != nil {
		return m.upsertAlertRuleStateFn(ctx, s)
	}
	return nil
}

func (m *mockDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.getUserByUsernameFn != nil {
		return m.getUserByUsernameFn(ctx, username)
	}
	return nil, nil
}

func (m *mockDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, u)
	}
	return nil, nil
}

func (m *mockDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, id, u)
	}
	return nil, nil
}

func (m *mockDB) DeleteUser(ctx context.Context, id int64) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	if m.getAPIKeyFn != nil {
		return m.getAPIKeyFn(ctx, keyHash)
	}
	return nil, nil
}

func (m *mockDB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	if m.getAPIKeyByIDFn != nil {
		return m.getAPIKeyByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	if m.createAPIKeyFn != nil {
		return m.createAPIKeyFn(ctx, k)
	}
	return nil, nil
}

func (m *mockDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	if m.getAPIKeysByUserFn != nil {
		return m.getAPIKeysByUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockDB) DeleteAPIKey(ctx context.Context, id int64) error {
	if m.deleteAPIKeyFn != nil {
		return m.deleteAPIKeyFn(ctx, id)
	}
	return nil
}

func (m *mockDB) RecordFlows(ctx context.Context, flows []models.Flow) error {
	if m.recordFlowsFn != nil {
		return m.recordFlowsFn(ctx, flows)
	}
	return nil
}

func (m *mockDB) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	if m.getFlowsFn != nil {
		return m.getFlowsFn(ctx, from, to, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockDB) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	if m.getTopTalkersFn != nil {
		return m.getTopTalkersFn(ctx, from, to, n)
	}
	return nil, nil
}

func (m *mockDB) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	if m.getProtocolStatsFn != nil {
		return m.getProtocolStatsFn(ctx, from, to)
	}
	return nil, nil
}

func (m *mockDB) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	if m.getFlowTimeseriesFn != nil {
		return m.getFlowTimeseriesFn(ctx, from, to, interval)
	}
	return nil, nil
}

func (m *mockDB) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	if m.getFlowStatsFn != nil {
		return m.getFlowStatsFn(ctx, from, to)
	}
	return models.FlowSummaryStats{}, nil
}

func (m *mockDB) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	if m.createCaptureSessionFn != nil {
		return m.createCaptureSessionFn(ctx, cs)
	}
	return nil, nil
}

func (m *mockDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	if m.getCaptureSessionFn != nil {
		return m.getCaptureSessionFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	if m.getCaptureSessionsFn != nil {
		return m.getCaptureSessionsFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	if m.stopCaptureSessionFn != nil {
		return m.stopCaptureSessionFn(ctx, id, stats)
	}
	return nil
}

func (m *mockDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	if m.insertCapturePacketFn != nil {
		return m.insertCapturePacketFn(ctx, sessionID, p)
	}
	return nil
}

func (m *mockDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	if m.getCapturePacketsFn != nil {
		return m.getCapturePacketsFn(ctx, sessionID, limit, offset)
	}
	return nil, nil
}

func (m *mockDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
	if m.upsertPortScanResultsFn != nil {
		return m.upsertPortScanResultsFn(ctx, deviceID, results)
	}
	return nil
}

func (m *mockDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	if m.getPortScanResultsFn != nil {
		return m.getPortScanResultsFn(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	if m.getDashboardsFn != nil {
		return m.getDashboardsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	if m.getDashboardFn != nil {
		return m.getDashboardFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	if m.saveDashboardFn != nil {
		return m.saveDashboardFn(ctx, d)
	}
	return nil, nil
}

func (m *mockDB) DeleteDashboard(ctx context.Context, id int64) error {
	if m.deleteDashboardFn != nil {
		return m.deleteDashboardFn(ctx, id)
	}
	return nil
}

func (m *mockDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneMetricsFn != nil {
		return m.pruneMetricsFn(ctx, olderThan)
	}
	return 0, nil
}

func (m *mockDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneFlowsFn != nil {
		return m.pruneFlowsFn(ctx, olderThan)
	}
	return 0, nil
}

func (m *mockDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	if m.pruneAlertsFn != nil {
		return m.pruneAlertsFn(ctx, olderThan)
	}
	return 0, nil
}

func (m *mockDB) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	if m.getDashboardStatsFn != nil {
		return m.getDashboardStatsFn(ctx)
	}
	return nil, nil
}

func (m *mockDB) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *mockDB) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	if m.getRefreshTokenFn != nil {
		return m.getRefreshTokenFn(ctx, tokenHash)
	}
	return nil, nil
}
func (m *mockDB) DeleteRefreshToken(ctx context.Context, tokenHash string) error    { return nil }
func (m *mockDB) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *mockDB) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)    { return 0, nil }
