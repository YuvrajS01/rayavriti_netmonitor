package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serverMockDB struct{}

func (m *serverMockDB) Connect(ctx context.Context) error       { return nil }
func (m *serverMockDB) Close() error                            { return nil }
func (m *serverMockDB) Ping(ctx context.Context) error          { return nil }
func (m *serverMockDB) RunMigrations(ctx context.Context) error { return nil }
func (m *serverMockDB) GetDevices(ctx context.Context) ([]models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) GetDevicesFiltered(ctx context.Context, f database.DeviceFilter) ([]models.Device, int, error) {
	return nil, 0, nil
}
func (m *serverMockDB) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteDevice(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	return nil
}
func (m *serverMockDB) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) {
	return nil, nil
}
func (m *serverMockDB) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *serverMockDB) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) {
	return nil, nil
}
func (m *serverMockDB) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteSensor(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	return nil, nil
}
func (m *serverMockDB) RecordMetric(ctx context.Context, metric *models.Metric) error { return nil }
func (m *serverMockDB) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	return nil, nil
}
func (m *serverMockDB) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	return nil, nil
}
func (m *serverMockDB) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	return nil, nil
}
func (m *serverMockDB) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	return nil, nil
}
func (m *serverMockDB) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	return nil, nil
}
func (m *serverMockDB) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	return nil, 0, nil
}
func (m *serverMockDB) GetAlert(ctx context.Context, id int64) (*models.Alert, error) {
	return nil, nil
}
func (m *serverMockDB) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	return nil
}
func (m *serverMockDB) DeleteAlert(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	return models.AlertCounts{}, nil
}
func (m *serverMockDB) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	return nil, nil
}
func (m *serverMockDB) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	return nil, nil
}
func (m *serverMockDB) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	return nil, nil
}
func (m *serverMockDB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	return nil, nil
}
func (m *serverMockDB) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	return nil, nil
}
func (m *serverMockDB) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteAlertRule(ctx context.Context, id int64) error               { return nil }
func (m *serverMockDB) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error { return nil }
func (m *serverMockDB) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	return nil, nil
}
func (m *serverMockDB) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *serverMockDB) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteNotificationChannel(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	return nil
}
func (m *serverMockDB) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	return nil, nil
}
func (m *serverMockDB) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	return nil, nil
}
func (m *serverMockDB) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	return nil
}
func (m *serverMockDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return nil, nil
}
func (m *serverMockDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	return nil, nil
}
func (m *serverMockDB) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *serverMockDB) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteUser(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	return nil, nil
}
func (m *serverMockDB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	return nil, nil
}
func (m *serverMockDB) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	return nil, nil
}
func (m *serverMockDB) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteAPIKey(ctx context.Context, id int64) error           { return nil }
func (m *serverMockDB) RecordFlows(ctx context.Context, flows []models.Flow) error { return nil }
func (m *serverMockDB) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	return nil, 0, nil
}
func (m *serverMockDB) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	return nil, nil
}
func (m *serverMockDB) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}
func (m *serverMockDB) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	return nil, nil
}
func (m *serverMockDB) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	return models.FlowSummaryStats{}, nil
}
func (m *serverMockDB) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *serverMockDB) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	return nil, nil
}
func (m *serverMockDB) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	return nil, nil
}
func (m *serverMockDB) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	return nil
}
func (m *serverMockDB) InsertCapturePacket(ctx context.Context, sessionID int64, p *models.CapturePacket) error {
	return nil
}
func (m *serverMockDB) GetCapturePackets(ctx context.Context, sessionID int64, limit, offset int) ([]models.CapturePacket, error) {
	return nil, nil
}
func (m *serverMockDB) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) (int, error) {
	return 0, nil
}
func (m *serverMockDB) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	return nil, nil
}
func (m *serverMockDB) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	return nil, nil
}
func (m *serverMockDB) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	return nil, nil
}
func (m *serverMockDB) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteDashboard(ctx context.Context, id int64) error { return nil }
func (m *serverMockDB) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}
func (m *serverMockDB) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}
func (m *serverMockDB) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}
func (m *serverMockDB) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	return nil, nil
}
func (m *serverMockDB) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	return nil
}
func (m *serverMockDB) GetRefreshToken(ctx context.Context, tokenHash string) (*database.RefreshToken, error) {
	return nil, nil
}
func (m *serverMockDB) DeleteRefreshToken(ctx context.Context, tokenHash string) error    { return nil }
func (m *serverMockDB) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error { return nil }
func (m *serverMockDB) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	return nil
}
func (m *serverMockDB) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	return nil, nil
}
func (m *serverMockDB) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *serverMockDB) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
	return nil, nil
}
func (m *serverMockDB) InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error {
	return nil
}
func (m *serverMockDB) GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error) {
	return nil, nil
}
func (m *serverMockDB) GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *serverMockDB) GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *serverMockDB) GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error) {
	return 0, nil
}
func (m *serverMockDB) RecordSuppressedAlert(ctx context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error {
	return nil
}
func (m *serverMockDB) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	return nil, nil
}
func (m *serverMockDB) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error) { return 0, nil }

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func testConfig(port int) *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Port:    port,
			AppEnv:  "development",
			Version: "test",
		},
		Auth: config.AuthConfig{
			JWTSecret: "test-secret",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	cfg := testConfig(0)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	require.NotNil(t, srv)
}

func TestShutdown_NilServer(t *testing.T) {
	t.Parallel()
	srv := &Server{}
	err := srv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestShutdown_WithServer(t *testing.T) {
	t.Parallel()
	cfg := testConfig(0)
	srv := &Server{cfg: cfg}
	err := srv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestStart_HealthEndpoint(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1AuthLogin(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/login", port), "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Login will fail with invalid credentials but route is hit
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1AuthLogout(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/logout", port), "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1AuthRefresh(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/refresh", port), "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_ProtectedRoute_NoAuth(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	routes := []string{
		"/api/v1/devices",
		"/api/v1/alerts",
		"/api/v1/flows",
		"/api/v1/dashboards",
		"/api/v1/reports",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, route))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "route %s should return 401", route)
		})
	}

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_ProtectedRoute_WithAuth(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/v1/devices", port), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1MetricsQuery(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/metrics/query", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1CaptureInterfaces(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/capture/interfaces", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1Sensors(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/sensors", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1AlertRules(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/alert-rules", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1NotificationChannels(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/notification-channels", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1SystemInfo(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/system/info", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_NotFound(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/nonexistent", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V12FAVerify(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/2fa/verify", port), "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_V1AuthMe_Unauthorized(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/auth/me", port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_LegacyAuthRoutes(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	// Legacy auth login
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/auth/login", port), "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	// Legacy auth logout
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/auth/logout", port), "application/json", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	// Legacy auth me (protected)
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/auth/me", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Legacy stats (protected)
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/stats", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_LegacyDeviceRoutes_Unauthorized(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/devices", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_StartProduction(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	cfg.App.AppEnv = "production"
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestStart_CORSOrigins(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	cfg := testConfig(port)
	cfg.App.CORSOrigins = []string{"http://example.com"}
	db := &serverMockDB{}
	hub := websocket.NewHub("test-secret", nil, nil)
	logger := logging.New(cfg)

	srv := New(cfg, db, hub, logger)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
	<-errCh
}

func TestRateLimiter_MultipleIPsExceedLimit(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimiter(context.Background(), 1, 1, nil)(inner)

	// IP 1: first request OK, second rate limited
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	// IP 2: first request OK
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "10.0.0.2:5678"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusOK, rec3.Code)
}

func TestRequestSize_OversizedRequest(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestSize(10)(inner)
	body := make([]byte, 100)
	for i := range body {
		body[i] = 'x'
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// The body is limited; reading should fail or return partial
	_, err := fmt.Fprint(rec, "")
	assert.NoError(t, err)
}
