package logging

import (
	"context"
	"log/slog"
)

// AlertLogger records alert engine decisions.
type AlertLogger struct {
	base *Logger
}

func NewAlertLogger(base *Logger) *AlertLogger {
	return &AlertLogger{base: base}
}

func (a *AlertLogger) LogTriggered(ctx context.Context, ruleID, deviceID int64, ruleName, severity, message string) {
	a.base.base.LogAttrs(ctx, slog.LevelWarn, "alert.triggered",
		slog.Int64("rule_id", ruleID),
		slog.Int64("device_id", deviceID),
		slog.String("rule_name", ruleName),
		slog.String("severity", severity),
		slog.String("message", message),
	)
}

func (a *AlertLogger) LogResolved(ctx context.Context, alertID, deviceID int64) {
	a.base.base.LogAttrs(ctx, slog.LevelInfo, "alert.resolved",
		slog.Int64("alert_id", alertID),
		slog.Int64("device_id", deviceID),
	)
}

func (a *AlertLogger) LogEvaluation(ctx context.Context, ruleID, deviceID int64, matched bool) {
	a.base.base.LogAttrs(ctx, slog.LevelDebug, "alert.evaluated",
		slog.Int64("rule_id", ruleID),
		slog.Int64("device_id", deviceID),
		slog.Bool("matched", matched),
	)
}
