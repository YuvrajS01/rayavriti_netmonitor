package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type AlertRuleHandler struct {
	db database.Database
}

func NewAlertRuleHandler(db database.Database) *AlertRuleHandler {
	return &AlertRuleHandler{db: db}
}

func (h *AlertRuleHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.db.GetAlertRules(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if rules == nil {
		rules = []models.AlertRule{}
	}
	httputil.SendOK(w, rules)
}

func (h *AlertRuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	rule, err := h.db.GetAlertRule(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert rule not found")
		return
	}
	httputil.SendOK(w, rule)
}

func (h *AlertRuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rule models.AlertRule
	if err := httputil.ParseJSON(r, &rule); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if rule.Name == "" {
		httputil.SendError(w, 400, "name is required")
		return
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.ConditionLogic == "" {
		rule.ConditionLogic = "all"
	}
	rule.Enabled = true
	if rule.CooldownSec == 0 {
		rule.CooldownSec = 300
	}
	if rule.ScopeType == "" {
		rule.ScopeType = "global"
	}
	claims := auth.GetClaims(r.Context())
	if claims != nil {
		rule.CreatedBy = &claims.UserID
	}
	created, err := h.db.CreateAlertRule(r.Context(), &rule)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *AlertRuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if _, err := h.db.GetAlertRule(r.Context(), id); err != nil {
		httputil.SendError(w, 404, "alert rule not found")
		return
	}
	var rule models.AlertRule
	if err := httputil.ParseJSON(r, &rule); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	updated, err := h.db.UpdateAlertRule(r.Context(), id, &rule)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, updated)
}

func (h *AlertRuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteAlertRule(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}

func (h *AlertRuleHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	rule, err := h.db.GetAlertRule(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert rule not found")
		return
	}
	newEnabled := !rule.Enabled
	if err := h.db.ToggleAlertRule(r.Context(), id, newEnabled); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]any{"id": id, "enabled": newEnabled})
}

// Test performs a dry-run evaluation of the rule against the latest metrics
// for each device the rule applies to. No alerts are created.
func (h *AlertRuleHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	rule, err := h.db.GetAlertRule(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "alert rule not found")
		return
	}

	devices, err := h.db.GetEnabledDevices(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}

	latestMetrics, err := h.db.GetLatestMetrics(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	metricByDevice := make(map[int64]models.Metric)
	for _, m := range latestMetrics {
		metricByDevice[m.DeviceID] = m
	}

	type deviceResult struct {
		DeviceID   int64                    `json:"deviceId"`
		DeviceName string                   `json:"deviceName"`
		Matched    bool                     `json:"matched"`
		Conditions []engine.ConditionResult `json:"conditions"`
	}

	var results []deviceResult
	for _, device := range devices {
		if !engine.RuleAppliesToDevice(rule, &device) {
			continue
		}
		metric, ok := metricByDevice[device.ID]
		if !ok {
			continue
		}
		var condResults []engine.ConditionResult
		conditionsMet := 0
		for _, cond := range rule.Conditions {
			cr := engine.EvaluateCondition(cond, &metric, "")
			condResults = append(condResults, cr)
			if cr.Result {
				conditionsMet++
			}
		}
		matched := false
		if rule.ConditionLogic == "all" {
			matched = conditionsMet == len(rule.Conditions)
		} else {
			matched = conditionsMet > 0
		}
		results = append(results, deviceResult{
			DeviceID:   device.ID,
			DeviceName: device.Name,
			Matched:    matched,
			Conditions: condResults,
		})
	}

	httputil.SendOK(w, map[string]any{
		"ruleId":     rule.ID,
		"ruleName":   rule.Name,
		"severity":   rule.Severity,
		"conditions": rule.Conditions,
		"dryRun":     true,
		"results":    results,
	})
}
