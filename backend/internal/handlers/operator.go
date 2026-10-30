package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

// OperatorHandler serves the v5.1 operator-workflow endpoints: metric
// history for charts, the alerts workbench (bulk actions, notes, assignee)
// and global search for the command palette. Every endpoint applies the
// caller's location/subnet scopes.
type OperatorHandler struct {
	db   database.Database
	pool *pgxpool.Pool
	hub  *websocket.Hub
}

func NewOperatorHandler(db database.Database, pool *pgxpool.Pool, hub *websocket.Hub) *OperatorHandler {
	return &OperatorHandler{db: db, pool: pool, hub: hub}
}

const (
	maxHistoryDevices = 20
	maxBulkAlerts     = 500
	maxNoteLen        = 2000
)

// ── Metric history ───────────────────────────────────────────────────────

type historyPoint struct {
	T       time.Time `json:"t"`
	Avg     *float64  `json:"avg"`
	Min     *float64  `json:"min"`
	Max     *float64  `json:"max"`
	Samples int64     `json:"samples"`
	Down    int64     `json:"down"`
}

type deviceHistory struct {
	DeviceID   int64          `json:"deviceId"`
	DeviceName string         `json:"deviceName"`
	Points     []historyPoint `json:"points"`
}

var historyBuckets = map[string]time.Duration{
	"1m": time.Minute, "5m": 5 * time.Minute, "15m": 15 * time.Minute, "1h": time.Hour, "1d": 24 * time.Hour,
}

// pickBucket chooses a bucket that yields at most a few hundred points.
func pickBucket(span time.Duration) string {
	switch {
	case span <= 3*time.Hour:
		return "1m"
	case span <= 24*time.Hour:
		return "5m"
	case span <= 3*24*time.Hour:
		return "15m"
	case span <= 31*24*time.Hour:
		return "1h"
	default:
		return "1d"
	}
}

// History returns per-device response-time series. Buckets of an hour or
// more are served from the metrics_hourly rollup; finer ones from raw rows.
func (h *OperatorHandler) History(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ids, err := parseIDList(q.Get("devices"))
	if err != nil || len(ids) == 0 {
		httputil.SendError(w, http.StatusBadRequest, "devices must be a comma-separated list of ids")
		return
	}
	if len(ids) > maxHistoryDevices {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("at most %d devices per request", maxHistoryDevices))
		return
	}
	to := time.Now()
	from := to.Add(-time.Hour)
	if v := q.Get("from"); v != "" {
		if from, err = time.Parse(time.RFC3339, v); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "from must be RFC3339")
			return
		}
	}
	if v := q.Get("to"); v != "" {
		if to, err = time.Parse(time.RFC3339, v); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "to must be RFC3339")
			return
		}
	}
	if !from.Before(to) {
		httputil.SendError(w, http.StatusBadRequest, "from must be before to")
		return
	}
	bucket := q.Get("bucket")
	if bucket == "" {
		bucket = pickBucket(to.Sub(from))
	}
	width, ok := historyBuckets[bucket]
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "bucket must be one of 1m, 5m, 15m, 1h, 1d")
		return
	}

	allowed, err := scopedDeviceIDs(r.Context(), r, h.db)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if allowed != nil {
		kept := ids[:0]
		for _, id := range ids {
			if allowed[id] {
				kept = append(kept, id)
			}
		}
		ids = kept
	}

	var query string
	if width >= time.Hour {
		query = `
			SELECT h.device_id, COALESCE(d.name,''), time_bucket($4::interval, h.bucket) AS b,
			       SUM(h.rt_sum) / NULLIF(SUM(h.rt_count), 0), MIN(h.rt_min), MAX(h.rt_max),
			       SUM(h.samples)::bigint, SUM(h.down_samples)::bigint
			FROM metrics_hourly h JOIN devices d ON d.id = h.device_id
			WHERE h.device_id = ANY($1) AND h.bucket >= date_trunc('hour', $2::timestamptz) AND h.bucket < $3
			GROUP BY h.device_id, d.name, b ORDER BY h.device_id, b`
	} else {
		query = `
			SELECT m.device_id, COALESCE(d.name,''), time_bucket($4::interval, m.timestamp) AS b,
			       AVG(m.response_time), MIN(m.response_time), MAX(m.response_time),
			       COUNT(*), COUNT(*) FILTER (WHERE m.status = 'down')
			FROM metrics m JOIN devices d ON d.id = m.device_id
			WHERE m.device_id = ANY($1) AND m.timestamp >= $2 AND m.timestamp < $3
			GROUP BY m.device_id, d.name, b ORDER BY m.device_id, b`
	}
	out := make([]deviceHistory, 0, len(ids))
	if len(ids) > 0 {
		rows, err := h.pool.Query(r.Context(), query, ids, from, to, width.String())
		if err != nil {
			httputil.SendInternalError(w, err)
			return
		}
		defer rows.Close()
		index := map[int64]int{}
		for rows.Next() {
			var id int64
			var name string
			var p historyPoint
			if err := rows.Scan(&id, &name, &p.T, &p.Avg, &p.Min, &p.Max, &p.Samples, &p.Down); err != nil {
				httputil.SendInternalError(w, err)
				return
			}
			i, seen := index[id]
			if !seen {
				i = len(out)
				index[id] = i
				out = append(out, deviceHistory{DeviceID: id, DeviceName: name, Points: []historyPoint{}})
			}
			out[i].Points = append(out[i].Points, p)
		}
		if err := rows.Err(); err != nil {
			httputil.SendInternalError(w, err)
			return
		}
	}
	httputil.SendOK(w, out)
}

func parseIDList(s string) ([]int64, error) {
	var ids []int64
	seen := map[int64]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid id %q", part)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ── Alerts workbench ─────────────────────────────────────────────────────

func callerName(r *http.Request) string {
	if c := auth.GetClaims(r.Context()); c != nil {
		return c.Username
	}
	return ""
}

func hasPermission(r *http.Request, perm string) bool {
	c := auth.GetClaims(r.Context())
	return c != nil && (rbac.IsUnscopedRole(c.Role) || rbac.HasPermission(c.Permissions, perm))
}

// broadcastAlert re-reads an alert and publishes it as alert:updated.
func (h *OperatorHandler) broadcastAlert(ctx context.Context, id int64) *models.Alert {
	a, err := h.db.GetAlert(ctx, id)
	if err != nil || a == nil {
		return nil
	}
	if h.hub != nil {
		h.hub.Broadcast(websocket.Message{Type: websocket.EventAlertUpdated, Data: a})
	}
	return a
}

type bulkSkip struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// Bulk acknowledges or resolves many alerts. Each alert is checked against
// the caller's scope and its current state; failures are reported per id.
func (h *OperatorHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
		Note   string  `json:"note"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > maxBulkAlerts {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("ids must contain 1-%d alert ids", maxBulkAlerts))
		return
	}
	if len(body.Note) > maxNoteLen {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("note must be at most %d characters", maxNoteLen))
		return
	}
	var target string
	switch body.Action {
	case "acknowledge":
		target = "acknowledged"
		if !hasPermission(r, models.PermAlertsAcknowledge) {
			httputil.SendError(w, http.StatusForbidden, "missing permission: "+models.PermAlertsAcknowledge)
			return
		}
	case "resolve":
		target = "resolved"
		if !hasPermission(r, models.PermAlertsResolve) {
			httputil.SendError(w, http.StatusForbidden, "missing permission: "+models.PermAlertsResolve)
			return
		}
	default:
		httputil.SendError(w, http.StatusBadRequest, "action must be acknowledge or resolve")
		return
	}

	by := callerName(r)
	updated := []int64{}
	skipped := []bulkSkip{}
	seen := map[int64]bool{}
	for _, id := range body.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		a, err := h.db.GetAlert(r.Context(), id)
		if err != nil || a == nil {
			skipped = append(skipped, bulkSkip{id, "not_found"})
			continue
		}
		if isScoped(r) {
			device, err := h.db.GetDevice(r.Context(), a.DeviceID)
			if err != nil || !canAccessAlert(r, a, device) {
				// Same answer as a missing alert: don't confirm it exists.
				skipped = append(skipped, bulkSkip{id, "not_found"})
				continue
			}
		}
		if a.Status == "resolved" || a.Status == target {
			skipped = append(skipped, bulkSkip{id, "invalid_state"})
			continue
		}
		if err := h.db.UpdateAlertStatus(r.Context(), id, target, by); err != nil {
			httputil.SendInternalError(w, err)
			return
		}
		if body.Note != "" {
			if _, err := h.insertNote(r.Context(), id, by, body.Note); err != nil {
				httputil.SendInternalError(w, err)
				return
			}
		}
		updated = append(updated, id)
		h.broadcastAlert(r.Context(), id)
	}
	httputil.SendOK(w, map[string]any{"updated": updated, "skipped": skipped})
}

func (h *OperatorHandler) insertNote(ctx context.Context, alertID int64, author, text string) (*models.AlertNote, error) {
	n := models.AlertNote{AlertID: alertID, Author: author, Text: text}
	err := h.pool.QueryRow(ctx,
		`INSERT INTO alert_notes(alert_id, author, text) VALUES ($1,$2,$3) RETURNING id, created_at`,
		alertID, author, text).Scan(&n.ID, &n.CreatedAt)
	return &n, err
}

// Notes lists an alert's notes, oldest first. AlertScopeGuard has already
// checked the alert is visible to the caller.
func (h *OperatorHandler) Notes(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, alert_id, text, author, created_at FROM alert_notes WHERE alert_id=$1 ORDER BY created_at, id`, id)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	defer rows.Close()
	notes := []models.AlertNote{}
	for rows.Next() {
		var n models.AlertNote
		if err := rows.Scan(&n.ID, &n.AlertID, &n.Text, &n.Author, &n.CreatedAt); err != nil {
			httputil.SendInternalError(w, err)
			return
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, notes)
}

// AddNote appends a note to an alert.
func (h *OperatorHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.Text = strings.TrimSpace(body.Text)
	if body.Text == "" || len(body.Text) > maxNoteLen {
		httputil.SendError(w, http.StatusBadRequest, fmt.Sprintf("text must be 1-%d characters", maxNoteLen))
		return
	}
	n, err := h.insertNote(r.Context(), id, callerName(r), body.Text)
	if err != nil {
		httputil.SendDBError(w, err)
		return
	}
	httputil.SendCreated(w, n)
}

// SetAssignee assigns an alert to a user who can acknowledge alerts, or
// clears the assignment with userId null.
func (h *OperatorHandler) SetAssignee(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(r, "id")
	if !ok {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		UserID *int64 `json:"userId"`
	}
	if err := httputil.ParseJSON(r, &body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.UserID != nil {
		assignable, err := h.assignableUserIDs(r.Context())
		if err != nil {
			httputil.SendInternalError(w, err)
			return
		}
		if !assignable[*body.UserID] {
			httputil.SendError(w, http.StatusBadRequest, "user cannot be assigned alerts")
			return
		}
	}
	tag, err := h.pool.Exec(r.Context(), `UPDATE alerts SET assigned_to=$2 WHERE id=$1`, id, body.UserID)
	if err != nil {
		httputil.SendDBError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		notFound(w)
		return
	}
	httputil.SendOK(w, h.broadcastAlert(r.Context(), id))
}

type assignableUser struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// assignableQuery selects enabled users whose role grants alerts.acknowledge,
// either through a custom role or a built-in role that implies it.
const assignableQuery = `
	SELECT u.id, u.username, COALESCE(NULLIF(u.display_name,''), u.username)
	FROM users u LEFT JOIN roles ro ON ro.id = u.role_id
	WHERE u.enabled AND (
		u.role IN ('super_admin','admin','network_admin')
		OR (ro.permissions IS NOT NULL AND ro.permissions ? 'alerts.acknowledge'))
	ORDER BY 3`

func (h *OperatorHandler) assignableUserIDs(ctx context.Context) (map[int64]bool, error) {
	users, err := h.assignableUsers(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[int64]bool, len(users))
	for _, u := range users {
		ids[u.ID] = true
	}
	return ids, nil
}

func (h *OperatorHandler) assignableUsers(ctx context.Context) ([]assignableUser, error) {
	rows, err := h.pool.Query(ctx, assignableQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []assignableUser{}
	for rows.Next() {
		var u assignableUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// AssignableUsers lists users who can be assigned alerts.
func (h *OperatorHandler) AssignableUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.assignableUsers(r.Context())
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, users)
}

// ── Search ───────────────────────────────────────────────────────────────

type searchDevice struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	IPAddress    string  `json:"ipAddress"`
	Status       string  `json:"status"`
	LocationName *string `json:"locationName,omitempty"`
}

type searchLocation struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type searchAlert struct {
	ID         int64     `json:"id"`
	Message    string    `json:"message"`
	Severity   string    `json:"severity"`
	DeviceName string    `json:"deviceName,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Search finds devices, locations and active alerts for the command palette.
func (h *OperatorHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		httputil.SendError(w, http.StatusBadRequest, "q must be at least 2 characters")
		return
	}
	limit := httputil.QueryParamInt(r, "limit", 8, 1, 25)
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.ToLower(q)) + "%"

	// Unscoped callers get no clause at all (an IN over devices would also
	// drop alerts without a device).
	devClause, alertClause, scopeArgs := "", "", []any(nil)
	if sf := scopeFilterFromContext(r); sf != nil {
		var cond string
		cond, scopeArgs = database.ScopeCondition(sf, 3)
		devClause = " AND d.id IN (SELECT id FROM devices WHERE " + cond + ")"
		alertClause = " AND device_id IN (SELECT id FROM devices WHERE " + cond + ")"
	}
	ctx := r.Context()
	result := map[string]any{"devices": []searchDevice{}, "locations": []searchLocation{}, "alerts": []searchAlert{}}

	devArgs := append([]any{pattern, limit}, scopeArgs...)
	rows, err := h.pool.Query(ctx, `
		SELECT d.id, d.name, d.ip_address, d.status, l.name
		FROM devices d LEFT JOIN locations l ON l.id = d.location_id
		WHERE (LOWER(d.name) LIKE $1 OR LOWER(d.ip_address) LIKE $1 OR LOWER(COALESCE(d.mac_address,'')) LIKE $1
		       OR LOWER(COALESCE(d.serial_number,'')) LIKE $1 OR LOWER(COALESCE(d.asset_tag,'')) LIKE $1)`+devClause+`
		ORDER BY (LOWER(d.name) = LOWER($1)) DESC, d.name
		LIMIT $2`, devArgs...)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	devices, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (searchDevice, error) {
		var d searchDevice
		err := row.Scan(&d.ID, &d.Name, &d.IPAddress, &d.Status, &d.LocationName)
		return d, err
	})
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	result["devices"] = devices

	// Locations: scoped users only see the locations they are scoped to.
	locSQL := `SELECT id, name, type FROM locations WHERE LOWER(name) LIKE $1 ORDER BY name LIMIT $2`
	locArgs := []any{pattern, limit}
	if sf := scopeFilterFromContext(r); sf != nil {
		locSQL = `SELECT id, name, type FROM locations WHERE LOWER(name) LIKE $1 AND id::text = ANY($3) ORDER BY name LIMIT $2`
		locArgs = append(locArgs, sf.LocationIDs)
	}
	rows, err = h.pool.Query(ctx, locSQL, locArgs...)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	locations, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (searchLocation, error) {
		var l searchLocation
		err := row.Scan(&l.ID, &l.Name, &l.Type)
		return l, err
	})
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	result["locations"] = locations

	alertArgs := append([]any{pattern, limit}, scopeArgs...)
	rows, err = h.pool.Query(ctx, `
		SELECT id, message, severity, COALESCE(device_name,''), created_at FROM alerts
		WHERE status <> 'resolved' AND (LOWER(message) LIKE $1 OR LOWER(COALESCE(device_name,'')) LIKE $1)`+alertClause+`
		ORDER BY created_at DESC LIMIT $2`, alertArgs...)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	alerts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (searchAlert, error) {
		var a searchAlert
		err := row.Scan(&a.ID, &a.Message, &a.Severity, &a.DeviceName, &a.CreatedAt)
		return a, err
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httputil.SendInternalError(w, err)
		return
	}
	result["alerts"] = alerts
	httputil.SendOK(w, result)
}
