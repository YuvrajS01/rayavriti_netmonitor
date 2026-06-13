package logging

import (
	"context"
	"fmt"
	"log/slog"
)

// AlertLogger records alert engine decisions including rule evaluation,
// alert firing, notification delivery, and auto-resolution.
type AlertLogger struct {
	base *Logger
}

// NewAlertLogger creates an alert engine logger.
func NewAlertLogger(base *Logger) *AlertLogger {
	return &AlertLogger{base: base}
}

// ConditionResult holds the evaluation result for a single alert condition.
type ConditionResult struct {
	ConditionID         int64   `json:"condition_id"`
	Type                string  `json:"type"`
	Field               string  `json:"field"`
	Operator            string  `json:"operator"`
	Threshold           float64 `json:"threshold"`
	ActualValue         float64 `json:"actual_value"`
	Result              bool    `json:"result"`
	SustainedSeconds    int     `json:"sustained_seconds"`
	RequiredDurationSec int     `json:"required_duration_seconds"`
}

// LogEvaluation logs a rule evaluation with full condition results.
func (a *AlertLogger) LogEvaluation(ctx context.Context, ruleID, deviceID int64, ruleName, deviceName string, conditionsChecked, conditionsMet int, conditionResults []ConditionResult, verdict, ruleState string) {
	l := a.base.With("alert_engine")
	l.DebugCtx(ctx, fmt.Sprintf("Evaluating rule '%s' for device %d", ruleName, deviceID),
		"event", "rule_evaluation",
		"rule_id", ruleID,
		"rule_name", ruleName,
		"device_id", deviceID,
		"device_name", deviceName,
		"conditions_checked", conditionsChecked,
		"conditions_met", conditionsMet,
		"condition_results", conditionResults,
		"verdict", verdict,
		"rule_state", ruleState,
	)
}

// LogTriggered logs an alert that has been fired.
func (a *AlertLogger) LogTriggered(ctx context.Context, alertID, ruleID, deviceID int64, ruleName, deviceName, severity, triggerReason string, conditionValues map[string]any, cooldownRemainingSec int) {
	l := a.base.With("alert_engine")
	l.WarnCtx(ctx, fmt.Sprintf("🔔 Alert FIRED: %s — %s", ruleName, deviceName),
		"event", "alert_fired",
		"alert_id", alertID,
		"rule_id", ruleID,
		"rule_name", ruleName,
		"severity", severity,
		"device_id", deviceID,
		"device_name", deviceName,
		"trigger_reason", triggerReason,
		"condition_values", conditionValues,
		"cooldown_remaining_seconds", cooldownRemainingSec,
	)
}

// LogResolved logs an alert that has been manually resolved.
func (a *AlertLogger) LogResolved(ctx context.Context, alertID, deviceID int64, resolvedBy string) {
	l := a.base.With("alert_engine")
	l.InfoCtx(ctx, "Alert resolved",
		"event", "alert_resolved",
		"alert_id", alertID,
		"device_id", deviceID,
		"resolved_by", resolvedBy,
	)
}

// LogAutoResolved logs an alert that was automatically resolved when conditions cleared.
func (a *AlertLogger) LogAutoResolved(ctx context.Context, alertID, ruleID, deviceID int64, deviceName string, durationActiveSec int, reason string) {
	l := a.base.With("alert_engine")
	l.InfoCtx(ctx, fmt.Sprintf("✓ Alert auto-resolved: %s", deviceName),
		"event", "alert_auto_resolved",
		"alert_id", alertID,
		"rule_id", ruleID,
		"device_id", deviceID,
		"device_name", deviceName,
		"duration_active_seconds", durationActiveSec,
		"resolved_reason", reason,
	)
}

// LogNotificationSent logs a successful notification delivery.
func (a *AlertLogger) LogNotificationSent(ctx context.Context, alertID, channelID int64, channelType, channelName string, httpStatus, retryCount int, durationMs float64) {
	l := a.base.With("alert_engine.notifier")
	l.InfoCtx(ctx, fmt.Sprintf("Notification delivered: %s → %s", channelType, channelName),
		"event", "notification_sent",
		"alert_id", alertID,
		"channel_id", channelID,
		"channel_type", channelType,
		"channel_name", channelName,
		"delivery_status", "success",
		"http_status", httpStatus,
		"retry_count", retryCount,
		"duration_ms", durationMs,
	)
}

// LogNotificationFailed logs a failed notification delivery attempt.
func (a *AlertLogger) LogNotificationFailed(ctx context.Context, alertID, channelID int64, channelType, channelName string, err error, retryCount, maxRetries int, willRetry bool, durationMs float64) {
	l := a.base.With("alert_engine.notifier")
	l.base.LogAttrs(ctx, slog.LevelError,
		fmt.Sprintf("Notification delivery FAILED: %s → %s", channelType, channelName),
		slog.String("event", "notification_failed"),
		slog.Int64("alert_id", alertID),
		slog.Int64("channel_id", channelID),
		slog.String("channel_type", channelType),
		slog.String("channel_name", channelName),
		slog.String("delivery_status", "failed"),
		slog.String("error", err.Error()),
		slog.Int("retry_count", retryCount),
		slog.Int("max_retries", maxRetries),
		slog.Bool("will_retry", willRetry),
		slog.Float64("duration_ms", durationMs),
	)
}

// LogCooldownSkip logs when an alert evaluation is skipped due to cooldown period.
func (a *AlertLogger) LogCooldownSkip(ctx context.Context, ruleID, deviceID int64, ruleName, deviceName string, cooldownRemainingSec int) {
	l := a.base.With("alert_engine")
	l.DebugCtx(ctx, fmt.Sprintf("Cooldown skip: %s — %s", ruleName, deviceName),
		"event", "cooldown_skip",
		"rule_id", ruleID,
		"device_id", deviceID,
		"rule_name", ruleName,
		"device_name", deviceName,
		"cooldown_remaining_seconds", cooldownRemainingSec,
	)
}
