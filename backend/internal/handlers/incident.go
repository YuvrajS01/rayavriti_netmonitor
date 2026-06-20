package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type IncidentHandler struct {
	pool *pgxpool.Pool
	hub  *websocket.Hub
}

func NewIncidentHandler(db database.Database, hub *websocket.Hub) *IncidentHandler {
	pg, ok := db.(*database.Postgres)
	if !ok {
		slog.Warn("IncidentHandler: database is not *Postgres, incident features will be unavailable")
		return &IncidentHandler{hub: hub}
	}
	return &IncidentHandler{pool: pg.Pool(), hub: hub}
}

func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	var body struct {
		Title             string  `json:"title"`
		Description       string  `json:"description"`
		Severity          string  `json:"severity"`
		Source            string  `json:"source"`
		SourceAlertID     *int64  `json:"sourceAlertId"`
		LocationID        *int64  `json:"locationId"`
		ImpactDescription string  `json:"impactDescription"`
		AffectedDeviceIDs []int64 `json:"affectedDeviceIds"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Title == "" || body.Severity == "" {
		httputil.SendError(w, http.StatusBadRequest, "title and severity are required")
		return
	}
	if body.Source == "" {
		body.Source = "manual"
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var incidentID int64
	err = tx.QueryRow(r.Context(),
		`INSERT INTO incidents(title, description, severity, status, source, source_alert_id, location_id, impact_description, affected_device_count, created_by)
		 VALUES($1,$2,$3,'open',$4,$5,$6,$7,$8,$9) RETURNING id`,
		body.Title, body.Description, body.Severity, body.Source,
		body.SourceAlertID, body.LocationID, body.ImpactDescription,
		len(body.AffectedDeviceIDs), h.actorID(r),
	).Scan(&incidentID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to create incident: "+err.Error())
		return
	}

	for _, deviceID := range body.AffectedDeviceIDs {
		_, _ = tx.Exec(r.Context(),
			`INSERT INTO incident_devices(incident_id, device_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
			incidentID, deviceID,
		)
	}

	_, _ = tx.Exec(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, new_value, message, author)
		 VALUES($1,'created','open','Incident created',$2)`,
		incidentID, h.actorString(r),
	)

	if err := tx.Commit(r.Context()); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	item, err := h.getIncident(r.Context(), incidentID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, "failed to fetch incident")
		return
	}
	httputil.SendCreated(w, item)
}

func (h *IncidentHandler) AcknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var currentStatus string
	err = h.pool.QueryRow(r.Context(), `SELECT status FROM incidents WHERE id=$1`, id).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.SendError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if currentStatus != "open" {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("cannot acknowledge incident in %s status", currentStatus))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE incidents SET status='acknowledged', acknowledged_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, old_value, new_value, message, author)
		 VALUES($1,'status_change','open','acknowledged','Incident acknowledged',$2)`,
		id, h.actorString(r),
	)

	item, _ := h.getIncident(r.Context(), id)
	httputil.SendOK(w, item)
}

func (h *IncidentHandler) ResolveIncident(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Resolution        string `json:"resolution"`
		RootCause         string `json:"rootCause"`
		RootCauseCategory string `json:"rootCauseCategory"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}

	var startedAt time.Time
	var currentStatus string
	err = h.pool.QueryRow(r.Context(), `SELECT status, started_at FROM incidents WHERE id=$1`, id).Scan(&currentStatus, &startedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.SendError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if currentStatus == "resolved" || currentStatus == "closed" {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("cannot resolve incident in %s status", currentStatus))
		return
	}

	duration := int(time.Since(startedAt).Seconds())
	slaBreached := h.checkSLABreach(r.Context(), currentStatus, duration)

	_, err = h.pool.Exec(r.Context(),
		`UPDATE incidents SET status='resolved', resolution=$1, root_cause=$2, root_cause_category=$3,
		 resolved_at=NOW(), duration_seconds=$4, sla_breached=$5, updated_at=NOW() WHERE id=$6`,
		body.Resolution, body.RootCause, body.RootCauseCategory, duration, slaBreached, id,
	)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, old_value, new_value, message, author)
		 VALUES($1,'status_change',$2,'resolved','Incident resolved',$3)`,
		id, currentStatus, h.actorString(r),
	)

	item, _ := h.getIncident(r.Context(), id)
	httputil.SendOK(w, item)
}

func (h *IncidentHandler) CloseIncident(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var currentStatus string
	err = h.pool.QueryRow(r.Context(), `SELECT status FROM incidents WHERE id=$1`, id).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.SendError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if currentStatus != "resolved" {
		httputil.SendError(w, http.StatusBadRequest, "can only close a resolved incident")
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE incidents SET status='closed', closed_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, old_value, new_value, message, author)
		 VALUES($1,'status_change','resolved','closed','Incident closed',$2)`,
		id, h.actorString(r),
	)

	item, _ := h.getIncident(r.Context(), id)
	httputil.SendOK(w, item)
}

func (h *IncidentHandler) AssignIncident(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		AssignedTo int64 `json:"assignedTo"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}

	var prevAssigned *int64
	err = h.pool.QueryRow(r.Context(), `SELECT assigned_to FROM incidents WHERE id=$1`, id).Scan(&prevAssigned)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.SendError(w, http.StatusNotFound, "incident not found")
		return
	}
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE incidents SET assigned_to=$1, updated_at=NOW() WHERE id=$2`, body.AssignedTo, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	prevVal := "unassigned"
	if prevAssigned != nil {
		prevVal = strconv.FormatInt(*prevAssigned, 10)
	}
	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, old_value, new_value, message, author)
		 VALUES($1,'assignment',$2,$3,'Incident assigned',$4)`,
		id, prevVal, strconv.FormatInt(body.AssignedTo, 10), h.actorString(r),
	)

	item, _ := h.getIncident(r.Context(), id)
	httputil.SendOK(w, item)
}

func (h *IncidentHandler) AddTimelineEntry(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		EntryType string `json:"entryType"`
		Message   string `json:"message"`
		OldValue  string `json:"oldValue"`
		NewValue  string `json:"newValue"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.EntryType == "" {
		body.EntryType = "note"
	}
	if body.Message == "" {
		httputil.SendError(w, http.StatusBadRequest, "message is required")
		return
	}

	var exists bool
	_ = h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM incidents WHERE id=$1)`, id).Scan(&exists)
	if !exists {
		httputil.SendError(w, http.StatusNotFound, "incident not found")
		return
	}

	var entryID int64
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO incident_timeline(incident_id, entry_type, old_value, new_value, message, author)
		 VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		id, body.EntryType, body.OldValue, body.NewValue, body.Message, h.actorString(r),
	).Scan(&entryID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.SendCreated(w, map[string]any{
		"id":         entryID,
		"incidentId": id,
		"entryType":  body.EntryType,
		"message":    body.Message,
		"author":     h.actorString(r),
	})
}

func (h *IncidentHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, incident_id, entry_type, old_value, new_value, message, author, created_at
		 FROM incident_timeline WHERE incident_id=$1 ORDER BY created_at ASC`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	entries := []map[string]any{}
	for rows.Next() {
		var entryID, incID int64
		var entryType, message string
		var oldVal, newVal, author *string
		var createdAt time.Time
		if err := rows.Scan(&entryID, &incID, &entryType, &oldVal, &newVal, &message, &author, &createdAt); err != nil {
			continue
		}
		entries = append(entries, map[string]any{
			"id":         entryID,
			"incidentId": incID,
			"entryType":  entryType,
			"oldValue":   oldVal,
			"newValue":   newVal,
			"message":    message,
			"author":     author,
			"createdAt":  createdAt,
		})
	}
	httputil.SendOK(w, entries)
}

func (h *IncidentHandler) GetIncidentDevices(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT d.id, d.name, d.ip_address, d.status
		 FROM incident_devices id2
		 JOIN devices d ON d.id = id2.device_id
		 WHERE id2.incident_id=$1`, id)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	devices := []map[string]any{}
	for rows.Next() {
		var devID int64
		var name, ip, status string
		if err := rows.Scan(&devID, &name, &ip, &status); err != nil {
			continue
		}
		devices = append(devices, map[string]any{
			"id":     devID,
			"name":   name,
			"ip":     ip,
			"status": status,
		})
	}
	httputil.SendOK(w, devices)
}

func (h *IncidentHandler) GetIncidentStats(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}

	stats := map[string]any{}

	var openCount, ackCount, resolvedCount, closedCount, breachCount int
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE status='open'`).Scan(&openCount)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE status='acknowledged'`).Scan(&ackCount)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE status='resolved'`).Scan(&resolvedCount)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE status='closed'`).Scan(&closedCount)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM incidents WHERE sla_breached=true`).Scan(&breachCount)
	stats["open"] = openCount
	stats["acknowledged"] = ackCount
	stats["resolved"] = resolvedCount
	stats["closed"] = closedCount
	stats["sla_breached"] = breachCount
	stats["total"] = openCount + ackCount + resolvedCount + closedCount

	var avgDuration *float64
	_ = h.pool.QueryRow(r.Context(),
		`SELECT AVG(duration_seconds) FROM incidents WHERE duration_seconds IS NOT NULL AND status IN ('resolved','closed')`).Scan(&avgDuration)
	if avgDuration != nil {
		stats["avg_duration_seconds"] = *avgDuration
	}

	var avgResponse *float64
	_ = h.pool.QueryRow(r.Context(),
		`SELECT AVG(EXTRACT(EPOCH FROM (acknowledged_at - started_at)))
		 FROM incidents WHERE acknowledged_at IS NOT NULL`).Scan(&avgResponse)
	if avgResponse != nil {
		stats["avg_response_seconds"] = *avgResponse
	}

	httputil.SendOK(w, stats)
}

func (h *IncidentHandler) GetSLAReport(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "incident service unavailable")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT s.id, s.name, s.severity, s.response_time_minutes, s.resolution_time_minutes, s.enabled,
		 COUNT(i.id) as total_incidents,
		 COUNT(i.id) FILTER (WHERE i.sla_breached) as breached_count
		 FROM sla_definitions s
		 LEFT JOIN incidents i ON i.severity = s.severity
		 GROUP BY s.id ORDER BY s.id`)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	reports := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, severity string
		var respMin, resMin int
		var enabled bool
		var total, breached int
		if err := rows.Scan(&id, &name, &severity, &respMin, &resMin, &enabled, &total, &breached); err != nil {
			continue
		}
		compliance := 100.0
		if total > 0 {
			compliance = float64(total-breached) / float64(total) * 100
		}
		reports = append(reports, map[string]any{
			"id":                    id,
			"name":                  name,
			"severity":              severity,
			"responseTimeMinutes":   respMin,
			"resolutionTimeMinutes": resMin,
			"enabled":               enabled,
			"totalIncidents":        total,
			"breachedCount":         breached,
			"compliancePercent":     fmt.Sprintf("%.1f", compliance),
		})
	}
	httputil.SendOK(w, reports)
}

func (h *IncidentHandler) getIncident(ctx context.Context, id int64) (map[string]any, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, title, description, severity, status, root_cause, root_cause_category, resolution,
		 source, source_alert_id, assigned_to, location_id, impact_description, affected_device_count,
		 started_at, acknowledged_at, resolved_at, closed_at, duration_seconds, sla_breached, created_by, created_at
		 FROM incidents WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	out := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(map[string]any, len(fields))
		for i, fd := range fields {
			item[fd.Name] = vals[i]
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return out[0], nil
}

func (h *IncidentHandler) checkSLABreach(ctx context.Context, severity string, durationSeconds int) bool {
	var resMin int
	err := h.pool.QueryRow(ctx,
		`SELECT resolution_time_minutes FROM sla_definitions WHERE severity=$1 AND enabled=true`, severity,
	).Scan(&resMin)
	if err != nil {
		return false
	}
	return durationSeconds > resMin*60
}

func (h *IncidentHandler) actorString(r *http.Request) string {
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		return fmt.Sprintf("user:%d", claims.UserID)
	}
	return "system"
}

func (h *IncidentHandler) actorID(r *http.Request) *int64 {
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		return &claims.UserID
	}
	return nil
}
