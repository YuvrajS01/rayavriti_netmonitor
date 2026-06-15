package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AlertHandler struct{ db database.Database }

func NewAlertHandler(db database.Database) *AlertHandler { return &AlertHandler{db: db} }

func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	alerts, total, err := h.db.GetAlerts(r.Context(), status, limit, offset)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{"alerts": alerts, "total": total})
}

func (h *AlertHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	a, err := h.db.GetAlert(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert not found")
		return
	}
	httputil.SendOK(w, a)
}

func (h *AlertHandler) Create(w http.ResponseWriter, r *http.Request) {
	var a models.Alert
	if err := httputil.ParseJSON(r, &a); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if a.Status == "" {
		a.Status = "active"
	}
	created, err := h.db.CreateAlert(r.Context(), &a)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *AlertHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	existing, err := h.db.GetAlert(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert not found")
		return
	}
	var payload struct {
		Severity *string `json:"severity"`
		Message  *string `json:"message"`
		Status   *string `json:"status"`
		Comment  *string `json:"comment"`
	}
	if err := httputil.ParseJSON(r, &payload); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if payload.Severity != nil {
		existing.Severity = *payload.Severity
	}
	if payload.Message != nil {
		existing.Message = *payload.Message
	}
	if payload.Status != nil {
		claims := auth.GetClaims(r.Context())
		by := ""
		if claims != nil {
			by = claims.Username
		}
		status := *payload.Status
		// Normalize "triggered" → "active" for compatibility
		if status == "triggered" {
			status = "active"
		}
		if err := h.db.UpdateAlertStatus(r.Context(), id, status, by); err != nil {
			httputil.SendError(w, 500, err.Error())
			return
		}
	}
	updated, err := h.db.GetAlert(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert not found")
		return
	}
	httputil.SendOK(w, updated)
}

func (h *AlertHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	by := ""
	if claims != nil {
		by = claims.Username
	}
	if err := h.db.UpdateAlertStatus(r.Context(), id, "acknowledged", by); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"acknowledged": true})
}

func (h *AlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	claims := auth.GetClaims(r.Context())
	by := ""
	if claims != nil {
		by = claims.Username
	}
	if err := h.db.UpdateAlertStatus(r.Context(), id, "resolved", by); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"resolved": true})
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteAlert(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

func (h *AlertHandler) Counts(w http.ResponseWriter, r *http.Request) {
	counts, err := h.db.GetAlertCounts(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, counts)
}

func (h *AlertHandler) History(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	history, err := h.db.GetAlertHistory(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if history == nil {
		history = []models.AlertHistory{}
	}
	httputil.SendOK(w, history)
}

func (h *AlertHandler) Grouped(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "active"
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 300
	}
	alerts, _, err := h.db.GetAlerts(r.Context(), status, limit, 0)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	type AlertGroup struct {
		GroupID string         `json:"groupId"`
		RuleID  *int64         `json:"ruleId,omitempty"`
		Count   int            `json:"count"`
		Alerts  []models.Alert `json:"alerts"`
	}

	groupMap := make(map[string]*AlertGroup)
	var groupOrder []string

	for _, a := range alerts {
		gid := ""
		if a.GroupID != nil {
			gid = *a.GroupID
		}
		if gid == "" {
			gid = fmt.Sprintf("ungrouped-%d", a.ID)
		}

		if _, exists := groupMap[gid]; !exists {
			groupMap[gid] = &AlertGroup{
				GroupID: gid,
				RuleID:  a.RuleID,
			}
			groupOrder = append(groupOrder, gid)
		}
		groupMap[gid].Alerts = append(groupMap[gid].Alerts, a)
		groupMap[gid].Count = len(groupMap[gid].Alerts)
	}

	groups := make([]AlertGroup, 0, len(groupOrder))
	for _, gid := range groupOrder {
		groups = append(groups, *groupMap[gid])
	}

	httputil.SendOK(w, groups)
}

// AlertStats returns alert engine statistics: counts by status, recent activity, and rule breakdown.
func (h *AlertHandler) AlertStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.db.GetAlertCounts(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	rules, err := h.db.GetAlertRules(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	rulesSummary := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		rulesSummary = append(rulesSummary, map[string]any{
			"id":       rule.ID,
			"name":     rule.Name,
			"severity": rule.Severity,
			"enabled":  rule.Enabled,
		})
	}

	channels, err := h.db.GetNotificationChannels(r.Context())
	if err != nil {
		channels = []models.NotificationChannel{}
	}
	channelsSummary := make([]map[string]any, 0, len(channels))
	for _, ch := range channels {
		channelsSummary = append(channelsSummary, map[string]any{
			"id":      ch.ID,
			"name":    ch.Name,
			"type":    ch.Type,
			"enabled": ch.Enabled,
		})
	}

	httputil.SendOK(w, map[string]any{
		"alertCounts":   counts,
		"rules":         rulesSummary,
		"channels":      channelsSummary,
		"totalRules":    len(rules),
		"totalChannels": len(channels),
	})
}
