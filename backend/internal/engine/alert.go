package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
)

// AlertEngine is a rule-based alert evaluation engine. It loads rules from the
// database, evaluates them against incoming metrics, manages alert lifecycle
// (pending → firing → notified → resolved), and dispatches notifications.
type AlertEngine struct {
	db       database.Database
	hub      *websocket.Hub
	notifier *Notifier
}

// NewAlertEngine creates a rule-based alert engine.
func NewAlertEngine(db database.Database, hub *websocket.Hub, notifier *Notifier) *AlertEngine {
	return &AlertEngine{
		db:       db,
		hub:      hub,
		notifier: notifier,
	}
}

// ProcessMetric is called by the scheduler after each collector run. It evaluates
// every enabled rule that applies to the device against the new metric.
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

// Start begins the alert engine. Currently a no-op; a background absence
// checker can be added here in the future.
func (e *AlertEngine) Start(ctx context.Context) {
	slog.Info("Alert engine started")
}

// Stop gracefully shuts down the alert engine.
func (e *AlertEngine) Stop() {
	slog.Info("Alert engine stopped")
}

// ReloadRules is a no-op since rules are loaded fresh on each evaluation.
func (e *AlertEngine) ReloadRules(_ context.Context) error {
	return nil
}

// ── rule matching ────────────────────────────────────────────────────────────

// RuleAppliesToDevice reports whether the given rule should be evaluated for the device.
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
	// 1. Evaluate every condition against the current metric.
	conditionsMet := 0
	results := make([]ConditionResult, 0, len(rule.Conditions))
	for _, cond := range rule.Conditions {
		cr := EvaluateCondition(cond, metric, previousStatus)
		results = append(results, cr)
		if cr.Result {
			conditionsMet++
		}
	}

	// 2. Apply condition logic (all = AND, any = OR).
	ruleTriggered := false
	if rule.ConditionLogic == "all" {
		ruleTriggered = conditionsMet == len(rule.Conditions)
	} else {
		ruleTriggered = conditionsMet > 0
	}

	// 3. Get or initialise persisted rule state for this (rule, device) pair.
	state, err := e.db.GetAlertRuleState(ctx, rule.ID, device.ID)
	if err != nil {
		// Row does not exist yet — treat as idle.
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
	switch state.State {
	case "idle":
		// idle → pending: start tracking sustained duration.
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
		// Check if sustained duration has been reached.
		if state.FirstMetAt != nil {
			elapsed := now.Sub(*state.FirstMetAt).Seconds()
			if elapsed >= float64(rule.CooldownSec) {
				// pending → firing: create alert and send notifications.
				e.fireAlert(ctx, rule, device, state, now, results, int(elapsed))
				return
			}
		}
		// Still pending — update evaluation timestamp.
		state.LastEvaluatedAt = &now
		state.ConditionSnapshot = snapshotFromResults(results)
		e.upsertState(ctx, state)

	case "resolved":
		// Check if cooldown has elapsed since last resolution.
		if state.LastResolvedAt != nil {
			cooldownLeft := float64(rule.CooldownSec) - now.Sub(*state.LastResolvedAt).Seconds()
			if cooldownLeft > 0 {
				// Still in cooldown — just update evaluation timestamp.
				state.LastEvaluatedAt = &now
				e.upsertState(ctx, state)
				return
			}
		}
		// resolved → firing: re-fire after cooldown.
		e.fireAlert(ctx, rule, device, state, now, results, 0)

	case "firing", "notified", "acknowledged":
		// Already active — update evaluation timestamp.
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
		// pending → idle: sustained duration not yet reached.
		e.upsertState(ctx, &models.AlertRuleState{
			RuleID:          rule.ID,
			DeviceID:        device.ID,
			State:           "idle",
			LastEvaluatedAt: &now,
		})

	case "firing", "notified", "acknowledged":
		if !rule.AutoResolve {
			// No auto-resolve — stay in current state.
			state.LastEvaluatedAt = &now
			e.upsertState(ctx, state)
			return
		}
		// Auto-resolve the active alert.
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
							"alert_id":     *state.ActiveAlertID,
							"device_id":    device.ID,
							"device_name":  device.Name,
							"resolved_by":  "system:alert_engine",
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
		// → resolved
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
	// Idempotency check: do not create a duplicate active alert for the same rule+device.
	if existing := e.findActiveAlertForRule(ctx, rule.ID, device.ID); existing != nil {
		state.LastFiredAt = &now
		state.ActiveAlertID = &existing.ID
		state.State = "firing"
		e.upsertState(ctx, state)
		return
	}

	// Create the alert.
	alertMsg := fmt.Sprintf("Alert rule '%s' triggered for %s", rule.Name, device.Name)
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
		slog.Warn("Failed to create alert",
			"rule_id", rule.ID, "device_id", device.ID, "error", err)
		return
	}

	// Record history.
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

	// Broadcast via WebSocket.
	if e.hub != nil {
		e.hub.Broadcast(websocket.Message{
			Type: websocket.EventAlertTriggered,
			Data: created,
		})
	}

	// Dispatch notifications.
	e.sendNotifications(ctx, rule, created)

	// Transition to notified.
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
	if err := e.db.UpsertAlertRuleState(ctx, s); err != nil {
		slog.Warn("Failed to persist alert rule state",
			"rule_id", s.RuleID, "device_id", s.DeviceID,
			"state", s.State, "error", err)
	}
}

func snapshotFromResults(results []ConditionResult) map[string]any {
	return map[string]any{"results": results}
}
