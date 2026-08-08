package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/collectors"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDB struct {
	getEnabledDevicesFn        func(ctx context.Context) ([]models.Device, error)
	getDeviceFn                func(ctx context.Context, id int64) (*models.Device, error)
	recordMetricFn             func(ctx context.Context, m *models.Metric) error
	recordMetricsBatchFn       func(ctx context.Context, metrics []*models.Metric) error
	getLatestMetricsFn         func(ctx context.Context) ([]models.Metric, error)
	getLatestMetricForDeviceFn func(ctx context.Context, deviceID int64) (*models.Metric, error)
	updateDeviceStatusFn       func(ctx context.Context, id int64, status string) error
}

func (m *mockDB) Connect(ctx context.Context) error                       { return nil }
func (m *mockDB) Close() error                                            { return nil }
func (m *mockDB) Ping(ctx context.Context) error                          { return nil }
func (m *mockDB) RunMigrations(ctx context.Context) error                 { return nil }
func (m *mockDB) GetDevices(ctx context.Context) ([]models.Device, error) { return nil, nil }
func (m *mockDB) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	return nil, 0, nil
}
func (m *mockDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	if m.getDeviceFn != nil {
		return m.getDeviceFn(ctx, id)
	}
	return nil, nil
}
func (m *mockDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *mockDB) DeleteDevice(ctx context.Context, id int64) error { return nil }
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
	return nil, nil
}
func (m *mockDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) { return nil, nil }
func (m *mockDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) DeleteSensor(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *mockDB) RecordMetric(ctx context.Context, metric *models.Metric) error {
	if m.recordMetricFn != nil {
		return m.recordMetricFn(ctx, metric)
	}
	return nil
}
func (m *mockDB) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	if m.recordMetricsBatchFn != nil {
		return m.recordMetricsBatchFn(ctx, metrics)
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
func (m *mockDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *mockDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	return nil, nil
}
func (m *mockDB) GetAlerts(ctx context.Context, status string, limit, offset int, _ *database.ScopeFilter) ([]models.Alert, int, error) {
	return nil, 0, nil
}
func (m *mockDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error) { return nil, nil }
func (m *mockDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	return nil, nil
}
func (m *mockDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	return nil
}
func (m *mockDB) DeleteAlert(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	return models.AlertCounts{}, nil
}
func (m *mockDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	return nil, nil
}
func (m *mockDB) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	return nil, nil
}
func (m *mockDB) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	if m.getLatestMetricForDeviceFn != nil {
		return m.getLatestMetricForDeviceFn(ctx, deviceID)
	}
	return nil, nil
}
func (m *mockDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	return nil, nil
}
func (m *mockDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) { return nil, nil }
func (m *mockDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) DeleteAlertRule(ctx context.Context, id int64) error               { return nil }
func (m *mockDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error { return nil }
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
func (m *mockDB) DeleteNotificationChannel(ctx context.Context, id int64) error        { return nil }
func (m *mockDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error { return nil }
func (m *mockDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	return nil, nil
}
func (m *mockDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	return nil, nil
}
func (m *mockDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	return nil
}
func (m *mockDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *mockDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) { return nil, nil }
func (m *mockDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *mockDB) DeleteUser(ctx context.Context, id int64) error { return nil }
func (m *mockDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockDB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	return nil, nil
}
func (m *mockDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	return nil, nil
}
func (m *mockDB) DeleteAPIKey(ctx context.Context, id int64) error           { return nil }
func (m *mockDB) RecordFlows(ctx context.Context, flows []models.Flow) error { return nil }
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
func (m *mockDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *mockDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	return nil, nil
}
func (m *mockDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	return nil
}
func (m *mockDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *mockDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *mockDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) (int, error) {
	return 0, nil
}
func (m *mockDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	return nil, nil
}
func (m *mockDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	return nil, nil
}
func (m *mockDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	return nil, nil
}
func (m *mockDB) DeleteDashboard(ctx context.Context, id int64) error                  { return nil }
func (m *mockDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) { return 0, nil }
func (m *mockDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error)   { return 0, nil }
func (m *mockDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error)  { return 0, nil }
func (m *mockDB) GetDashboardStats(ctx context.Context) (map[string]any, error)        { return nil, nil }
func (m *mockDB) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *mockDB) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	return nil, nil
}
func (m *mockDB) DeleteRefreshToken(ctx context.Context, tokenHash string) error    { return nil }
func (m *mockDB) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *mockDB) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error)    { return 0, nil }
func (m *mockDB) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	return nil
}
func (m *mockDB) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	return nil, nil
}
func (m *mockDB) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *mockDB) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
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

func newTestHub() *websocket.Hub {
	return websocket.NewHub("test-secret", nil, nil)
}

func TestScheduler_New(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	s := New(db, reg, newTestHub(), nil, 30)
	require.NotNil(t, s)
	assert.Equal(t, 0, s.JobCount())
}

func TestScheduler_StartStop_NoDevices(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	s.Stop()
}

func TestScheduler_StartStop_WithDevices(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "r1", Protocol: "ping", Interval: 10},
			}, nil
		},
	}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	assert.GreaterOrEqual(t, s.dispatcher.Count(), 1)
	s.Stop()
}

func TestScheduler_StopIdempotent(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
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
			switch callCount {
			case 1:
				return []models.Device{
					{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
				}, nil
			case 2:
				return []models.Device{
					{ID: 1, Name: "d1", Protocol: "ping", Interval: 10},
					{ID: 2, Name: "d2", Protocol: "ping", Interval: 10},
				}, nil
			default:
				// Third call: device 1 removed, only device 2 remains
				return []models.Device{
					{ID: 2, Name: "d2", Protocol: "ping", Interval: 10},
				}, nil
			}
		},
	}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, s.JobCount())
	assert.Equal(t, 1, s.dispatcher.Count())

	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, s.JobCount())
	assert.Equal(t, 2, s.dispatcher.Count())

	s.reconcile(ctx)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, s.JobCount())
	assert.Equal(t, 1, s.dispatcher.Count())
	assert.ElementsMatch(t, []int64{2}, s.dispatcher.DeviceIDs())

	s.Stop()
}

func TestScheduler_Reconcile_DBError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getEnabledDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	s.reconcile(ctx)
	s.Stop()
}

func TestScheduler_ScheduleDevice(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "ping", Interval: 10}
	s.scheduleDevice(device)
	assert.GreaterOrEqual(t, s.dispatcher.Count(), 1)
}

func TestScheduler_UnschedDevice(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	device := models.Device{ID: 1, Name: "d1", Protocol: "ping", Interval: 10}
	s.scheduleDevice(device)
	assert.GreaterOrEqual(t, s.dispatcher.Count(), 1)
	s.unscheduleDevice(1)
	assert.Equal(t, 0, s.dispatcher.Count())
}

func TestCollectAndReturnResult_UnknownProtocol(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	s := New(db, collectors.NewRegistry(), newTestHub(), nil, 30)
	job := PollJob{Device: models.Device{ID: 1, Name: "d1", Protocol: "nonexistent"}}
	result := s.collectAndReturnResult(context.Background(), job)
	assert.Error(t, result.Error)
}

func TestCollectAndReturnResult_Success(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	reg.Register(&mockCollector{
		name:   "test_proto",
		result: &collectors.Result{Status: "up", ResponseTime: f64p(5.0)},
	})
	s := New(db, reg, newTestHub(), nil, 30)
	job := PollJob{Device: models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}}
	result := s.collectAndReturnResult(context.Background(), job)
	assert.NoError(t, result.Error)
	assert.Equal(t, "up", result.Status)
}

func TestCollectAndReturnResult_NilResult(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	reg.Register(&mockCollector{
		name:   "test_proto",
		result: nil,
	})
	s := New(db, reg, newTestHub(), nil, 30)
	job := PollJob{Device: models.Device{ID: 1, Name: "d1", Protocol: "test_proto"}}
	result := s.collectAndReturnResult(context.Background(), job)
	assert.NoError(t, result.Error)
	assert.Equal(t, "down", result.Status)
}

func TestCollectAndReturnResult_CollectorError(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	reg := collectors.NewRegistry()
	reg.Register(&mockCollector{
		name: "err_proto",
		err:  fmt.Errorf("collect failed"),
	})
	s := New(db, reg, newTestHub(), nil, 30)
	job := PollJob{Device: models.Device{ID: 1, Name: "d1", Protocol: "err_proto"}}
	result := s.collectAndReturnResult(context.Background(), job)
	assert.Error(t, result.Error)
}

func TestWorkerPool_EnqueueAndExecute(t *testing.T) {
	t.Parallel()
	var executed atomic.Int64
	wp := NewWorkerPool(DefaultWorkerPoolConfig(), func(ctx context.Context, job PollJob) PollResult {
		executed.Add(1)
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	wp.Enqueue(PollJob{Device: models.Device{ID: 1}, Priority: 0})
	wp.Enqueue(PollJob{Device: models.Device{ID: 2}, Priority: 1})
	wp.Enqueue(PollJob{Device: models.Device{ID: 3}, Priority: 2})
	time.Sleep(200 * time.Millisecond)
	wp.Stop()
	assert.Equal(t, int64(3), executed.Load())
}

func TestWorkerPool_QueueDrainsOnStop(t *testing.T) {
	t.Parallel()
	var executed atomic.Int64
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 2}, func(ctx context.Context, job PollJob) PollResult {
		time.Sleep(50 * time.Millisecond)
		executed.Add(1)
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	for i := 0; i < 10; i++ {
		wp.Enqueue(PollJob{Device: models.Device{ID: int64(i)}, Priority: 1})
	}
	time.Sleep(50 * time.Millisecond)
	wp.Stop()
	assert.Greater(t, executed.Load(), int64(0))
}

func TestWorkerPool_PriorityOrder(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var order []int
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		mu.Lock()
		order = append(order, job.Priority)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Enqueue(PollJob{Device: models.Device{ID: 3}, Priority: 2})
	wp.Enqueue(PollJob{Device: models.Device{ID: 1}, Priority: 0})
	wp.Enqueue(PollJob{Device: models.Device{ID: 2}, Priority: 1})
	wp.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	wp.Stop()
	mu.Lock()
	defer mu.Unlock()
	if len(order) >= 3 {
		assert.Equal(t, 0, order[0])
	}
}

func TestWorkerPool_Metrics(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(DefaultWorkerPoolConfig(), func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	wp.Enqueue(PollJob{Device: models.Device{ID: 1}, Priority: 0})
	time.Sleep(100 * time.Millisecond)
	m := wp.Metrics()
	assert.GreaterOrEqual(t, m.Completed, int64(1))
	wp.Stop()
}

func TestWorkerPool_EnqueueDroppedWhenFull(t *testing.T) {
	t.Parallel()
	cfg := DefaultWorkerPoolConfig()
	cfg.CriticalQueueSize = 1
	wp := NewWorkerPool(cfg, func(ctx context.Context, job PollJob) PollResult {
		time.Sleep(100 * time.Millisecond)
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	wp.Enqueue(PollJob{Device: models.Device{ID: 1}, Priority: 0})
	wp.Enqueue(PollJob{Device: models.Device{ID: 2}, Priority: 0})
	wp.Enqueue(PollJob{Device: models.Device{ID: 3}, Priority: 0})
	time.Sleep(200 * time.Millisecond)
	wp.Stop()
	m := wp.Metrics()
	assert.GreaterOrEqual(t, m.Completed, int64(1))
}

func TestPollDispatcher_UpsertAndDispatch(t *testing.T) {
	t.Parallel()
	executed := make(chan PollJob, 10)
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 2}, func(ctx context.Context, job PollJob) PollResult {
		executed <- job
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1, Name: "d1"}, 0, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	select {
	case job := <-executed:
		assert.Equal(t, int64(1), job.Device.ID)
	default:
		t.Fatal("expected job to be dispatched")
	}
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_Remove(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1}, 0, time.Hour)
	assert.Equal(t, 1, d.Count())
	d.Remove(1)
	assert.Equal(t, 0, d.Count())
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_DeviceIDs(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 10}, 0, time.Hour)
	d.Upsert(models.Device{ID: 20}, 0, time.Hour)
	assert.ElementsMatch(t, []int64{10, 20}, d.DeviceIDs())
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_PauseResume(t *testing.T) {
	t.Parallel()
	executed := make(chan PollJob, 10)
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		executed <- job
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1, Name: "d1"}, 0, 50*time.Millisecond)
	d.Pause(1)
	time.Sleep(100 * time.Millisecond)
	select {
	case <-executed:
		t.Fatal("should not execute paused device")
	default:
	}
	d.Resume(1)
	time.Sleep(100 * time.Millisecond)
	select {
	case <-executed:
	default:
		t.Fatal("should execute after resume")
	}
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_Backoff(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "down", Error: fmt.Errorf("fail"), FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1, Name: "d1"}, 0, 50*time.Millisecond)
	d.RecordFailure(1)
	e := d.deviceMap[1]
	require.NotNil(t, e)
	assert.Equal(t, 1, e.Failures)
	d.RecordFailure(1)
	assert.Equal(t, 2, e.Failures)
	d.RecordFailure(1)
	assert.Equal(t, 3, e.Failures)
	assert.Equal(t, StateUnreachable, e.State)
	d.Stop()
	wp.Stop()
}

func TestPollDispatcher_SuccessResets(t *testing.T) {
	t.Parallel()
	wp := NewWorkerPool(WorkerPoolConfig{WorkerCount: 1}, func(ctx context.Context, job PollJob) PollResult {
		return PollResult{Device: job.Device, Status: "up", FinishedAt: time.Now()}
	})
	d := NewPollDispatcher(wp, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	d.Start(ctx)
	d.Upsert(models.Device{ID: 1}, 0, time.Hour)
	d.RecordFailure(1)
	d.RecordFailure(1)
	d.RecordFailure(1)
	e := d.deviceMap[1]
	require.NotNil(t, e)
	assert.Equal(t, StateUnreachable, e.State)
	d.RecordSuccess(1)
	assert.Equal(t, StateHealthy, e.State)
	assert.Equal(t, 0, e.Failures)
	d.Stop()
	wp.Stop()
}

func TestDeviceStateTracker_RecordSuccess(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordSuccess(1, 10*time.Millisecond)
	h := dst.GetState(1)
	require.NotNil(t, h)
	assert.Equal(t, "up", h.CurrentStatus)
	assert.Equal(t, 0, h.ConsecutiveFails)
}

func TestDeviceStateTracker_RecordFailure(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordFailure(1, errors.New("timeout"))
	h := dst.GetState(1)
	require.NotNil(t, h)
	assert.Equal(t, 1, h.ConsecutiveFails)
	dst.RecordFailure(1, errors.New("timeout"))
	dst.RecordFailure(1, errors.New("timeout"))
	h = dst.GetState(1)
	assert.Equal(t, 3, h.ConsecutiveFails)
	assert.Equal(t, StateUnreachable, h.State)
	assert.Equal(t, 4, h.BackoffMultiplier)
}

func TestDeviceStateTracker_UnreachableCount(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(2, errors.New("err"))
	dst.RecordFailure(2, errors.New("err"))
	dst.RecordFailure(2, errors.New("err"))
	assert.Equal(t, 2, dst.GetUnreachableCount())
}

func TestDeviceStateTracker_PauseResume(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordSuccess(1, 5*time.Millisecond)
	dst.Pause(1)
	assert.Equal(t, 1, dst.GetPausedCount())
	dst.Resume(1)
	assert.Equal(t, 0, dst.GetPausedCount())
	assert.Equal(t, StateHealthy, dst.GetState(1).State)
}

func TestDeviceStateTracker_Remove(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordSuccess(1, 5*time.Millisecond)
	assert.Equal(t, 1, dst.Count())
	dst.Remove(1)
	assert.Equal(t, 0, dst.Count())
}

func TestDeviceStateTracker_RollingAverage(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	for i := 0; i < 10; i++ {
		dst.RecordSuccess(1, time.Duration(10+i)*time.Millisecond)
	}
	h := dst.GetState(1)
	require.NotNil(t, h)
	assert.Greater(t, h.AvgResponseTime, time.Duration(0))
}

func TestDeviceStateTracker_GetUnreachableDevices(t *testing.T) {
	t.Parallel()
	dst := NewDeviceStateTracker()
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(1, errors.New("err"))
	dst.RecordFailure(2, errors.New("err"))
	ids := dst.GetUnreachableDevices()
	assert.Contains(t, ids, int64(1))
	assert.Len(t, ids, 1)
}

func TestDependencyTree_SetParent(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	parent, exists := dt.GetParent(2)
	assert.True(t, exists)
	assert.Equal(t, int64(1), parent)
	children := dt.GetChildren(1)
	assert.Contains(t, children, int64(2))
}

func TestDependencyTree_RemoveDevice(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	dt.RemoveDevice(2)
	_, exists := dt.GetParent(2)
	assert.False(t, exists)
	assert.Empty(t, dt.GetChildren(1))
}

func TestDependencyTree_GetDescendants(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	dt.SetParent(3, 2)
	dt.SetParent(4, 1)
	desc := dt.GetDescendants(1)
	assert.Contains(t, desc, int64(2))
	assert.Contains(t, desc, int64(3))
	assert.Contains(t, desc, int64(4))
}

func TestDependencyTree_GetAncestors(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	dt.SetParent(3, 2)
	anc := dt.GetAncestors(3)
	assert.Contains(t, anc, int64(1))
	assert.Contains(t, anc, int64(2))
}

func TestDependencyTree_IsDependency(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	assert.True(t, dt.IsDependency(2))
	assert.False(t, dt.IsDependency(1))
}

func TestDependencyTree_IsParent(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(2, 1)
	assert.True(t, dt.IsParent(1))
	assert.False(t, dt.IsParent(2))
}

func TestDependencyTree_ReassignParent(t *testing.T) {
	t.Parallel()
	dt := NewDependencyTree()
	dt.SetParent(3, 1)
	dt.SetParent(3, 2)
	_, exists := dt.GetParent(3)
	assert.True(t, exists)
	assert.Empty(t, dt.GetChildren(1))
	assert.Contains(t, dt.GetChildren(2), int64(3))
}

func TestResultPipeline_SubmitAndFlush(t *testing.T) {
	t.Parallel()
	db := &mockDB{}
	p := NewResultPipeline(ResultPipelineConfig{
		DB:        db,
		BatchSize: 10,
		FlushMs:   100,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	for i := 0; i < 5; i++ {
		p.Submit(PollResult{
			Device:     models.Device{ID: int64(i), Name: fmt.Sprintf("d%d", i)},
			Status:     "up",
			FinishedAt: time.Now(),
		})
	}
	time.Sleep(150 * time.Millisecond)
	p.Stop()
}

func TestResultPipeline_BatchSizeTrigger(t *testing.T) {
	t.Parallel()
	var batchCount atomic.Int64
	db := &mockDB{
		recordMetricsBatchFn: func(ctx context.Context, metrics []*models.Metric) error {
			batchCount.Add(1)
			return nil
		},
	}
	p := NewResultPipeline(ResultPipelineConfig{
		DB:        db,
		BatchSize: 5,
		FlushMs:   5000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	for i := 0; i < 5; i++ {
		p.Submit(PollResult{
			Device:     models.Device{ID: int64(i)},
			Status:     "up",
			FinishedAt: time.Now(),
		})
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int64(1), batchCount.Load())
	p.Stop()
}

func TestResultPipeline_FlushOnStop(t *testing.T) {
	t.Parallel()
	var flushed bool
	db := &mockDB{
		recordMetricsBatchFn: func(ctx context.Context, metrics []*models.Metric) error {
			flushed = true
			return nil
		},
	}
	p := NewResultPipeline(ResultPipelineConfig{
		DB:        db,
		BatchSize: 100,
		FlushMs:   5000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Submit(PollResult{
		Device:     models.Device{ID: 1, Name: "d1"},
		Status:     "up",
		FinishedAt: time.Now(),
	})
	time.Sleep(50 * time.Millisecond)
	p.Stop()
	assert.True(t, flushed)
}

func f64p(v float64) *float64 { return &v }

type mockCollector struct {
	name   string
	result *collectors.Result
	err    error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context, d *models.Device) (*collectors.Result, error) {
	return m.result, m.err
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultSchedulerConfig()
	assert.Greater(t, cfg.WorkerCount, 0)
	assert.Greater(t, cfg.MaxWorkerCount, cfg.WorkerCount)
	assert.Equal(t, 256, cfg.CriticalQueueSize)
	assert.Equal(t, 1024, cfg.NormalQueueSize)
	assert.Equal(t, 512, cfg.LowQueueSize)
}

func TestWorkerPoolConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultWorkerPoolConfig()
	assert.Equal(t, 32, cfg.WorkerCount)
	assert.Equal(t, 64, cfg.MaxWorkerCount)
}

func TestResultPipeline_FullCollectResultAndDeviceStatusUpdate(t *testing.T) {
	t.Parallel()
	var recordedMetrics []*models.Metric
	var updatedStatusDeviceID int64
	var updatedStatus string
	db := &mockDB{
		recordMetricsBatchFn: func(ctx context.Context, metrics []*models.Metric) error {
			recordedMetrics = append(recordedMetrics, metrics...)
			return nil
		},
		updateDeviceStatusFn: func(ctx context.Context, id int64, status string) error {
			updatedStatusDeviceID = id
			updatedStatus = status
			return nil
		},
	}
	p := NewResultPipeline(ResultPipelineConfig{
		DB:        db,
		BatchSize: 100,
		FlushMs:   5000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	cpu := 45.5
	mem := 60.2
	loss := 0.0
	bw := 1000.0
	resp := 12.3
	details := map[string]any{"cpu_temp": 42}

	p.Submit(PollResult{
		Device: models.Device{ID: 10, Name: "switch1", Protocol: "snmp", Status: "down"},
		Status: "up",
		CollectResult: &collectors.Result{
			Status:       "up",
			ResponseTime: &resp,
			PacketLoss:   &loss,
			CPUUsage:     &cpu,
			MemoryUsage:  &mem,
			Bandwidth:    &bw,
			Details:      details,
		},
		FinishedAt: time.Now(),
	})
	time.Sleep(50 * time.Millisecond)
	p.Stop()

	assert.Len(t, recordedMetrics, 1)
	if len(recordedMetrics) == 1 {
		m := recordedMetrics[0]
		assert.Equal(t, int64(10), m.DeviceID)
		assert.Equal(t, "up", m.Status)
		assert.Equal(t, &resp, m.ResponseTime)
		assert.Equal(t, &loss, m.PacketLoss)
		assert.Equal(t, &cpu, m.CPUUsage)
		assert.Equal(t, &mem, m.MemoryUsage)
		assert.Equal(t, &bw, m.Bandwidth)
		assert.Equal(t, details, m.Details)
	}

	assert.Equal(t, int64(10), updatedStatusDeviceID)
	assert.Equal(t, "up", updatedStatus)
}
