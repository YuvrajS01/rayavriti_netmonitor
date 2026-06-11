package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockQueryDB struct {
	getRecentHTTPRequestsFn  func(ctx context.Context, limit int) ([]HTTPRequest, error)
	getRecentDBQueriesFn     func(ctx context.Context, limit int) ([]DBQuery, error)
	getRecentCollectorRunsFn func(ctx context.Context, limit int) ([]CollectorRun, error)
	getRecentSystemMetricsFn func(ctx context.Context, limit int) ([]SystemMetrics, error)
	getRecentAuditLogFn      func(ctx context.Context, limit int) ([]AuditLogEntry, error)
}

func (m *mockQueryDB) GetRecentHTTPRequests(ctx context.Context, limit int) ([]HTTPRequest, error) {
	if m.getRecentHTTPRequestsFn != nil {
		return m.getRecentHTTPRequestsFn(ctx, limit)
	}
	return nil, nil
}
func (m *mockQueryDB) GetRecentDBQueries(ctx context.Context, limit int) ([]DBQuery, error) {
	if m.getRecentDBQueriesFn != nil {
		return m.getRecentDBQueriesFn(ctx, limit)
	}
	return nil, nil
}
func (m *mockQueryDB) GetRecentCollectorRuns(ctx context.Context, limit int) ([]CollectorRun, error) {
	if m.getRecentCollectorRunsFn != nil {
		return m.getRecentCollectorRunsFn(ctx, limit)
	}
	return nil, nil
}
func (m *mockQueryDB) GetRecentSystemMetrics(ctx context.Context, limit int) ([]SystemMetrics, error) {
	if m.getRecentSystemMetricsFn != nil {
		return m.getRecentSystemMetricsFn(ctx, limit)
	}
	return nil, nil
}
func (m *mockQueryDB) GetRecentAuditLog(ctx context.Context, limit int) ([]AuditLogEntry, error) {
	if m.getRecentAuditLogFn != nil {
		return m.getRecentAuditLogFn(ctx, limit)
	}
	return nil, nil
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
		Error   any            `json:"error"`
	}
	err := json.NewDecoder(rec.Body).Decode(&envelope)
	require.NoError(t, err)
	return envelope.Data
}

func TestNewMonitoringHandler(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{}
	handler := NewMonitoringHandler(db)
	require.NotNil(t, handler)
}

func TestSystemLogs_DefaultComponent(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Contains(t, result, "http_requests")
	assert.Contains(t, result, "db_queries")
	assert.Contains(t, result, "collector_runs")
	assert.Contains(t, result, "audit_log")
}

func TestSystemLogs_HTTPComponent(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentHTTPRequestsFn: func(ctx context.Context, limit int) ([]HTTPRequest, error) {
			return []HTTPRequest{{RequestID: "r1", Method: "GET", Path: "/api/v1/devices", StatusCode: 200}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?component=http", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, "http", result["component"])
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemLogs_DBComponent(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentDBQueriesFn: func(ctx context.Context, limit int) ([]DBQuery, error) {
			return []DBQuery{{RequestID: "q1", Operation: "SELECT", Table: "devices"}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?component=db", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, "db", result["component"])
}

func TestSystemLogs_CollectorComponent(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentCollectorRunsFn: func(ctx context.Context, limit int) ([]CollectorRun, error) {
			return []CollectorRun{{DeviceID: 1, Protocol: "ping", Status: "up"}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?component=collector", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, "collector", result["component"])
}

func TestSystemLogs_AuditComponent(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentAuditLogFn: func(ctx context.Context, limit int) ([]AuditLogEntry, error) {
			return []AuditLogEntry{{EventType: "auth.login", Actor: "admin"}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?component=audit", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, "audit", result["component"])
}

func TestSystemLogsStats(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs/stats", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogsStats(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.NotNil(t, result["message"])
}

func TestSystemMonitoring_NoMetrics(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentSystemMetricsFn: func(ctx context.Context, limit int) ([]SystemMetrics, error) {
			return nil, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoring(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.NotNil(t, result["message"])
}

func TestSystemMonitoring_WithMetrics(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentSystemMetricsFn: func(ctx context.Context, limit int) ([]SystemMetrics, error) {
			return []SystemMetrics{{UptimeSeconds: 3600, GoroutineCount: 10, NumCPU: 8}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoring(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(3600), result["uptime_seconds"])
}

func TestSystemMonitoringHistory(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentSystemMetricsFn: func(ctx context.Context, limit int) ([]SystemMetrics, error) {
			return []SystemMetrics{{UptimeSeconds: 100}, {UptimeSeconds: 200}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/history?hours=1", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringHistory(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["hours"])
	assert.Equal(t, float64(2), result["count"])
}

func TestSystemMonitoringRequests(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentHTTPRequestsFn: func(ctx context.Context, limit int) ([]HTTPRequest, error) {
			return []HTTPRequest{
				{RequestID: "r1", Method: "GET", Path: "/api/v1/devices", StatusCode: 200, DurationMs: 10},
				{RequestID: "r2", Method: "POST", Path: "/api/v1/devices", StatusCode: 201, DurationMs: 50},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/requests?path=/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringRequests(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(2), result["count"])
}

func TestSystemMonitoringRequests_FilterByStatus(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentHTTPRequestsFn: func(ctx context.Context, limit int) ([]HTTPRequest, error) {
			return []HTTPRequest{{StatusCode: 200}, {StatusCode: 500}}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/requests?status_code=500", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringRequests(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemMonitoringQueries_SlowOnly(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentDBQueriesFn: func(ctx context.Context, limit int) ([]DBQuery, error) {
			return []DBQuery{
				{MethodName: "GetDevices", IsSlow: false},
				{MethodName: "GetMetrics", IsSlow: true},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/queries?slow_only=true", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringQueries(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemMonitoringQueries_FilterByMethod(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentDBQueriesFn: func(ctx context.Context, limit int) ([]DBQuery, error) {
			return []DBQuery{
				{MethodName: "GetDevices"},
				{MethodName: "GetMetrics"},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/queries?method=GetDevices", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringQueries(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemAuditLog(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentAuditLogFn: func(ctx context.Context, limit int) ([]AuditLogEntry, error) {
			return []AuditLogEntry{
				{EventType: "auth.login", Actor: "admin"},
				{EventType: "auth.logout", Actor: "user1"},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/audit-log", nil)
	rec := httptest.NewRecorder()
	handler.SystemAuditLog(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(2), result["count"])
}

func TestSystemAuditLog_FilterByEventType(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentAuditLogFn: func(ctx context.Context, limit int) ([]AuditLogEntry, error) {
			return []AuditLogEntry{
				{EventType: "auth.login", Actor: "admin"},
				{EventType: "auth.logout", Actor: "admin"},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/audit-log?event_type=auth.login", nil)
	rec := httptest.NewRecorder()
	handler.SystemAuditLog(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemAuditLog_FilterByActor(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentAuditLogFn: func(ctx context.Context, limit int) ([]AuditLogEntry, error) {
			return []AuditLogEntry{
				{EventType: "auth.login", Actor: "admin"},
				{EventType: "auth.login", Actor: "user1"},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/audit-log?actor=admin", nil)
	rec := httptest.NewRecorder()
	handler.SystemAuditLog(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestSystemCollectorsStats(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentCollectorRunsFn: func(ctx context.Context, limit int) ([]CollectorRun, error) {
			return []CollectorRun{
				{DeviceID: 1, DeviceName: "r1", Protocol: "ping", Status: "up", DurationMs: 10},
				{DeviceID: 1, DeviceName: "r1", Protocol: "ping", Status: "down", DurationMs: 5000},
				{DeviceID: 2, DeviceName: "s1", Protocol: "snmp", Status: "up", DurationMs: 20},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/collectors/stats", nil)
	rec := httptest.NewRecorder()
	handler.SystemCollectorsStats(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(2), result["count"])
}

func TestSystemCollectorsStats_FilterByDevice(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentCollectorRunsFn: func(ctx context.Context, limit int) ([]CollectorRun, error) {
			return []CollectorRun{
				{DeviceID: 1, DeviceName: "r1", Protocol: "ping", Status: "up", DurationMs: 10},
				{DeviceID: 2, DeviceName: "s1", Protocol: "snmp", Status: "up", DurationMs: 20},
			}, nil
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/collectors/stats?device_id=1", nil)
	rec := httptest.NewRecorder()
	handler.SystemCollectorsStats(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decodeResponse(t, rec)
	assert.Equal(t, float64(1), result["count"])
}

func TestParseIntQuery_Default(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, 100, parseIntQuery(req, "limit", 100))
}

func TestParseIntQuery_Valid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/?limit=50", nil)
	assert.Equal(t, 50, parseIntQuery(req, "limit", 100))
}

func TestParseIntQuery_Invalid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/?limit=abc", nil)
	assert.Equal(t, 100, parseIntQuery(req, "limit", 100))
}

func TestSystemLogs_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentHTTPRequestsFn: func(ctx context.Context, limit int) ([]HTTPRequest, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?component=http", nil)
	rec := httptest.NewRecorder()
	handler.SystemLogs(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemMonitoring_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentSystemMetricsFn: func(ctx context.Context, limit int) ([]SystemMetrics, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoring(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemMonitoringRequests_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentHTTPRequestsFn: func(ctx context.Context, limit int) ([]HTTPRequest, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/requests", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringRequests(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemMonitoringQueries_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentDBQueriesFn: func(ctx context.Context, limit int) ([]DBQuery, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/queries", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringQueries(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemAuditLog_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentAuditLogFn: func(ctx context.Context, limit int) ([]AuditLogEntry, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/audit-log", nil)
	rec := httptest.NewRecorder()
	handler.SystemAuditLog(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemCollectorsStats_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentCollectorRunsFn: func(ctx context.Context, limit int) ([]CollectorRun, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/collectors/stats", nil)
	rec := httptest.NewRecorder()
	handler.SystemCollectorsStats(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSystemMonitoringHistory_DBError(t *testing.T) {
	t.Parallel()
	db := &mockQueryDB{
		getRecentSystemMetricsFn: func(ctx context.Context, limit int) ([]SystemMetrics, error) {
			return nil, assert.AnError
		},
	}
	handler := NewMonitoringHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring/history", nil)
	rec := httptest.NewRecorder()
	handler.SystemMonitoringHistory(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
