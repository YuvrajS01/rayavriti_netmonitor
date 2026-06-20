package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type StatusPageHandler struct {
	pool *pgxpool.Pool
}

func NewStatusPageHandler(db database.Database) *StatusPageHandler {
	pg, ok := db.(*database.Postgres)
	if !ok {
		slog.Warn("StatusPageHandler: database is not *Postgres, status page features will be unavailable")
		return &StatusPageHandler{}
	}
	return &StatusPageHandler{pool: pg.Pool()}
}

func (h *StatusPageHandler) PublicStatusJSON(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}

	services, err := h.getEnabledServices(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	groups := map[string][]map[string]any{}
	overall := "operational"

	for _, svc := range services {
		deviceStatuses, _ := h.getServiceDeviceStatuses(r.Context(), svc["id"].(int64))
		serviceStatus := h.deriveServiceStatus(svc, deviceStatuses)
		group, _ := svc["group_name"].(string)
		if group == "" {
			group = "General"
		}

		entry := map[string]any{
			"id":          svc["id"],
			"name":        svc["name"],
			"description": svc["description"],
			"status":      serviceStatus,
		}

		if showUptime, _ := svc["show_uptime"].(bool); showUptime {
			uptime := h.computeUptime(r.Context(), svc["id"].(int64))
			entry["uptime_30d"] = uptime
		}

		groups[group] = append(groups[group], entry)

		if serviceStatus != "operational" && overall == "operational" {
			overall = serviceStatus
		}
	}

	activeIncidents, _ := h.getActiveIncidents(r.Context())
	active := []map[string]any{}
	for _, inc := range activeIncidents {
		servicesAffected, _ := h.getIncidentServiceNames(r.Context(), inc["id"].(int64))
		inc["affected_services"] = servicesAffected
		active = append(active, inc)
		if overall == "operational" {
			overall = "degraded"
		}
	}

	groupRows := []map[string]any{}
	for name, items := range groups {
		groupRows = append(groupRows, map[string]any{"name": name, "services": items})
	}

	httputil.SendOK(w, map[string]any{
		"campus":           "Main Campus",
		"overall_status":   overall,
		"last_updated":     time.Now().UTC().Format(time.RFC3339),
		"groups":           groupRows,
		"active_incidents": active,
	})
}

func (h *StatusPageHandler) AddServiceDevice(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}
	serviceID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	var body struct {
		DeviceID int64 `json:"deviceId"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.DeviceID == 0 {
		httputil.SendError(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO status_page_service_devices(service_id, device_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		serviceID, body.DeviceID,
	)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendCreated(w, map[string]any{"serviceId": serviceID, "deviceId": body.DeviceID})
}

func (h *StatusPageHandler) RemoveServiceDevice(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}
	serviceID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	deviceID, err := parseID(chi.URLParam(r, "did"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`DELETE FROM status_page_service_devices WHERE service_id=$1 AND device_id=$2`,
		serviceID, deviceID,
	)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}

func (h *StatusPageHandler) ListServiceDevices(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}
	serviceID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid service id")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT d.id, d.name, d.ip_address, d.status
		 FROM status_page_service_devices spsd
		 JOIN devices d ON d.id = spsd.device_id
		 WHERE spsd.service_id=$1`, serviceID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	devices := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, ip, status string
		if err := rows.Scan(&id, &name, &ip, &status); err != nil {
			continue
		}
		devices = append(devices, map[string]any{
			"id":     id,
			"name":   name,
			"ip":     ip,
			"status": status,
		})
	}
	httputil.SendOK(w, devices)
}

func (h *StatusPageHandler) LinkIncidentServices(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}
	incidentID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid incident id")
		return
	}
	var body struct {
		ServiceIDs []int64 `json:"serviceIds"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}

	for _, serviceID := range body.ServiceIDs {
		_, _ = h.pool.Exec(r.Context(),
			`INSERT INTO status_page_incident_services(incident_id, service_id) VALUES($1,$2) ON CONFLICT DO NOTHING`,
			incidentID, serviceID,
		)
	}
	httputil.SendOK(w, map[string]any{"incidentId": incidentID, "serviceIds": body.ServiceIDs})
}

func (h *StatusPageHandler) ListIncidentUpdates(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "status page service unavailable")
		return
	}
	incidentID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid incident id")
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, incident_id, status, message, created_by, created_at
		 FROM status_page_incident_updates WHERE incident_id=$1 ORDER BY created_at ASC`, incidentID)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	updates := []map[string]any{}
	for rows.Next() {
		var id, incID int64
		var status, message string
		var createdBy *int64
		var createdAt time.Time
		if err := rows.Scan(&id, &incID, &status, &message, &createdBy, &createdAt); err != nil {
			continue
		}
		updates = append(updates, map[string]any{
			"id":         id,
			"incidentId": incID,
			"status":     status,
			"message":    message,
			"createdBy":  createdBy,
			"createdAt":  createdAt,
		})
	}
	httputil.SendOK(w, updates)
}

func (h *StatusPageHandler) getEnabledServices(ctx context.Context) ([]map[string]any, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, description, group_name, aggregation, display_order, show_response_time, show_uptime, enabled
		 FROM status_page_services WHERE enabled=true ORDER BY display_order ASC, id ASC`)
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
	return out, rows.Err()
}

func (h *StatusPageHandler) getServiceDeviceStatuses(ctx context.Context, serviceID int64) ([]string, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT d.status FROM status_page_service_devices spsd
		 JOIN devices d ON d.id = spsd.device_id
		 WHERE spsd.service_id=$1`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			continue
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

func (h *StatusPageHandler) deriveServiceStatus(svc map[string]any, deviceStatuses []string) string {
	if len(deviceStatuses) == 0 {
		return "operational"
	}
	aggregation, _ := svc["aggregation"].(string)

	downCount := 0
	for _, s := range deviceStatuses {
		if s == "down" || s == "critical" {
			downCount++
		}
	}

	switch aggregation {
	case "any_down":
		if downCount > 0 {
			return "major_outage"
		}
	case "all_down":
		if downCount == len(deviceStatuses) {
			return "major_outage"
		}
		if downCount > 0 {
			return "degraded_performance"
		}
	default:
		if downCount == len(deviceStatuses) {
			return "major_outage"
		}
		if downCount > len(deviceStatuses)/2 {
			return "major_outage"
		}
		if downCount > 0 {
			return "degraded_performance"
		}
	}
	return "operational"
}

func (h *StatusPageHandler) computeUptime(ctx context.Context, serviceID int64) float64 {
	var totalDevices, downDevices int
	_ = h.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE d.status IN ('down','critical'))
		 FROM status_page_service_devices spsd
		 JOIN devices d ON d.id = spsd.device_id
		 WHERE spsd.service_id=$1`, serviceID,
	).Scan(&totalDevices, &downDevices)

	if totalDevices == 0 {
		return 100.0
	}
	return float64(totalDevices-downDevices) / float64(totalDevices) * 100.0
}

func (h *StatusPageHandler) getActiveIncidents(ctx context.Context) ([]map[string]any, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, title, message, severity, status, started_at
		 FROM status_page_incidents WHERE resolved_at IS NULL ORDER BY started_at DESC`)
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
	return out, rows.Err()
}

func (h *StatusPageHandler) getIncidentServiceNames(ctx context.Context, incidentID int64) ([]string, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT sps.name FROM status_page_incident_services spis
		 JOIN status_page_services sps ON sps.id = spis.service_id
		 WHERE spis.incident_id=$1`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
