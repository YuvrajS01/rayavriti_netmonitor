package logging

import (
	"context"
	"log/slog"
)

// AuditLogger records security-relevant user actions.
type AuditLogger struct {
	base *Logger
}

func NewAuditLogger(base *Logger) *AuditLogger {
	return &AuditLogger{base: base}
}

func (a *AuditLogger) LogLogin(ctx context.Context, userID int64, username, ip string, success bool) {
	a.base.base.LogAttrs(ctx, slog.LevelInfo, "auth.login",
		slog.Int64("user_id", userID),
		slog.String("username", username),
		slog.String("ip", ip),
		slog.Bool("success", success),
	)
}

func (a *AuditLogger) LogLogout(ctx context.Context, userID int64, username string) {
	a.base.base.LogAttrs(ctx, slog.LevelInfo, "auth.logout",
		slog.Int64("user_id", userID),
		slog.String("username", username),
	)
}

func (a *AuditLogger) LogAction(ctx context.Context, userID int64, action, resource string, resourceID int64, ok bool) {
	a.base.base.LogAttrs(ctx, slog.LevelInfo, "audit.action",
		slog.Int64("user_id", userID),
		slog.String("action", action),
		slog.String("resource", resource),
		slog.Int64("resource_id", resourceID),
		slog.Bool("ok", ok),
	)
}
