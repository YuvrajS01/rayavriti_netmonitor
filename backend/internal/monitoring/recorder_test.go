package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecorder(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{}
	recorder := NewRecorder(db)
	require.NotNil(t, recorder)
}

func TestRecorder_RecordHTTP(t *testing.T) {
	t.Parallel()

	var recorded *HTTPRequest
	db := &mockMonitoringDB{
		recordHTTPFn: func(ctx context.Context, r *HTTPRequest) error {
			recorded = r
			return nil
		},
	}
	recorder := NewRecorder(db)

	req := &HTTPRequest{
		RequestID:  "req-123",
		Method:     "GET",
		Path:       "/api/v1/devices",
		StatusCode: 200,
		DurationMs: 45.5,
		Timestamp:  time.Now(),
	}

	err := recorder.RecordHTTP(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "req-123", recorded.RequestID)
	assert.Equal(t, "GET", recorded.Method)
	assert.Equal(t, "/api/v1/devices", recorded.Path)
	assert.Equal(t, 200, recorded.StatusCode)
}

func TestRecorder_RecordDB(t *testing.T) {
	t.Parallel()

	var recorded *DBQuery
	db := &mockMonitoringDB{
		recordDBQueryFn: func(ctx context.Context, q *DBQuery) error {
			recorded = q
			return nil
		},
	}
	recorder := NewRecorder(db)

	query := &DBQuery{
		RequestID:  "req-456",
		Operation:  "SELECT",
		Table:      "devices",
		MethodName: "GetDevices",
		DurationMs: 12.3,
		Timestamp:  time.Now(),
	}

	err := recorder.RecordDB(context.Background(), query)
	require.NoError(t, err)
	assert.Equal(t, "req-456", recorded.RequestID)
	assert.Equal(t, "SELECT", recorded.Operation)
	assert.Equal(t, "devices", recorded.Table)
}

func TestRecorder_RecordCollector(t *testing.T) {
	t.Parallel()

	var recorded *CollectorRun
	db := &mockMonitoringDB{
		recordCollectorFn: func(ctx context.Context, c *CollectorRun) error {
			recorded = c
			return nil
		},
	}
	recorder := NewRecorder(db)

	run := &CollectorRun{
		DeviceID:   1,
		DeviceName: "router-1",
		Protocol:   "ping",
		Status:     "up",
		DurationMs: 100.0,
		Timestamp:  time.Now(),
	}

	err := recorder.RecordCollector(context.Background(), run)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recorded.DeviceID)
	assert.Equal(t, "router-1", recorded.DeviceName)
	assert.Equal(t, "up", recorded.Status)
}

func TestRecorder_RecordSystem(t *testing.T) {
	t.Parallel()

	var recorded *SystemMetrics
	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			recorded = m
			return nil
		},
	}
	recorder := NewRecorder(db)

	metrics := &SystemMetrics{
		UptimeSeconds:  3600,
		GoroutineCount: 10,
		NumCPU:         8,
		Timestamp:      time.Now(),
	}

	err := recorder.RecordSystem(context.Background(), metrics)
	require.NoError(t, err)
	assert.Equal(t, int64(3600), recorded.UptimeSeconds)
	assert.Equal(t, 10, recorded.GoroutineCount)
}

func TestRecorder_RecordAudit(t *testing.T) {
	t.Parallel()

	var recorded *AuditLogEntry
	db := &mockMonitoringDB{
		recordAuditFn: func(ctx context.Context, e *AuditLogEntry) error {
			recorded = e
			return nil
		},
	}
	recorder := NewRecorder(db)

	entry := &AuditLogEntry{
		EventType: "auth.login",
		Severity:  "info",
		Actor:     "admin",
		Timestamp: time.Now(),
	}

	err := recorder.RecordAudit(context.Background(), entry)
	require.NoError(t, err)
	assert.Equal(t, "auth.login", recorded.EventType)
	assert.Equal(t, "admin", recorded.Actor)
}

func TestRecorder_RecordAlert(t *testing.T) {
	t.Parallel()

	var recorded *AlertActivity
	db := &mockMonitoringDB{
		recordAlertActivityFn: func(ctx context.Context, a *AlertActivity) error {
			recorded = a
			return nil
		},
	}
	recorder := NewRecorder(db)

	activity := &AlertActivity{
		RuleID:    1,
		RuleName:  "high-latency",
		DeviceID:  5,
		Action:    "fired",
		Timestamp: time.Now(),
	}

	err := recorder.RecordAlert(context.Background(), activity)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recorded.RuleID)
	assert.Equal(t, "fired", recorded.Action)
}

func TestRecorder_RecordHTTP_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordHTTPFn: func(ctx context.Context, r *HTTPRequest) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordHTTP(context.Background(), &HTTPRequest{})
	assert.Error(t, err)
}

func TestRecorder_RecordDB_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordDBQueryFn: func(ctx context.Context, q *DBQuery) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordDB(context.Background(), &DBQuery{})
	assert.Error(t, err)
}

func TestRecorder_RecordCollector_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordCollectorFn: func(ctx context.Context, c *CollectorRun) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordCollector(context.Background(), &CollectorRun{})
	assert.Error(t, err)
}

func TestRecorder_RecordSystem_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordSystem(context.Background(), &SystemMetrics{})
	assert.Error(t, err)
}

func TestRecorder_RecordAudit_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordAuditFn: func(ctx context.Context, e *AuditLogEntry) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordAudit(context.Background(), &AuditLogEntry{})
	assert.Error(t, err)
}

func TestRecorder_RecordAlert_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordAlertActivityFn: func(ctx context.Context, a *AlertActivity) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)

	err := recorder.RecordAlert(context.Background(), &AlertActivity{})
	assert.Error(t, err)
}

func TestRecorder_MultipleRecords(t *testing.T) {
	t.Parallel()

	httpCount := 0
	dbCount := 0
	db := &mockMonitoringDB{
		recordHTTPFn: func(ctx context.Context, r *HTTPRequest) error {
			httpCount++
			return nil
		},
		recordDBQueryFn: func(ctx context.Context, q *DBQuery) error {
			dbCount++
			return nil
		},
	}
	recorder := NewRecorder(db)

	for i := 0; i < 5; i++ {
		require.NoError(t, recorder.RecordHTTP(context.Background(), &HTTPRequest{}))
		require.NoError(t, recorder.RecordDB(context.Background(), &DBQuery{}))
	}

	assert.Equal(t, 5, httpCount)
	assert.Equal(t, 5, dbCount)
}
