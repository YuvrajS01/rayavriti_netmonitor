package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

// MonitoringQueryDB defines the query interface needed by monitoring handlers.
type MonitoringQueryDB interface {
	GetRecentHTTPRequests(ctx context.Context, limit int) ([]HTTPRequest, error)
	GetRecentDBQueries(ctx context.Context, limit int) ([]DBQuery, error)
	GetRecentCollectorRuns(ctx context.Context, limit int) ([]CollectorRun, error)
	GetRecentSystemMetrics(ctx context.Context, limit int) ([]SystemMetrics, error)
	GetRecentAuditLog(ctx context.Context, limit int) ([]AuditLogEntry, error)
}

// MonitoringHandler serves the system monitoring API endpoints.
type MonitoringHandler struct {
	db MonitoringQueryDB
}

// NewMonitoringHandler creates a monitoring handler.
func NewMonitoringHandler(db MonitoringQueryDB) *MonitoringHandler {
	return &MonitoringHandler{db: db}
}

// parseIntQuery extracts an int from a query parameter with a default.
func parseIntQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// SystemLogs returns structured logs from monitoring DB tables.
// GET /api/v1/system/logs
//
//	?component=http|db|collector|audit|alert_engine|websocket|scheduler
//	?level=trace|debug|info|warn|error
//	?from=ISO8601&to=ISO8601
//	?device_id=5
//	?request_id=a1b2c3d4
//	?limit=100&offset=0
func (h *MonitoringHandler) SystemLogs(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 100)
	component := r.URL.Query().Get("component")

	var result map[string]any

	switch component {
	case "http":
		data, err := h.db.GetRecentHTTPRequests(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query HTTP logs")
			return
		}
		result = map[string]any{"component": "http", "data": data, "count": len(data)}
	case "db":
		data, err := h.db.GetRecentDBQueries(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query DB logs")
			return
		}
		result = map[string]any{"component": "db", "data": data, "count": len(data)}
	case "collector":
		data, err := h.db.GetRecentCollectorRuns(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query collector logs")
			return
		}
		result = map[string]any{"component": "collector", "data": data, "count": len(data)}
	case "audit":
		data, err := h.db.GetRecentAuditLog(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query audit logs")
			return
		}
		result = map[string]any{"component": "audit", "data": data, "count": len(data)}
	default:
		// Return all categories
		httpReqs, _ := h.db.GetRecentHTTPRequests(r.Context(), limit)
		dbQueries, _ := h.db.GetRecentDBQueries(r.Context(), limit)
		collectorRuns, _ := h.db.GetRecentCollectorRuns(r.Context(), limit)
		auditLog, _ := h.db.GetRecentAuditLog(r.Context(), limit)
		result = map[string]any{
			"http_requests":  httpReqs,
			"db_queries":     dbQueries,
			"collector_runs": collectorRuns,
			"audit_log":      auditLog,
		}
	}

	httputil.SendOK(w, result)
}

// SystemLogsStats returns log volume statistics.
// GET /api/v1/system/logs/stats
func (h *MonitoringHandler) SystemLogsStats(w http.ResponseWriter, r *http.Request) {
	httpReqs, err := h.db.GetRecentHTTPRequests(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query HTTP logs for stats")
		return
	}
	dbQueries, err := h.db.GetRecentDBQueries(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query DB logs for stats")
		return
	}
	collectorRuns, err := h.db.GetRecentCollectorRuns(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query collector logs for stats")
		return
	}
	auditLog, err := h.db.GetRecentAuditLog(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query audit logs for stats")
		return
	}

	type componentStats struct {
		Count     int            `json:"count"`
		ByStatus  map[string]int `json:"byStatus,omitempty"`
		ByHour    map[string]int `json:"byHour,omitempty"`
		AvgDurMs  float64        `json:"avgDurationMs,omitempty"`
		SlowCount int            `json:"slowCount,omitempty"`
	}

	httpStats := componentStats{
		Count:    len(httpReqs),
		ByStatus: make(map[string]int),
		ByHour:   make(map[string]int),
	}
	var totalDur float64
	for _, req := range httpReqs {
		httpStats.ByStatus[fmt.Sprintf("%d", req.StatusCode)]++
		httpStats.ByHour[req.Timestamp.Format("15:00")]++
		totalDur += req.DurationMs
	}
	if len(httpReqs) > 0 {
		httpStats.AvgDurMs = totalDur / float64(len(httpReqs))
	}

	dbStats := componentStats{
		Count:    len(dbQueries),
		ByStatus: make(map[string]int),
		ByHour:   make(map[string]int),
	}
	var totalDbDur float64
	for _, q := range dbQueries {
		if q.IsError {
			dbStats.ByStatus["error"]++
		} else {
			dbStats.ByStatus["ok"]++
		}
		dbStats.ByHour[q.Timestamp.Format("15:00")]++
		totalDbDur += q.DurationMs
		if q.IsSlow {
			dbStats.SlowCount++
		}
	}
	if len(dbQueries) > 0 {
		dbStats.AvgDurMs = totalDbDur / float64(len(dbQueries))
	}

	collectorStats := componentStats{
		Count:    len(collectorRuns),
		ByStatus: make(map[string]int),
		ByHour:   make(map[string]int),
	}
	for _, run := range collectorRuns {
		collectorStats.ByStatus[run.Status]++
		collectorStats.ByHour[run.Timestamp.Format("15:00")]++
	}

	auditStats := componentStats{
		Count:    len(auditLog),
		ByStatus: make(map[string]int),
		ByHour:   make(map[string]int),
	}
	for _, entry := range auditLog {
		auditStats.ByStatus[entry.EventType]++
		auditStats.ByHour[entry.Timestamp.Format("15:00")]++
	}

	httputil.SendOK(w, map[string]any{
		"http_requests":  httpStats,
		"db_queries":     dbStats,
		"collector_runs": collectorStats,
		"audit_log":      auditStats,
	})
}

// SystemMonitoring returns current application health snapshot.
// GET /api/v1/system/monitoring
func (h *MonitoringHandler) SystemMonitoring(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.db.GetRecentSystemMetrics(r.Context(), 1)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query system metrics")
		return
	}
	if len(metrics) == 0 {
		httputil.SendOK(w, map[string]any{"message": "no health snapshots yet"})
		return
	}
	httputil.SendOK(w, metrics[0])
}

// SystemMonitoringHistory returns historical health snapshots.
// GET /api/v1/system/monitoring/history?hours=24
func (h *MonitoringHandler) SystemMonitoringHistory(w http.ResponseWriter, r *http.Request) {
	hours := parseIntQuery(r, "hours", 24)
	// Use hours * 60 as a rough limit (one snapshot per minute)
	limit := hours * 60
	if limit > 1440 {
		limit = 1440
	}
	metrics, err := h.db.GetRecentSystemMetrics(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query system metrics history")
		return
	}
	httputil.SendOK(w, map[string]any{
		"hours":   hours,
		"count":   len(metrics),
		"metrics": metrics,
	})
}

// SystemMonitoringRequests returns HTTP request performance analytics.
// GET /api/v1/system/monitoring/requests
//
//	?from=ISO8601&to=ISO8601
//	?path=/api/v1/devices
//	?min_duration_ms=100
//	?status_code=500
func (h *MonitoringHandler) SystemMonitoringRequests(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 100)
	requests, err := h.db.GetRecentHTTPRequests(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query request analytics")
		return
	}

	// Filter by query params if provided
	path := r.URL.Query().Get("path")
	statusCode := parseIntQuery(r, "status_code", 0)
	minDuration := parseIntQuery(r, "min_duration_ms", 0)

	var filtered []HTTPRequest
	for _, req := range requests {
		if path != "" && req.Path != path {
			continue
		}
		if statusCode > 0 && req.StatusCode != statusCode {
			continue
		}
		if minDuration > 0 && req.DurationMs < float64(minDuration) {
			continue
		}
		filtered = append(filtered, req)
	}

	httputil.SendOK(w, map[string]any{
		"count":    len(filtered),
		"requests": filtered,
	})
}

// SystemMonitoringQueries returns slow query analysis.
// GET /api/v1/system/monitoring/queries
//
//	?slow_only=true
//	?method=GetMetricsForReport
func (h *MonitoringHandler) SystemMonitoringQueries(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 100)
	queries, err := h.db.GetRecentDBQueries(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query DB performance")
		return
	}

	slowOnly := r.URL.Query().Get("slow_only") == "true"
	method := r.URL.Query().Get("method")

	var filtered []DBQuery
	for _, q := range queries {
		if slowOnly && !q.IsSlow {
			continue
		}
		if method != "" && q.MethodName != method {
			continue
		}
		filtered = append(filtered, q)
	}

	httputil.SendOK(w, map[string]any{
		"count":   len(filtered),
		"queries": filtered,
	})
}

// SystemAuditLog returns security audit trail.
// GET /api/v1/system/audit-log
//
//	?event_type=auth.login_failure
//	?actor=user:admin
//	?from=ISO8601&to=ISO8601
func (h *MonitoringHandler) SystemAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 100)
	entries, err := h.db.GetRecentAuditLog(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query audit log")
		return
	}

	eventType := r.URL.Query().Get("event_type")
	actor := r.URL.Query().Get("actor")

	var filtered []AuditLogEntry
	for _, e := range entries {
		if eventType != "" && e.EventType != eventType {
			continue
		}
		if actor != "" && e.Actor != actor {
			continue
		}
		filtered = append(filtered, e)
	}

	httputil.SendOK(w, map[string]any{
		"count":   len(filtered),
		"entries": filtered,
	})
}

// SystemCollectorsStats returns collector success/failure rates per device.
// GET /api/v1/system/collectors/stats
//
//	?device_id=5
//	?hours=24
func (h *MonitoringHandler) SystemCollectorsStats(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 500)
	runs, err := h.db.GetRecentCollectorRuns(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query collector stats")
		return
	}

	deviceID := int64(parseIntQuery(r, "device_id", 0))

	// Aggregate stats by device
	type deviceStats struct {
		DeviceID   int64   `json:"device_id"`
		DeviceName string  `json:"device_name"`
		Protocol   string  `json:"protocol"`
		Total      int     `json:"total"`
		Success    int     `json:"success"`
		Failure    int     `json:"failure"`
		AvgDurMs   float64 `json:"avg_duration_ms"`
	}

	statsMap := map[int64]*deviceStats{}
	for _, run := range runs {
		if deviceID > 0 && run.DeviceID != deviceID {
			continue
		}
		ds, ok := statsMap[run.DeviceID]
		if !ok {
			ds = &deviceStats{
				DeviceID:   run.DeviceID,
				DeviceName: run.DeviceName,
				Protocol:   run.Protocol,
			}
			statsMap[run.DeviceID] = ds
		}
		ds.Total++
		if run.Status == "up" {
			ds.Success++
		} else {
			ds.Failure++
		}
		ds.AvgDurMs += run.DurationMs
	}

	var stats []deviceStats
	for _, ds := range statsMap {
		if ds.Total > 0 {
			ds.AvgDurMs /= float64(ds.Total)
		}
		stats = append(stats, *ds)
	}

	httputil.SendOK(w, map[string]any{
		"count": len(stats),
		"stats": stats,
	})
}
