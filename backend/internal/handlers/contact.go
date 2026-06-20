package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ContactHandler provides typed HTTP endpoints for contacts, escalation, and notifications.
type ContactHandler struct {
	resolver   *engine.ContactResolver
	escalation *engine.EscalationEngine
	pool       *pgxpool.Pool
}

// NewContactHandler creates a ContactHandler wired to the Postgres pool.
func NewContactHandler(db database.Database) *ContactHandler {
	pg, ok := db.(*database.Postgres)
	if !ok {
		slog.Warn("ContactHandler: database is not *Postgres, contact features will be unavailable")
		return &ContactHandler{}
	}
	pool := pg.Pool()
	resolver := engine.NewContactResolver(pool, db)
	notifier := engine.NewNotifier()
	escalation := engine.NewEscalationEngine(pool, resolver, notifier, nil)
	return &ContactHandler{
		resolver:   resolver,
		escalation: escalation,
		pool:       pool,
	}
}

// ResolveContacts returns the contacts that should be notified for a given device.
func (h *ContactHandler) ResolveContacts(w http.ResponseWriter, r *http.Request) {
	if h.resolver == nil {
		httputil.SendError(w, http.StatusNotImplemented, "contact service unavailable")
		return
	}
	deviceID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	severity := r.URL.Query().Get("severity")
	if severity == "" {
		severity = "warning"
	}

	contacts, err := h.resolver.ResolveForDevice(r.Context(), deviceID, nil, severity)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, contacts)
}

// EscalationStart begins multi-step escalation for an alert.
func (h *ContactHandler) EscalationStart(w http.ResponseWriter, r *http.Request) {
	if h.escalation == nil {
		httputil.SendError(w, http.StatusNotImplemented, "escalation service unavailable")
		return
	}
	var body struct {
		AlertID  int64 `json:"alertId"`
		PolicyID int64 `json:"policyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.AlertID == 0 || body.PolicyID == 0 {
		httputil.SendError(w, http.StatusBadRequest, "alertId and policyId are required")
		return
	}

	// Fetch alert
	var alert models.Alert
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, device_id, COALESCE(device_name,''), COALESCE(severity,'warning'),
			COALESCE(message,''), COALESCE(status,'active')
		FROM alerts WHERE id = $1`,
		body.AlertID,
	).Scan(&alert.ID, &alert.DeviceID, &alert.DeviceName, &alert.Severity, &alert.Message, &alert.Status)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "alert not found")
		return
	}

	if err := h.escalation.StartEscalation(r.Context(), &alert, body.PolicyID); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.SendOK(w, map[string]any{"status": "escalation_started", "alertId": body.AlertID})
}

// EscalationCancel stops escalation for an alert.
func (h *ContactHandler) EscalationCancel(w http.ResponseWriter, r *http.Request) {
	if h.escalation == nil {
		httputil.SendError(w, http.StatusNotImplemented, "escalation service unavailable")
		return
	}
	alertID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid alert id")
		return
	}
	h.escalation.CancelEscalation(alertID)
	httputil.SendOK(w, map[string]any{"cancelled": true})
}

// EscalationStatus returns the current escalation step for an alert.
func (h *ContactHandler) EscalationStatus(w http.ResponseWriter, r *http.Request) {
	if h.escalation == nil {
		httputil.SendError(w, http.StatusNotImplemented, "escalation service unavailable")
		return
	}
	alertID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid alert id")
		return
	}
	step := h.escalation.GetActiveStep(alertID)
	httputil.SendOK(w, map[string]any{
		"alertId": alertID,
		"step":    step,
		"active":  step >= 0,
	})
}

// NotificationLog returns the notification log with optional filters.
func (h *ContactHandler) NotificationLog(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "contact service unavailable")
		return
	}

	query := `SELECT id, alert_id, contact_id, channel_type, recipient,
		COALESCE(message_preview,''), status, COALESCE(error_message,''),
		attempt_count, escalation_step, sent_at, created_at
	FROM notification_log WHERE 1=1`
	args := []any{}
	argIdx := 1

	if alertID := r.URL.Query().Get("alert_id"); alertID != "" {
		query += " AND alert_id = $" + strconv.Itoa(argIdx)
		if v, err := strconv.ParseInt(alertID, 10, 64); err == nil {
			args = append(args, v)
			argIdx++
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query += " AND status = $" + strconv.Itoa(argIdx)
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type logEntry struct {
		ID             int64   `json:"id"`
		AlertID        *int64  `json:"alertId,omitempty"`
		ContactID      int64   `json:"contactId"`
		ChannelType    string  `json:"channelType"`
		Recipient      string  `json:"recipient"`
		MessagePreview string  `json:"messagePreview,omitempty"`
		Status         string  `json:"status"`
		ErrorMessage   string  `json:"errorMessage,omitempty"`
		AttemptCount   int     `json:"attemptCount"`
		EscalationStep *int    `json:"escalationStep,omitempty"`
		SentAt         *string `json:"sentAt,omitempty"`
		CreatedAt      string  `json:"createdAt"`
	}

	var entries []logEntry
	for rows.Next() {
		var e logEntry
		if err := rows.Scan(
			&e.ID, &e.AlertID, &e.ContactID, &e.ChannelType, &e.Recipient,
			&e.MessagePreview, &e.Status, &e.ErrorMessage,
			&e.AttemptCount, &e.EscalationStep, &e.SentAt, &e.CreatedAt,
		); err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, entries)
}
