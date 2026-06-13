package logging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/stretchr/testify/assert"
)

// ── AuditLogger: uncovered convenience methods ────────────────────────────────

func TestAuditLogger_LogLogout(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogLogout(context.Background(), 1, "admin")
}

func TestAuditLogger_LogTokenRefresh(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogTokenRefresh(context.Background(), 1, "admin")
}

func TestAuditLogger_LogAPIKeyUsed(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogAPIKeyUsed(context.Background(), "my-key", 42, "10.0.0.1", "/api/v1/devices")
}

func TestAuditLogger_LogAPIKeyInvalid(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogAPIKeyInvalid(context.Background(), "10.0.0.1", "/api/v1/devices")
}

func TestAuditLogger_LogRateLimit(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogRateLimit(context.Background(), "user:admin", "jwt", "10.0.0.1", 100, 60)
}

func TestAuditLogger_LogUnauthorized(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogUnauthorized(context.Background(), "10.0.0.1", "/api/v1/admin", "missing token")
}

func TestAuditLogger_LogInvalidToken(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogInvalidToken(context.Background(), "10.0.0.1", "token expired")
}

func TestAuditLogger_LogCaptureEvent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogCaptureEvent(context.Background(), "started", "user:admin", "10.0.0.1", map[string]any{
		"interface": "eth0",
		"filter":    "port 80",
	})
}

func TestAuditLogger_LogRetentionPurge(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogRetentionPurge(context.Background(), map[string]any{
		"metrics_purged": 1000,
		"flows_purged":   500,
		"alerts_purged":  50,
		"retention_days": 30,
	})
}

func TestAuditLogger_LogAction_OkFalse(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogAction(context.Background(), 1, "create", "device", 0, false)
}

func TestAuditLogger_LogAction_SystemActor(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogAction(context.Background(), 0, "purge", "metrics", 0, true)
}

func TestAuditLogger_LogEvent_AllFields(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogEvent(context.Background(), AuditEvent{
		EventType:    "test.event",
		Severity:     "warn",
		Actor:        "user:admin",
		ActorIP:      "10.0.0.1",
		ResourceType: "device",
		ResourceID:   "42",
		Description:  "test event with all fields",
		Details:      map[string]any{"key1": "value1", "key2": 42},
	})
}

func TestAuditLogger_LogEvent_ErrorSeverity(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogEvent(context.Background(), AuditEvent{
		EventType:   "test.error",
		Severity:    "error",
		Description: "test error event",
	})
}

func TestAuditLogger_LogLogin_WithDetails(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogLogin(context.Background(), 1, "admin", "127.0.0.1", "Mozilla/5.0", true, map[string]any{
		"method": "password",
	})
}

func TestAuditLogger_LogConfigChange_WithDetails(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogConfigChange(context.Background(), "updated", "device", 1, "user:admin", "127.0.0.1", map[string]any{
		"field": "name",
		"old":   "Server1",
		"new":   "WebServer1",
	})
}

func TestAuditLogger_LogEvent_MinimalFields(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	audit := NewAuditLogger(logger)
	audit.LogEvent(context.Background(), AuditEvent{
		EventType:   "minimal.event",
		Severity:    "info",
		Description: "minimal event",
	})
}

// ── CollectorLogger: LogSNMPDetail ───────────────────────────────────────────

func TestCollectorLogger_LogSNMPDetail(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogSNMPDetail(context.Background(), 1,
		[]string{".1.3.6.1.2.1.1.3.0", ".1.3.6.1.2.1.1.5.0"},
		[]map[string]any{
			{"oid": ".1.3.6.1.2.1.1.3.0", "value": "12345"},
			{"oid": ".1.3.6.1.2.1.1.5.0", "value": "localhost"},
		},
		15.5,
	)
}

func TestCollectorLogger_LogSNMPDetail_EmptyOIDs(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogSNMPDetail(context.Background(), 1,
		[]string{},
		[]map[string]any{},
		0,
	)
}

func TestCollectorLogger_LogSNMPDetail_NilVarbinds(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogSNMPDetail(context.Background(), 1,
		nil,
		nil,
		5.0,
	)
}

// ── CollectorLogger: LogResult with SensorID and Extra ───────────────────────

func TestCollectorLogger_LogResult_WithSensorAndExtra(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogResult(context.Background(), CollectorEvent{
		DeviceID:       1,
		DeviceName:     "Server-1",
		Host:           "192.168.1.1",
		Protocol:       "snmp",
		SensorID:       5,
		Status:         "up",
		PreviousStatus: "down",
		StatusChanged:  true,
		ResponseTimeMs: 50,
		Value:          75.5,
		Message:        "CPU at 75.5%",
		MetricID:       42,
		DurationMs:     55,
		Extra:          map[string]any{"cpu_percent": 75.5, "memory_percent": 60.2},
	})
}

func TestCollectorLogger_LogFailure_WithSensorAndExtra(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogFailure(context.Background(), CollectorEvent{
		DeviceID:            1,
		DeviceName:          "Server-1",
		Host:                "192.168.1.1",
		Protocol:            "snmp",
		SensorID:            3,
		Status:              "down",
		PreviousStatus:      "up",
		StatusChanged:       true,
		Error:               assert.AnError,
		DurationMs:          5000,
		ConsecutiveFailures: 5,
		Extra:               map[string]any{"error_code": 500},
	})
}

func TestCollectorLogger_LogResult_NoSensor(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collector := NewCollectorLogger(logger)
	collector.LogResult(context.Background(), CollectorEvent{
		DeviceID:       2,
		DeviceName:     "Switch-1",
		Host:           "192.168.1.2",
		Protocol:       "ping",
		Status:         "up",
		ResponseTimeMs: 10,
		DurationMs:     15,
	})
}

// ── Logger: TraceCtx ────────────────────────────────────────────────────────

func TestLogger_TraceCtx(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		App:     config.AppConfig{Version: "1.0.0", AppEnv: "development"},
		Logging: config.LoggingConfig{Level: "trace", Format: "text"},
	}
	logger := New(cfg)
	ctx := WithRequestID(context.Background(), "req-trace-1")
	logger.TraceCtx(ctx, "trace context message", "key", "value")
}

func TestLogger_DebugCtx(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	ctx := WithRequestID(context.Background(), "req-debug-1")
	logger.DebugCtx(ctx, "debug context message", "key", "value")
}

func TestLogger_WarnCtx(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	ctx := WithRequestID(context.Background(), "req-warn-1")
	logger.WarnCtx(ctx, "warn context message", "key", "value")
}

func TestLogger_ErrorCtx(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	ctx := WithRequestID(context.Background(), "req-error-1")
	logger.ErrorCtx(ctx, "error context message", "key", "value")
}

// ── DBLogger: LogQuery edge cases ────────────────────────────────────────────

func TestDBLogger_LogQuery_EmptyQuery(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	db.LogQuery(context.Background(), "SELECT", "devices", "GetDevices",
		"", nil, 5*time.Millisecond, 0, 0, nil)
}

func TestDBLogger_LogQuery_SlowQuery(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 10) // 10ms threshold
	db.LogQuery(context.Background(), "SELECT", "metrics", "GetMetrics",
		"SELECT * FROM metrics WHERE device_id = ?", []any{1}, 50*time.Millisecond, 100, 0, nil)
}

func TestDBLogger_LogQuery_ErrorWithParams(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	db.LogQuery(context.Background(), "INSERT", "alerts", "CreateAlert",
		"INSERT INTO alerts (message) VALUES (?)", []any{"test alert"}, 5*time.Millisecond, 0, 0, assert.AnError)
}

func TestDBLogger_LogQuery_ErrorNoParams(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	db.LogQuery(context.Background(), "DELETE", "alerts", "DeleteAlert",
		"DELETE FROM alerts WHERE id = ?", nil, 5*time.Millisecond, 0, 0, assert.AnError)
}

func TestDBLogger_LogQuery_SlowWithParams(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 1) // 1ms threshold
	db.LogQuery(context.Background(), "SELECT", "devices", "GetDevices",
		"SELECT * FROM devices WHERE status = ?", []any{"up"}, 100*time.Millisecond, 5, 0, nil)
}

func TestDBLogger_LogQuery_WithTraceID(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	ctx := WithRequestID(context.Background(), "req-123")
	ctx = WithTraceID(ctx, "trace-456")
	db.LogQuery(ctx, "SELECT", "devices", "GetDevices",
		"SELECT * FROM devices", nil, 5*time.Millisecond, 3, 0, nil)
}

func TestDBLogger_LogQuery_SlowWithTraceID(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 1) // 1ms threshold
	ctx := WithRequestID(context.Background(), "req-789")
	ctx = WithTraceID(ctx, "trace-012")
	db.LogQuery(ctx, "SELECT", "devices", "GetDevices",
		"SELECT * FROM devices WHERE id = ?", []any{1}, 50*time.Millisecond, 1, 0, nil)
}

func TestDBLogger_LogQuery_NormalQuery(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 10000) // high threshold
	ctx := WithRequestID(context.Background(), "req-norm")
	db.LogQuery(ctx, "SELECT", "devices", "GetDevices",
		"SELECT * FROM devices", nil, 5*time.Millisecond, 3, 0, nil)
}

// ── DBLogger: LogTransaction edge cases ──────────────────────────────────────

func TestDBLogger_LogTransaction_CommitSuccess(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	ctx := WithRequestID(context.Background(), "req-tx-1")
	db.LogTransaction(ctx, "COMMIT", 10*time.Millisecond, nil)
}

func TestDBLogger_LogTransaction_RollbackSuccess(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	ctx := WithRequestID(context.Background(), "req-tx-2")
	db.LogTransaction(ctx, "ROLLBACK", 5*time.Millisecond, nil)
}

func TestDBLogger_LogTransaction_RollbackError(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	ctx := WithRequestID(context.Background(), "req-tx-3")
	db.LogTransaction(ctx, "ROLLBACK", 5*time.Millisecond, assert.AnError)
}

// ── RequestLogger: WebSocket upgrade requests ────────────────────────────────

func TestRequestLogger_WebSocketUpgrade(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	middleware := RequestLogger(logger, 10000)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Sec-WebSocket-Accept", "test")
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSwitchingProtocols, w.Code)
}

// ── parseLevel edge cases ────────────────────────────────────────────────────

func TestParseLevel_AllLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"trace", int(LevelTrace)},
		{"debug", -4},
		{"info", 0},
		{"warn", 4},
		{"error", 8},
		{"TRACE", 0},
		{"DEBUG", 0},
		{"INFO", 0},
	}
	for _, tt := range tests {
		level := parseLevel(tt.input)
		assert.Equal(t, tt.expected, int(level), "parseLevel(%q)", tt.input)
	}
}

// ── Logger.With additional tests ──────────────────────────────────────────────

func TestLogger_With_Component(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	logger := New(cfg)
	child := logger.With("database")
	assert.Equal(t, "database", child.component)
}

// ── DBLogger.LogPoolStats edge cases ─────────────────────────────────────────

func TestDBLogger_LogPoolStats_Zeros(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	db.LogPoolStats(0, 0, 0, 0, 0)
}

func TestDBLogger_LogPoolStats_HighValues(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	db := NewDBLogger(logger, 100)
	db.LogPoolStats(100, 50, 25, 1000, 15.5)
}
