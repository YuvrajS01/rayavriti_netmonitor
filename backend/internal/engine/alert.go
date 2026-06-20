package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

type AlertEngine struct {
	db         database.Database
	hub        *websocket.Hub
	notifier   *Notifier
	stateCache *cache.AlertStateCache

	suppressionChecker SuppressionChecker
	maintenanceChecker MaintenanceChecker
	suppressedRecorder SuppressedAlertRecorder

	baselineCache *BaselineCache
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewAlertEngine(db database.Database, hub *websocket.Hub, notifier *Notifier, opts ...AlertEngineOption) *AlertEngine {
	e := &AlertEngine{
		db:            db,
		hub:           hub,
		notifier:      notifier,
		baselineCache: NewBaselineCache(15 * time.Minute),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type AlertEngineOption func(*AlertEngine)

func WithAlertStateCache(sc *cache.AlertStateCache) AlertEngineOption {
	return func(e *AlertEngine) { e.stateCache = sc }
}

func WithSuppressionChecker(sc SuppressionChecker) AlertEngineOption {
	return func(e *AlertEngine) { e.suppressionChecker = sc }
}

func WithMaintenanceChecker(mc MaintenanceChecker) AlertEngineOption {
	return func(e *AlertEngine) { e.maintenanceChecker = mc }
}

func WithSuppressedAlertRecorder(sr SuppressedAlertRecorder) AlertEngineOption {
	return func(e *AlertEngine) { e.suppressedRecorder = sr }
}

func (e *AlertEngine) ProcessMetric(ctx context.Context, device *models.Device, metric *models.Metric, previousStatus string) error {
	rules, err := e.db.GetAlertRules(ctx)
	if err != nil {
		return fmt.Errorf("load alert rules: %w", err)
	}

	for i := range rules {
		rule := &rules[i]
		if !rule.Enabled {
			continue
		}
		if len(rule.Conditions) == 0 {
			continue
		}
		if !ruleAppliesToDevice(rule, device) {
			continue
		}
		e.evaluateRule(ctx, rule, device, metric, previousStatus)
	}
	return nil
}

func (e *AlertEngine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go e.absenceLoop(ctx)
	slog.Info("Alert engine started")
}

func (e *AlertEngine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
	slog.Info("Alert engine stopped")
}

func (e *AlertEngine) ReloadRules(_ context.Context) error {
	return nil
}

// ── absence background loop ──────────────────────────────────────────────────

func (e *AlertEngine) absenceLoop(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAbsenceConditions(ctx)
		}
	}
}

func (e *AlertEngine) evaluateAbsenceConditions(ctx context.Context) {
	rules, err := e.db.GetAlertRules(ctx)
	if err != nil {
		slog.Warn("Failed to load rules for absence check", "error", err)
		return
	}

	devices, err := e.db.GetEnabledDevices(ctx)
	if err != nil {
		slog.Warn("Failed to load devices for absence check", "error", err)
		return
	}

	for i := range rules {
		rule := &rules[i]
		if !rule.Enabled {
			continue
		}
		for _, cond := range rule.Conditions {
			if cond.Type != "absence" {
				continue
			}
			for j := range devices {
				device := &devices[j]
				if !ruleAppliesToDevice(rule, device) {
					continue
				}
				e.checkAbsence(ctx, rule, device, cond)
			}
		}
	}
}

func (e *AlertEngine) checkAbsence(ctx context.Context, rule *models.AlertRule, device *models.Device, condition models.AlertRuleCondition) {
	latest, err := e.db.GetLatestMetricForDevice(ctx, device.ID)
	if err != nil {
		return
	}

	metric := &models.Metric{
		DeviceID:  device.ID,
		Timestamp: time.Now().Add(-24 * time.Hour),
		Status:    "unknown",
	}
	if latest != nil {
		metric.Timestamp = latest.Timestamp
		metric.Status = latest.Status
	}

	cr := EvaluateCondition(condition, metric, "", nil)
	if !cr.Result {
		return
	}

	if existing := e.findActiveAlertForRule(ctx, rule.ID, device.ID); existing != nil {
		return
	}

	if e.checkSuppressionForDevice(ctx, device, rule) {
		return
	}

	alertMsg := fmt.Sprintf("No data received from %s for %.0fs", device.Name, cr.ActualValue)
	alert := &models.Alert{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Severity:   rule.Severity,
		Message:    alertMsg,
		Status:     "active",
		RuleID:     &rule.ID,
	}

	created, err := e.db.CreateAlert(ctx, alert)
	if err != nil {
		slog.Warn("Failed to create absence alert", "rule_id", rule.ID, "device_id", device.ID, "error", err)
		return
	}

	e.recordHistory(ctx, created.ID, rule.ID, "fired", "system:alert_engine", map[string]any{
		"trigger_reason": "absence detected",
		"description":    cr.Description,
	})

	slog.Info("Absence alert fired",
		"alert_id", created.ID, "rule_id", rule.ID,
		"device_id", device.ID, "device_name", device.Name,
	)

	if e.hub != nil {
		e.hub.Broadcast(websocket.Message{
			Type: websocket.EventAlertTriggered,
			Data: created,
		})
	}

	e.sendNotifications(ctx, rule, created)
}

// ── rule matching ────────────────────────────────────────────────────────────

func RuleAppliesToDevice(rule *models.AlertRule, device *models.Device) bool {
	return ruleAppliesToDevice(rule, device)
}

func ruleAppliesToDevice(rule *models.AlertRule, device *models.Device) bool {
	switch rule.ScopeType {
	case "device":
		return rule.DeviceID != nil && *rule.DeviceID == device.ID
	case "global", "":
		return true
	default:
		return true
	}
}

// ── core evaluation loop ─────────────────────────────────────────────────────

func (e *AlertEngine) evaluateRule(ctx context.Context, rule *models.AlertRule, device *models.Device, metric *models.Metric, previousStatus string) {
	conditionsMet := 0
	results := make([]ConditionResult, 0, len(rule.Conditions))
	for _, cond := range rule.Conditions {
		var baseline *AnomalyBaseline
		if cond.Type == "anomaly" {
			baseline = e.baselineCache.Get(device.ID, cond.MetricField)
		}
		cr := EvaluateCondition(cond, metric, previousStatus, baseline)
		results = append(results, cr)
		if cr.Result {
			conditionsMet++
		}
	}

	ruleTriggered := false
	if rule.ConditionLogic == "all" {
		ruleTriggered = conditionsMet == len(rule.Conditions)
	} else {
		ruleTriggered = conditionsMet > 0
	}

	var state *models.AlertRuleState
	var err error
	if e.stateCache != nil {
		state, err = e.stateCache.GetAlertRuleState(ctx, rule.ID, device.ID)
	} else {
		state, err = e.db.GetAlertRuleState(ctx, rule.ID, device.ID)
	}
	if err != nil {
		state = &models.AlertRuleState{
			RuleID:   rule.ID,
			DeviceID: device.ID,
			State:    "idle",
		}
	}

	now := time.Now()

	if ruleTriggered {
		e.handleConditionMet(ctx, rule, device, metric, state, now, results)
	} else {
		e.handleConditionCleared(ctx, rule, device, state, now)
	}
}

// ── state transitions ────────────────────────────────────────────────────────

func (e *AlertEngine) handleConditionMet(
	ctx context.Context,
	rule *models.AlertRule,
	device *models.Device,
	metric *models.Metric,
	state *models.AlertRuleState,
	now time.Time,
	results []ConditionResult,
) {
	sustainedDuration := rule.CooldownSec
	for _, cond := range rule.Conditions {
		if cond.DurationSeconds > 0 {
			sustainedDuration = cond.DurationSeconds
			break
		}
	}

	switch state.State {
	case "idle":
		firstMet := now
		e.upsertState(ctx, &models.AlertRuleState{
			RuleID:            rule.ID,
			DeviceID:          device.ID,
			State:             "pending",
			FirstMetAt:        &firstMet,
			LastEvaluatedAt:   &now,
			ConditionSnapshot: snapshotFromResults(results),
		})

	case "pending":
		if state.FirstMetAt != nil {
			elapsed := now.Sub(*state.FirstMetAt).Seconds()
			if elapsed >= float64(sustainedDuration) {
				e.fireAlert(ctx, rule, device, state, now, results, int(elapsed))
				return
			}
		}
		state.LastEvaluatedAt = &now
		state.ConditionSnapshot = snapshotFromResults(results)
		e.upsertState(ctx, state)

	case "resolved":
		if state.LastResolvedAt != nil {
			cooldownLeft := float64(rule.CooldownSec) - now.Sub(*state.LastResolvedAt).Seconds()
			if cooldownLeft > 0 {
				state.LastEvaluatedAt = &now
				e.upsertState(ctx, state)
				return
			}
		}
		e.fireAlert(ctx, rule, device, state, now, results, 0)

	case "firing", "notified", "acknowledged":
		state.LastEvaluatedAt = &now
		e.upsertState(ctx, state)
	}
}

func (e *AlertEngine) handleConditionCleared(
	ctx context.Context,
	rule *models.AlertRule,
	device *models.Device,
	state *models.AlertRuleState,
	now time.Time,
) {
	switch state.State {
	case "pending":
		e.upsertState(ctx, &models.AlertRuleState{
			RuleID:          rule.ID,
			DeviceID:        device.ID,
			State:           "idle",
			LastEvaluatedAt: &now,
		})

	case "firing", "notified", "acknowledged":
		if !rule.AutoResolve {
			state.LastEvaluatedAt = &now
			e.upsertState(ctx, state)
			return
		}
		if state.ActiveAlertID != nil {
			if err := e.db.UpdateAlertStatus(ctx, *state.ActiveAlertID, "resolved", "system:alert_engine"); err != nil {
				slog.Warn("Failed to auto-resolve alert", "alert_id", *state.ActiveAlertID, "error", err)
			} else {
				e.recordHistory(ctx, *state.ActiveAlertID, rule.ID, "auto_resolved", "system:alert_engine", map[string]any{
					"reason": "all_conditions_cleared",
				})
				if e.hub != nil {
					e.hub.Broadcast(websocket.Message{
						Type: websocket.EventAlertResolved,
						Data: map[string]any{
							"alert_id":    *state.ActiveAlertID,
							"device_id":   device.ID,
							"device_name": device.Name,
							"resolved_by": "system:alert_engine",
						},
					})
				}
				slog.Info("Alert auto-resolved",
					"alert_id", *state.ActiveAlertID,
					"rule_id", rule.ID,
					"device_id", device.ID,
					"device_name", device.Name,
				)
			}
		}
		now2 := now
		e.upsertState(ctx, &models.AlertRuleState{
			RuleID:          rule.ID,
			DeviceID:        device.ID,
			State:           "resolved",
			LastEvaluatedAt: &now,
			LastResolvedAt:  &now2,
		})
	}
}

// ── alert creation ───────────────────────────────────────────────────────────

func (e *AlertEngine) fireAlert(
	ctx context.Context,
	rule *models.AlertRule,
	device *models.Device,
	state *models.AlertRuleState,
	now time.Time,
	results []ConditionResult,
	sustainedSeconds int,
) {
	if existing := e.findActiveAlertForRule(ctx, rule.ID, device.ID); existing != nil {
		state.LastFiredAt = &now
		state.ActiveAlertID = &existing.ID
		state.State = "firing"
		e.upsertState(ctx, state)
		return
	}

	if e.checkSuppressionForDevice(ctx, device, rule) {
		state.LastEvaluatedAt = &now
		state.State = "idle"
		e.upsertState(ctx, state)
		return
	}

	alertMsg := buildAlertMessage(rule, device, results)
	groupID := fmt.Sprintf("%d-%d", rule.ID, now.Unix()/60)

	alert := &models.Alert{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Severity:   rule.Severity,
		Message:    alertMsg,
		Status:     "active",
		RuleID:     &rule.ID,
		GroupID:    &groupID,
	}

	created, err := e.db.CreateAlert(ctx, alert)
	if err != nil {
		slog.Warn("Failed to create alert",
			"rule_id", rule.ID, "device_id", device.ID, "error", err)
		return
	}

	e.recordHistory(ctx, created.ID, rule.ID, "fired", "system:alert_engine", map[string]any{
		"trigger_reason":    fmt.Sprintf("conditions sustained for %ds", sustainedSeconds),
		"condition_results": results,
	})

	slog.Info("Alert fired",
		"alert_id", created.ID,
		"rule_id", rule.ID,
		"rule_name", rule.Name,
		"device_id", device.ID,
		"device_name", device.Name,
		"severity", rule.Severity,
		"sustained_seconds", sustainedSeconds,
	)

	if e.hub != nil {
		e.hub.Broadcast(websocket.Message{
			Type: websocket.EventAlertTriggered,
			Data: created,
		})
	}

	e.sendNotifications(ctx, rule, created)

	now2 := now
	e.upsertState(ctx, &models.AlertRuleState{
		RuleID:            rule.ID,
		DeviceID:          device.ID,
		State:             "notified",
		LastEvaluatedAt:   &now,
		FirstMetAt:        state.FirstMetAt,
		LastFiredAt:       &now2,
		ActiveAlertID:     &created.ID,
		ConditionSnapshot: snapshotFromResults(results),
	})
}

func buildAlertMessage(rule *models.AlertRule, device *models.Device, results []ConditionResult) string {
	for _, r := range results {
		if r.Result && r.Description != "" {
			return fmt.Sprintf("%s on %s: %s", rule.Name, device.Name, r.Description)
		}
	}
	return fmt.Sprintf("Alert rule '%s' triggered for %s", rule.Name, device.Name)
}

// ── notifications ────────────────────────────────────────────────────────────

func (e *AlertEngine) sendNotifications(ctx context.Context, rule *models.AlertRule, alert *models.Alert) {
	if e.notifier == nil || len(rule.ChannelIDs) == 0 {
		return
	}

	channels, err := e.db.GetNotificationChannels(ctx)
	if err != nil {
		slog.Warn("Failed to load notification channels", "error", err)
		return
	}

	channelMap := make(map[int64]models.NotificationChannel, len(channels))
	for _, ch := range channels {
		channelMap[ch.ID] = ch
	}

	for _, chID := range rule.ChannelIDs {
		ch, ok := channelMap[chID]
		if !ok || !ch.Enabled {
			continue
		}

		start := time.Now()
		err := e.notifier.Send(ctx, ch, alert)
		duration := time.Since(start)

		if err != nil {
			slog.Warn("Notification delivery failed",
				"channel_id", ch.ID, "channel_type", ch.Type,
				"channel_name", ch.Name, "alert_id", alert.ID,
				"error", err, "duration_ms", duration.Milliseconds(),
			)
			e.recordHistory(ctx, alert.ID, rule.ID, "notification_failed",
				fmt.Sprintf("channel:%s", ch.Name), map[string]any{
					"channel_id":   ch.ID,
					"channel_type": ch.Type,
					"error":        err.Error(),
				})
		} else {
			slog.Debug("Notification delivered",
				"channel_id", ch.ID, "channel_type", ch.Type,
				"channel_name", ch.Name, "alert_id", alert.ID,
				"duration_ms", duration.Milliseconds(),
			)
			e.recordHistory(ctx, alert.ID, rule.ID, "notified",
				fmt.Sprintf("channel:%s", ch.Name), map[string]any{
					"channel_id":   ch.ID,
					"channel_type": ch.Type,
					"channel_name": ch.Name,
				})
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (e *AlertEngine) findActiveAlertForRule(ctx context.Context, ruleID, deviceID int64) *models.Alert {
	alert, err := e.db.FindActiveAlertByRuleAndDevice(ctx, ruleID, deviceID)
	if err != nil {
		return nil
	}
	return alert
}

func (e *AlertEngine) recordHistory(ctx context.Context, alertID, ruleID int64, action, actor string, details map[string]any) {
	h := &models.AlertHistory{
		AlertID: alertID,
		RuleID:  &ruleID,
		Action:  action,
		Actor:   actor,
		Details: details,
	}
	if err := e.db.RecordAlertHistory(ctx, h); err != nil {
		slog.Warn("Failed to record alert history",
			"alert_id", alertID, "action", action, "error", err)
	}
}

func (e *AlertEngine) upsertState(ctx context.Context, s *models.AlertRuleState) {
	var err error
	if e.stateCache != nil {
		err = e.stateCache.UpsertAlertRuleState(ctx, s)
	} else {
		err = e.db.UpsertAlertRuleState(ctx, s)
	}
	if err != nil {
		slog.Warn("Failed to persist alert rule state",
			"rule_id", s.RuleID, "device_id", s.DeviceID,
			"state", s.State, "error", err)
	}
}

func snapshotFromResults(results []ConditionResult) map[string]any {
	return map[string]any{"results": results}
}
