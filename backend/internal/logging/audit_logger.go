package logging

import (
	"context"
	"log/slog"
)

// AuditLogger records security-relevant user actions.
// Audit events should always be persisted to the monitoring DB regardless of log level.
type AuditLogger struct {
	base *Logger
}

// NewAuditLogger creates an audit logger.
func NewAuditLogger(base *Logger) *AuditLogger {
	return &AuditLogger{base: base.With("audit")}
}

// AuditEvent holds all fields for a security/audit event.
type AuditEvent struct {
	EventType    string         // e.g., "auth.login_success", "config.device_created"
	Severity     string         // info, warn, error
	Actor        string         // "user:admin", "system", "api_key:keyname"
	ActorIP      string         // Remote IP address
	ResourceType string         // device, alert_rule, user, session, capture
	ResourceID   string         // ID of affected resource
	Description  string         // Human-readable description
	Details      map[string]any // Full event context
}

// LogEvent logs a structured audit event. This is the primary method — all convenience
// methods delegate here.
func (a *AuditLogger) LogEvent(ctx context.Context, event AuditEvent) {
	level := slog.LevelInfo
	switch event.Severity {
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	args := []any{
		"event", event.EventType,
		"severity", event.Severity,
	}
	if event.Actor != "" {
		args = append(args, "actor", event.Actor)
	}
	if event.ActorIP != "" {
		args = append(args, "actor_ip", event.ActorIP)
	}
	if event.ResourceType != "" {
		args = append(args, "resource_type", event.ResourceType)
	}
	if event.ResourceID != "" {
		args = append(args, "resource_id", event.ResourceID)
	}
	for k, v := range event.Details {
		args = append(args, k, v)
	}

	a.base.log(ctx, level, event.Description, args...)
}

// LogLogin logs a login success or failure event.
func (a *AuditLogger) LogLogin(ctx context.Context, userID int64, username, ip, userAgent string, success bool, details map[string]any) {
	eventType := "auth.login_success"
	severity := "info"
	desc := "User login successful"
	if !success {
		eventType = "auth.login_failure"
		severity = "warn"
		desc = "User login failed"
	}

	d := map[string]any{
		"user_id":    userID,
		"username":   username,
		"user_agent": userAgent,
	}
	for k, v := range details {
		d[k] = v
	}

	a.LogEvent(ctx, AuditEvent{
		EventType:    eventType,
		Severity:     severity,
		Actor:        "user:" + username,
		ActorIP:      ip,
		ResourceType: "session",
		Description:  desc,
		Details:      d,
	})
}

// LogLogout logs a user logout event.
func (a *AuditLogger) LogLogout(ctx context.Context, userID int64, username string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "auth.logout",
		Severity:     "info",
		Actor:        "user:" + username,
		ResourceType: "session",
		Description:  "User logged out",
		Details:      map[string]any{"user_id": userID, "username": username},
	})
}

// LogTokenRefresh logs a token refresh event.
func (a *AuditLogger) LogTokenRefresh(ctx context.Context, userID int64, username string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "auth.token_refresh",
		Severity:     "info",
		Actor:        "user:" + username,
		ResourceType: "session",
		Description:  "Token refreshed",
		Details:      map[string]any{"user_id": userID},
	})
}

// LogAPIKeyUsed logs an API key authentication event.
func (a *AuditLogger) LogAPIKeyUsed(ctx context.Context, keyName string, keyID int64, ip, endpoint string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "auth.apikey_used",
		Severity:     "info",
		Actor:        "api_key:" + keyName,
		ActorIP:      ip,
		ResourceType: "session",
		Description:  "API key authenticated",
		Details:      map[string]any{"key_name": keyName, "key_id": keyID, "endpoint": endpoint},
	})
}

// LogAPIKeyInvalid logs an invalid API key attempt.
func (a *AuditLogger) LogAPIKeyInvalid(ctx context.Context, ip, endpoint string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "auth.apikey_invalid",
		Severity:     "warn",
		ActorIP:      ip,
		ResourceType: "session",
		Description:  "Invalid API key attempted",
		Details:      map[string]any{"endpoint": endpoint},
	})
}

// LogRateLimit logs a rate limit exceeded event.
func (a *AuditLogger) LogRateLimit(ctx context.Context, userID string, authType, ip string, limit int, windowSec int) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "security.rate_limit",
		Severity:     "warn",
		ActorIP:      ip,
		ResourceType: "session",
		Description:  "Rate limit exceeded",
		Details:      map[string]any{"user_id": userID, "auth_type": authType, "limit": limit, "window_seconds": windowSec},
	})
}

// LogUnauthorized logs an unauthorized access attempt.
func (a *AuditLogger) LogUnauthorized(ctx context.Context, ip, path, reason string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "security.unauthorized",
		Severity:     "warn",
		ActorIP:      ip,
		Description:  "Unauthorized access attempt",
		Details:      map[string]any{"path": path, "reason": reason},
	})
}

// LogInvalidToken logs a malformed/invalid JWT event.
func (a *AuditLogger) LogInvalidToken(ctx context.Context, ip, reason string) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "security.invalid_token",
		Severity:     "warn",
		ActorIP:      ip,
		ResourceType: "session",
		Description:  "Invalid or malformed token",
		Details:      map[string]any{"reason": reason},
	})
}

// LogConfigChange logs a configuration change event (device/sensor/rule/channel/dashboard).
func (a *AuditLogger) LogConfigChange(ctx context.Context, action, resourceType string, resourceID int64, actor, actorIP string, details map[string]any) {
	eventType := "config." + resourceType + "_" + action
	severity := "info"
	if action == "deleted" {
		severity = "warn"
	}

	d := map[string]any{}
	for k, v := range details {
		d[k] = v
	}

	a.LogEvent(ctx, AuditEvent{
		EventType:    eventType,
		Severity:     severity,
		Actor:        actor,
		ActorIP:      actorIP,
		ResourceType: resourceType,
		ResourceID:   formatInt64(resourceID),
		Description:  resourceType + " " + action,
		Details:      d,
	})
}

// LogCaptureEvent logs packet capture start/stop events.
func (a *AuditLogger) LogCaptureEvent(ctx context.Context, action, actor, actorIP string, details map[string]any) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "capture." + action,
		Severity:     "info",
		Actor:        actor,
		ActorIP:      actorIP,
		ResourceType: "capture",
		Description:  "Packet capture " + action,
		Details:      details,
	})
}

// LogRetentionPurge logs a data retention purging event.
func (a *AuditLogger) LogRetentionPurge(ctx context.Context, details map[string]any) {
	a.LogEvent(ctx, AuditEvent{
		EventType:    "data.retention_purge",
		Severity:     "info",
		Actor:        "system",
		ResourceType: "data",
		Description:  "Data retention purge executed",
		Details:      details,
	})
}

// LogAction is a legacy convenience method for generic audit actions.
func (a *AuditLogger) LogAction(ctx context.Context, userID int64, action, resource string, resourceID int64, ok bool) {
	severity := "info"
	if !ok {
		severity = "warn"
	}
	a.LogEvent(ctx, AuditEvent{
		EventType:    "audit." + action,
		Severity:     severity,
		Actor:        formatActor(userID),
		ResourceType: resource,
		ResourceID:   formatInt64(resourceID),
		Description:  action + " on " + resource,
		Details:      map[string]any{"user_id": userID, "ok": ok},
	})
}

func formatActor(userID int64) string {
	if userID > 0 {
		return "user:" + formatInt64(userID)
	}
	return "system"
}

func formatInt64(v int64) string {
	if v == 0 {
		return ""
	}
	return slog.Int64Value(v).String()
}
