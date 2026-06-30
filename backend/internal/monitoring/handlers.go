package monitoring

import (
	"context"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
)

type MonitoringHandler struct {
	store   *Store
	runtime *logging.RuntimeControls
	legacy  legacyQueryDB
}

type legacyQueryDB interface {
	GetRecentHTTPRequests(ctx context.Context, limit int) ([]HTTPRequest, error)
	GetRecentDBQueries(ctx context.Context, limit int) ([]DBQuery, error)
	GetRecentCollectorRuns(ctx context.Context, limit int) ([]CollectorRun, error)
	GetRecentSystemMetrics(ctx context.Context, limit int) ([]SystemMetrics, error)
	GetRecentAuditLog(ctx context.Context, limit int) ([]AuditLogEntry, error)
}

func NewMonitoringHandler(source any, runtime ...*logging.RuntimeControls) *MonitoringHandler {
	h := &MonitoringHandler{}
	if len(runtime) > 0 {
		h.runtime = runtime[0]
	}
	if store, ok := source.(*Store); ok {
		h.store = store
		return h
	}
	if legacy, ok := source.(legacyQueryDB); ok {
		h.legacy = legacy
	}
	return h
}

func parseIntQuery(r *http.Request, key string, def int) int {
	return httputil.QueryParamInt(r, key, def, 0, 0)
}

func (h *MonitoringHandler) SystemLogs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.legacySystemLogs(w, r)
		return
	}
	q := parseLogQuery(r)
	events, total, err := h.store.QueryLogs(r.Context(), q)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query logs")
		return
	}
	httputil.SendOKWithMeta(w, map[string]any{"events": events}, &httputil.ResponseMeta{
		Page:     (q.Offset / q.Limit) + 1,
		PageSize: q.Limit,
		Total:    total,
	})
}

func (h *MonitoringHandler) SystemLogsStats(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.legacySystemLogsStats(w, r)
		return
	}
	stats, err := h.store.LogStats(r.Context(), parseLogQuery(r))
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query log stats")
		return
	}
	httputil.SendOK(w, stats)
}

func (h *MonitoringHandler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	q := parseLogQuery(r)
	q.Limit = 5000
	events, _, err := h.store.QueryLogs(r.Context(), q)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to export logs")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="netmonitor-logs.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"timestamp", "level", "component", "event_type", "message", "request_id", "user_id", "device_id", "path", "status_code", "duration_ms", "error"})
	for _, e := range events {
		_ = cw.Write([]string{
			e.Timestamp.Format(time.RFC3339),
			e.Level,
			e.Component,
			e.EventType,
			e.Message,
			e.RequestID,
			e.UserID,
			formatPtrInt64(e.DeviceID),
			e.Path,
			formatPtrInt(e.StatusCode),
			formatPtrFloat(e.DurationMs),
			e.Error,
		})
	}
	cw.Flush()
}

func (h *MonitoringHandler) ListVerboseSessions(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	sessions, err := h.store.ListVerboseSessions(r.Context(), activeOnly)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to query verbose sessions")
		return
	}
	httputil.SendOK(w, map[string]any{"sessions": sessions})
}

func (h *MonitoringHandler) CreateVerboseSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Level           string   `json:"level"`
		Components      []string `json:"components"`
		DeviceIDs       []int64  `json:"deviceIds"`
		UserIDs         []string `json:"userIds"`
		Reason          string   `json:"reason"`
		DurationMinutes int      `json:"durationMinutes"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Level = strings.ToLower(strings.TrimSpace(body.Level))
	if body.Level != "debug" && body.Level != "trace" {
		httputil.SendError(w, http.StatusBadRequest, "level must be debug or trace")
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Reason == "" {
		httputil.SendError(w, http.StatusBadRequest, "reason is required")
		return
	}
	if body.DurationMinutes <= 0 || body.DurationMinutes > 240 {
		httputil.SendError(w, http.StatusBadRequest, "durationMinutes must be between 1 and 240")
		return
	}
	var startedBy *int64
	if claims := auth.GetClaims(r.Context()); claims != nil {
		startedBy = &claims.UserID
	}
	session, err := h.store.CreateVerboseSession(r.Context(), body.Level, cleanStrings(body.Components), body.DeviceIDs, cleanStrings(body.UserIDs), body.Reason, startedBy, time.Now().Add(time.Duration(body.DurationMinutes)*time.Minute))
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create verbose session")
		return
	}
	h.runtime.UpsertVerboseSession(logging.VerboseSession{
		ID:         session.ID,
		Level:      session.Level,
		Components: session.Components,
		DeviceIDs:  session.DeviceIDs,
		UserIDs:    session.UserIDs,
		ExpiresAt:  session.ExpiresAt,
	})
	httputil.SendCreated(w, session)
}

func (h *MonitoringHandler) StopVerboseSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httputil.SendError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := h.store.StopVerboseSession(r.Context(), id); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to stop verbose session")
		return
	}
	h.runtime.StopVerboseSession(id)
	httputil.SendOK(w, map[string]any{"stopped": true})
}

func parseLogQuery(r *http.Request) LogQuery {
	limit := httputil.QueryParamInt(r, "limit", 100, 1, 1000)
	offset := httputil.QueryParamInt(r, "offset", 0, 0, 1000000)
	q := LogQuery{
		Level:     strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level"))),
		Component: strings.TrimSpace(r.URL.Query().Get("component")),
		EventType: strings.TrimSpace(r.URL.Query().Get("event_type")),
		UserID:    strings.TrimSpace(r.URL.Query().Get("user_id")),
		RequestID: strings.TrimSpace(r.URL.Query().Get("request_id")),
		TraceID:   strings.TrimSpace(r.URL.Query().Get("trace_id")),
		Search:    strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:     limit,
		Offset:    offset,
	}
	if raw := r.URL.Query().Get("from"); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			q.From = &ts
		}
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			q.To = &ts
		}
	}
	if raw := r.URL.Query().Get("device_id"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			q.DeviceID = &id
		}
	}
	return q
}

func cleanStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func formatPtrInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func formatPtrInt(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func formatPtrFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 3, 64)
}
