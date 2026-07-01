package monitoring

import (
	"fmt"
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

func (h *MonitoringHandler) legacySystemLogs(w http.ResponseWriter, r *http.Request) {
	if h.legacy == nil {
		httputil.SendError(w, http.StatusNotImplemented, "monitoring store unavailable")
		return
	}
	limit := httputil.QueryParamInt(r, "limit", 100, 1, 1000)
	component := r.URL.Query().Get("component")
	switch component {
	case "http":
		data, err := h.legacy.GetRecentHTTPRequests(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query HTTP logs")
			return
		}
		httputil.SendOK(w, map[string]any{"component": "http", "data": data, "count": len(data)})
	case "db":
		data, err := h.legacy.GetRecentDBQueries(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query DB logs")
			return
		}
		httputil.SendOK(w, map[string]any{"component": "db", "data": data, "count": len(data)})
	case "collector":
		data, err := h.legacy.GetRecentCollectorRuns(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query collector logs")
			return
		}
		httputil.SendOK(w, map[string]any{"component": "collector", "data": data, "count": len(data)})
	case "audit":
		data, err := h.legacy.GetRecentAuditLog(r.Context(), limit)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, "failed to query audit logs")
			return
		}
		httputil.SendOK(w, map[string]any{"component": "audit", "data": data, "count": len(data)})
	default:
		httpReqs, _ := h.legacy.GetRecentHTTPRequests(r.Context(), limit)
		dbQueries, _ := h.legacy.GetRecentDBQueries(r.Context(), limit)
		collectorRuns, _ := h.legacy.GetRecentCollectorRuns(r.Context(), limit)
		auditLog, _ := h.legacy.GetRecentAuditLog(r.Context(), limit)
		httputil.SendOK(w, map[string]any{"http_requests": httpReqs, "db_queries": dbQueries, "collector_runs": collectorRuns, "audit_log": auditLog})
	}
}

func (h *MonitoringHandler) legacySystemLogsStats(w http.ResponseWriter, r *http.Request) {
	if h.legacy == nil {
		httputil.SendError(w, http.StatusNotImplemented, "monitoring store unavailable")
		return
	}
	httpReqs, err := h.legacy.GetRecentHTTPRequests(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query HTTP logs for stats")
		return
	}
	dbQueries, err := h.legacy.GetRecentDBQueries(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query DB logs for stats")
		return
	}
	collectorRuns, err := h.legacy.GetRecentCollectorRuns(r.Context(), 1000)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query collector logs for stats")
		return
	}
	auditLog, err := h.legacy.GetRecentAuditLog(r.Context(), 1000)
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
	httpStats := componentStats{Count: len(httpReqs), ByStatus: map[string]int{}, ByHour: map[string]int{}}
	var totalHTTP float64
	for _, req := range httpReqs {
		httpStats.ByStatus[fmt.Sprintf("%d", req.StatusCode)]++
		httpStats.ByHour[req.Timestamp.Format("15:00")]++
		totalHTTP += req.DurationMs
	}
	if len(httpReqs) > 0 {
		httpStats.AvgDurMs = totalHTTP / float64(len(httpReqs))
	}
	dbStats := componentStats{Count: len(dbQueries), ByStatus: map[string]int{}, ByHour: map[string]int{}}
	var totalDB float64
	for _, q := range dbQueries {
		if q.IsError {
			dbStats.ByStatus["error"]++
		} else {
			dbStats.ByStatus["ok"]++
		}
		dbStats.ByHour[q.Timestamp.Format("15:00")]++
		totalDB += q.DurationMs
		if q.IsSlow {
			dbStats.SlowCount++
		}
	}
	if len(dbQueries) > 0 {
		dbStats.AvgDurMs = totalDB / float64(len(dbQueries))
	}
	collectorStats := componentStats{Count: len(collectorRuns), ByStatus: map[string]int{}, ByHour: map[string]int{}}
	for _, run := range collectorRuns {
		collectorStats.ByStatus[run.Status]++
		collectorStats.ByHour[run.Timestamp.Format("15:00")]++
	}
	auditStats := componentStats{Count: len(auditLog), ByStatus: map[string]int{}, ByHour: map[string]int{}}
	for _, entry := range auditLog {
		auditStats.ByStatus[entry.EventType]++
		auditStats.ByHour[entry.Timestamp.Format("15:00")]++
	}
	httputil.SendOK(w, map[string]any{"http_requests": httpStats, "db_queries": dbStats, "collector_runs": collectorStats, "audit_log": auditStats})
}

func (h *MonitoringHandler) SystemMonitoring(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.legacy.GetRecentSystemMetrics(r.Context(), 1)
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

func (h *MonitoringHandler) SystemMonitoringHistory(w http.ResponseWriter, r *http.Request) {
	hours := httputil.QueryParamInt(r, "hours", 24, 1, 168)
	limit := hours * 60
	if limit > 1440 {
		limit = 1440
	}
	metrics, err := h.legacy.GetRecentSystemMetrics(r.Context(), limit)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query system metrics history")
		return
	}
	httputil.SendOK(w, map[string]any{"hours": hours, "count": len(metrics), "metrics": metrics})
}

func (h *MonitoringHandler) SystemMonitoringRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := h.legacy.GetRecentHTTPRequests(r.Context(), httputil.QueryParamInt(r, "limit", 100, 1, 1000))
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query request analytics")
		return
	}
	path := r.URL.Query().Get("path")
	statusCode := httputil.QueryParamInt(r, "status_code", 0, 0, 599)
	minDuration := httputil.QueryParamInt(r, "min_duration_ms", 0, 0, 600000)
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
	httputil.SendOK(w, map[string]any{"count": len(filtered), "requests": filtered})
}

func (h *MonitoringHandler) SystemMonitoringQueries(w http.ResponseWriter, r *http.Request) {
	queries, err := h.legacy.GetRecentDBQueries(r.Context(), httputil.QueryParamInt(r, "limit", 100, 1, 1000))
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
	httputil.SendOK(w, map[string]any{"count": len(filtered), "queries": filtered})
}

func (h *MonitoringHandler) SystemAuditLog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.legacy.GetRecentAuditLog(r.Context(), httputil.QueryParamInt(r, "limit", 100, 1, 1000))
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
	httputil.SendOK(w, map[string]any{"count": len(filtered), "entries": filtered})
}

func (h *MonitoringHandler) SystemCollectorsStats(w http.ResponseWriter, r *http.Request) {
	runs, err := h.legacy.GetRecentCollectorRuns(r.Context(), httputil.QueryParamInt(r, "limit", 500, 1, 2000))
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query collector stats")
		return
	}
	deviceID := int64(httputil.QueryParamInt(r, "device_id", 0, 0, 1<<30))
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
		ds := statsMap[run.DeviceID]
		if ds == nil {
			ds = &deviceStats{DeviceID: run.DeviceID, DeviceName: run.DeviceName, Protocol: run.Protocol}
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
	httputil.SendOK(w, map[string]any{"count": len(stats), "stats": stats})
}
