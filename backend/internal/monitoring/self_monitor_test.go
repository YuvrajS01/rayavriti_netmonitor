package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMonitoringDB struct {
	recordHTTPFn        func(ctx context.Context, r *HTTPRequest) error
	recordDBQueryFn     func(ctx context.Context, q *DBQuery) error
	recordCollectorFn   func(ctx context.Context, c *CollectorRun) error
	recordSystemFn      func(ctx context.Context, m *SystemMetrics) error
	recordAuditFn       func(ctx context.Context, e *AuditLogEntry) error
	recordAlertActivityFn func(ctx context.Context, a *AlertActivity) error
}

func (m *mockMonitoringDB) RecordHTTPRequest(ctx context.Context, r *HTTPRequest) error {
	if m.recordHTTPFn != nil {
		return m.recordHTTPFn(ctx, r)
	}
	return nil
}
func (m *mockMonitoringDB) RecordDBQuery(ctx context.Context, q *DBQuery) error {
	if m.recordDBQueryFn != nil {
		return m.recordDBQueryFn(ctx, q)
	}
	return nil
}
func (m *mockMonitoringDB) RecordCollectorRun(ctx context.Context, c *CollectorRun) error {
	if m.recordCollectorFn != nil {
		return m.recordCollectorFn(ctx, c)
	}
	return nil
}
func (m *mockMonitoringDB) RecordSystemMetrics(ctx context.Context, sm *SystemMetrics) error {
	if m.recordSystemFn != nil {
		return m.recordSystemFn(ctx, sm)
	}
	return nil
}
func (m *mockMonitoringDB) RecordAuditEvent(ctx context.Context, e *AuditLogEntry) error {
	if m.recordAuditFn != nil {
		return m.recordAuditFn(ctx, e)
	}
	return nil
}
func (m *mockMonitoringDB) RecordAlertActivity(ctx context.Context, a *AlertActivity) error {
	if m.recordAlertActivityFn != nil {
		return m.recordAlertActivityFn(ctx, a)
	}
	return nil
}

func TestNewSelfMonitor(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, 60*time.Second)
	require.NotNil(t, sm)
}

func TestNewSelfMonitor_DefaultInterval(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, 0)
	require.NotNil(t, sm)
	assert.Equal(t, 60*time.Second, sm.interval)
}

func TestSelfMonitor_Collect(t *testing.T) {
	t.Parallel()

	var recorded *SystemMetrics
	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			recorded = m
			return nil
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	sm.collect(context.Background())

	require.NotNil(t, recorded)
	assert.GreaterOrEqual(t, recorded.UptimeSeconds, int64(0))
	assert.Greater(t, recorded.GoroutineCount, 0)
	assert.GreaterOrEqual(t, recorded.HeapAllocBytes, int64(0))
	assert.Greater(t, recorded.NumCPU, 0)
	assert.False(t, recorded.Timestamp.IsZero())
}

func TestSelfMonitor_Collect_WithProviders(t *testing.T) {
	t.Parallel()

	var recorded *SystemMetrics
	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			recorded = m
			return nil
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	sm.WSConnectionCount = func() int { return 5 }
	sm.CaptureSessionCount = func() int { return 2 }
	sm.SchedulerJobCount = func() int { return 10 }
	sm.DBStats = func() (open, idle int, waitCount int64, waitDurationMs float64) {
		return 10, 5, 100, 50.0
	}
	sm.RequestStats = func() (total, active, errors int64) {
		return 1000, 50, 10
	}

	sm.collect(context.Background())

	require.NotNil(t, recorded)
	assert.Equal(t, 5, recorded.ActiveWSConnections)
	assert.Equal(t, 2, recorded.ActiveCaptureSessions)
	assert.Equal(t, 10, recorded.SchedulerJobsActive)
	assert.Equal(t, 10, recorded.DBOpenConnections)
	assert.Equal(t, 5, recorded.DBIdleConnections)
	assert.Equal(t, int64(100), recorded.DBWaitCount)
	assert.Equal(t, 50.0, recorded.DBWaitDurationMs)
	assert.Equal(t, int64(1000), recorded.RequestsTotal)
	assert.Equal(t, int64(50), recorded.RequestsActive)
	assert.Equal(t, int64(10), recorded.ErrorsTotal)
}

func TestSelfMonitor_StartStop(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			return nil
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, 10*time.Millisecond)

	sm.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	sm.Stop()
}

func TestSelfMonitor_StopWithoutStart(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	// Should not panic
	sm.Stop()
}

func TestSelfMonitor_Collect_DBError(t *testing.T) {
	t.Parallel()

	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			return assert.AnError
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	// Should not panic on DB error
	sm.collect(context.Background())
}

func TestSelfMonitor_MultipleCollects(t *testing.T) {
	t.Parallel()

	count := 0
	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			count++
			return nil
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	for i := 0; i < 5; i++ {
		sm.collect(context.Background())
	}

	assert.Equal(t, 5, count)
}

func TestSelfMonitor_Collect_RuntimeMetrics(t *testing.T) {
	t.Parallel()

	var recorded *SystemMetrics
	db := &mockMonitoringDB{
		recordSystemFn: func(ctx context.Context, m *SystemMetrics) error {
			recorded = m
			return nil
		},
	}
	recorder := NewRecorder(db)
	sm := NewSelfMonitor(recorder, time.Second)

	sm.collect(context.Background())

	require.NotNil(t, recorded)
	assert.GreaterOrEqual(t, recorded.GCRuns, 0)
	assert.Greater(t, recorded.HeapSysBytes, int64(0))
	assert.Greater(t, recorded.StackInUseBytes, int64(0))
}
