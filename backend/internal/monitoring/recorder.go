package monitoring

import (
	"context"
	"time"
)

// Recorder writes operational metrics to monitoring DB tables.
type Recorder struct {
	db MonitoringDB
}

// MonitoringDB defines the interface for persisting monitoring data.
type MonitoringDB interface {
	RecordHTTPRequest(ctx context.Context, r *HTTPRequest) error
	RecordDBQuery(ctx context.Context, q *DBQuery) error
	RecordCollectorRun(ctx context.Context, c *CollectorRun) error
	RecordSystemMetrics(ctx context.Context, m *SystemMetrics) error
	RecordAuditEvent(ctx context.Context, e *AuditLogEntry) error
	RecordAlertActivity(ctx context.Context, a *AlertActivity) error
}

// NewRecorder creates a monitoring recorder.
func NewRecorder(db MonitoringDB) *Recorder {
	return &Recorder{db: db}
}

// HTTPRequest represents a logged HTTP request for the monitoring_http_requests table.
type HTTPRequest struct {
	RequestID    string
	TraceID      string
	Method       string
	Path         string
	QueryString  string
	StatusCode   int
	DurationMs   float64
	RequestSize  int64
	ResponseSize int64
	RemoteAddr   string
	UserID       *int64
	AuthType     string
	UserAgent    string
	ErrorCode    string
	ErrorMessage string
	Timestamp    time.Time
}

// DBQuery represents a logged database query for the monitoring_db_queries table.
type DBQuery struct {
	RequestID    string
	TraceID      string
	Operation    string
	Table        string
	MethodName   string
	DurationMs   float64
	RowsReturned int
	RowsAffected int
	IsSlow       bool
	IsError      bool
	ErrorMessage string
	Timestamp    time.Time
}

// CollectorRun represents a logged collector execution for the monitoring_collector_runs table.
type CollectorRun struct {
	TraceID             string
	DeviceID            int64
	DeviceName          string
	Host                string
	Protocol            string
	SensorID            int64
	Status              string
	PreviousStatus      string
	StatusChanged       bool
	ResponseTimeMs      float64
	Value               float64
	Message             string
	ErrorMessage        string
	DurationMs          float64
	MetricID            int64
	AlertID             int64
	ConsecutiveFailures int
	Timestamp           time.Time
}

// SystemMetrics represents a health snapshot for the monitoring_app_health table.
type SystemMetrics struct {
	UptimeSeconds         int64     `json:"uptime_seconds"`
	GoroutineCount        int       `json:"goroutine_count"`
	HeapAllocBytes        int64     `json:"heap_alloc_bytes"`
	HeapSysBytes          int64     `json:"heap_sys_bytes"`
	StackInUseBytes       int64     `json:"stack_in_use_bytes"`
	GCPauseTotalNs        int64     `json:"gc_pause_total_ns"`
	GCRuns                int       `json:"gc_runs"`
	GCLastPauseNs         int64     `json:"gc_last_pause_ns"`
	NumCPU                int       `json:"num_cpu"`
	ActiveWSConnections   int       `json:"active_ws_connections"`
	ActiveCaptureSessions int       `json:"active_capture_sessions"`
	SchedulerJobsActive   int       `json:"scheduler_jobs_active"`
	DBOpenConnections     int       `json:"db_open_connections"`
	DBIdleConnections     int       `json:"db_idle_connections"`
	DBWaitCount           int64     `json:"db_wait_count"`
	DBWaitDurationMs      float64   `json:"db_wait_duration_ms"`
	RequestsTotal         int64     `json:"requests_total"`
	RequestsActive        int64     `json:"requests_active"`
	ErrorsTotal           int64     `json:"errors_total"`
	Timestamp             time.Time `json:"timestamp"`
}

// AuditLogEntry represents a security/audit event for the monitoring_audit_log table.
type AuditLogEntry struct {
	RequestID    string
	EventType    string
	Severity     string
	Actor        string
	ActorIP      string
	ResourceType string
	ResourceID   string
	Description  string
	Details      map[string]any
	Timestamp    time.Time
}

// AlertActivity represents an alert engine event for the monitoring_alert_activity table.
type AlertActivity struct {
	TraceID     string
	RuleID      int64
	RuleName    string
	DeviceID    int64
	DeviceName  string
	Action      string // evaluated, fired, notified, resolved, auto_resolved, cooldown_skip
	Severity    string
	AlertID     int64
	ChannelID   int64
	ChannelType string
	Details     map[string]any
	DurationMs  float64
	Error       string
	Timestamp   time.Time
}

// RecordHTTP persists an HTTP request record.
func (r *Recorder) RecordHTTP(ctx context.Context, req *HTTPRequest) error {
	return r.db.RecordHTTPRequest(ctx, req)
}

// RecordDB persists a database query record.
func (r *Recorder) RecordDB(ctx context.Context, query *DBQuery) error {
	return r.db.RecordDBQuery(ctx, query)
}

// RecordCollector persists a collector execution record.
func (r *Recorder) RecordCollector(ctx context.Context, run *CollectorRun) error {
	return r.db.RecordCollectorRun(ctx, run)
}

// RecordSystem persists a system health snapshot.
func (r *Recorder) RecordSystem(ctx context.Context, metrics *SystemMetrics) error {
	return r.db.RecordSystemMetrics(ctx, metrics)
}

// RecordAudit persists a security/audit event.
func (r *Recorder) RecordAudit(ctx context.Context, entry *AuditLogEntry) error {
	return r.db.RecordAuditEvent(ctx, entry)
}

// RecordAlert persists an alert engine activity record.
func (r *Recorder) RecordAlert(ctx context.Context, activity *AlertActivity) error {
	return r.db.RecordAlertActivity(ctx, activity)
}
