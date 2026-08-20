package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
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

	// ruleLocks serializes evaluation per (ruleID, deviceID) so the non-atomic
	// read-modify-write on alert_rule_state and the findActive→Create alert
	// TOCTOU race cannot fire the same rule twice under concurrency (pipeline
	// goroutine + port-scan handler).
	ruleLocks *keyedMutex

	// notifQueue decouples notification delivery from the synchronous
	// evaluation path. fireAlert enqueues; a bounded worker pool drains it so a
	// slow webhook or SMTP host never blocks metric persistence.
	notifQueue    chan notificationJob
	notifWorkers  int
	notifExitOnce sync.Once
}

const (
	notifQueueSize   = 256
	notifWorkerCount = 4
)

type notificationJob struct {
	rule  *models.AlertRule
	alert *models.Alert
}

func NewAlertEngine(db database.Database, hub *websocket.Hub, notifier *Notifier, opts ...AlertEngineOption) *AlertEngine {
	e := &AlertEngine{
		db:            db,
		hub:           hub,
		notifier:      notifier,
		baselineCache: NewBaselineCache(15 * time.Minute),
		ruleLocks:     newKeyedMutex(),
		notifQueue:    make(chan notificationJob, notifQueueSize),
		notifWorkers:  notifWorkerCount,
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

// WithBaselineCache injects a shared baseline cache (e.g. owned by the anomaly
// engine) so that anomaly conditions read the same refreshed baselines instead
// of an empty local cache that nothing ever populates.
func WithBaselineCache(bc *BaselineCache) AlertEngineOption {
	return func(e *AlertEngine) {
		if bc != nil {
			e.baselineCache = bc
		}
	}
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
		// Serialize the read-modify-write per (rule, device) so concurrent
		// evaluations cannot both fire the same rule or create duplicate active
		// alerts (H2).
		unlock := e.ruleLocks.lock(ruleLockKey(rule.ID, device.ID))
		e.evaluateRule(ctx, rule, device, metric, previousStatus)
		unlock()
	}
	return nil
}

func ruleLockKey(ruleID, deviceID int64) string {
	return fmt.Sprintf("%d:%d", ruleID, deviceID)
}

func (e *AlertEngine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go e.absenceLoop(ctx)
	for i := 0; i < e.notifWorkers; i++ {
		e.wg.Add(1)
		go e.notifWorker(ctx)
	}
	slog.Info("Alert engine started", "notif_workers", e.notifWorkers)
}

func (e *AlertEngine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	// Stop draining the notification queue so in-flight jobs finish on the
	// workers and no Send call ever blocks the evaluation path.
	e.notifExitOnce.Do(func() {
		if e.notifQueue != nil {
			close(e.notifQueue)
		}
	})
	e.wg.Wait()
	slog.Info("Alert engine stopped")
}

// ── absence background loop ──────────────────────────────────────────────────

func (e *AlertEngine) absenceLoop(ctx context.Context) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in absence loop", "panic", r, "stack", string(debug.Stack()))
		}
	}()
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

	// Skip devices with no metric history: a newly added device should not
	// fire an absence alert on the first tick. The absence clock starts at
	// first-seen (H24).
	if latest == nil {
		return
	}

	metric := &models.Metric{
		DeviceID:  device.ID,
		Timestamp: latest.Timestamp,
		Status:    latest.Status,
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
		if errors.Is(err, database.ErrDuplicateActiveAlert) {
			return
		}
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
	// Use the maximum duration across all conditions so that every condition
	// must sustain for at least its own duration before firing. This is more
	// correct than the previous behavior which used only the first condition's
	// duration and ignored the rest (H22).
	sustainedDuration := rule.CooldownSec
	for _, cond := range rule.Conditions {
		if cond.DurationSeconds > sustainedDuration {
			sustainedDuration = cond.DurationSeconds
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
		// Only persist state if the condition snapshot changed to avoid
		// writing on every single evaluation (H21).
		newSnapshot := snapshotFromResults(results)
		if !snapshotsEqual(state.ConditionSnapshot, newSnapshot) {
			state.LastEvaluatedAt = &now
			state.ConditionSnapshot = newSnapshot
			e.upsertState(ctx, state)
		}

	case "resolved":
		if state.LastResolvedAt != nil {
			cooldownLeft := float64(rule.CooldownSec) - now.Sub(*state.LastResolvedAt).Seconds()
			if cooldownLeft > 0 {
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
	// Group ID is rule+device so all alerts from the same rule on the same
	// device share a group (M44 — previously used rule+minute bucket which
	// collided across rules in the same minute and differed across restarts).
	groupID := fmt.Sprintf("%d-%d", rule.ID, device.ID)

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
		if errors.Is(err, database.ErrDuplicateActiveAlert) {
			// A concurrent instance already fired this rule/device. Re-point
			// the durable state at the existing alert without double-notifying.
			if existing := e.findActiveAlertForRule(ctx, rule.ID, device.ID); existing != nil {
				state.LastFiredAt = &now
				state.ActiveAlertID = &existing.ID
				state.State = "firing"
				e.upsertState(ctx, state)
			}
			return
		}
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

// sendNotifications hands the alert to the bounded notification worker pool and
// returns immediately. No notification I/O happens on the evaluation/pipeline
// goroutine, so a slow webhook or stuck SMTP host cannot stall metric
// persistence or alert evaluation (H3).
func (e *AlertEngine) sendNotifications(_ context.Context, rule *models.AlertRule, alert *models.Alert) {
	if e.notifier == nil || len(rule.ChannelIDs) == 0 {
		return
	}
	select {
	case e.notifQueue <- notificationJob{rule: rule, alert: alert}:
		slog.Debug("Alert queued for notifications",
			"alert_id", alert.ID, "rule_id", rule.ID, "device_id", alert.DeviceID)
	default:
		// Queue full: drop rather than block the monitoring pipeline. The
		// alert itself and its history are already persisted.
		slog.Warn("Notification queue full, dropping delivery",
			"alert_id", alert.ID, "rule_id", rule.ID, "device_id", alert.DeviceID)
	}
}

// notifWorker drains the notification queue with a per-delivery timeout so a
// slow channel is bounded and isolated from other deliveries.
func (e *AlertEngine) notifWorker(ctx context.Context) {
	defer e.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-e.notifQueue:
			if !ok {
				return
			}
			deliverCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			e.deliverNotifications(deliverCtx, job.rule, job.alert)
			cancel()
		}
	}
}

func (e *AlertEngine) deliverNotifications(ctx context.Context, rule *models.AlertRule, alert *models.Alert) {
	// History writes must not be dropped just because the delivery deadline
	// elapsed; use a context without cancellation for persistence.
	histCtx := context.WithoutCancel(ctx)
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
			e.recordHistory(histCtx, alert.ID, rule.ID, "notification_failed",
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
			e.recordHistory(histCtx, alert.ID, rule.ID, "notified",
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

// snapshotsEqual returns true if two condition snapshots represent the same
// set of condition results. Used to avoid redundant state writes (H21).
func snapshotsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	aResults, aOK := a["results"].([]ConditionResult)
	bResults, bOK := b["results"].([]ConditionResult)
	if !aOK || !bOK || len(aResults) != len(bResults) {
		return false
	}
	for i := range aResults {
		if aResults[i] != bResults[i] {
			return false
		}
	}
	return true
}
