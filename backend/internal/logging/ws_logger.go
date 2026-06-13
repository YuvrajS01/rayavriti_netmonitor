package logging

import (
	"context"
	"log/slog"
)

// WSLogger logs WebSocket events.
type WSLogger struct {
	base *Logger
}

// NewWSLogger creates a WebSocket event logger.
func NewWSLogger(base *Logger) *WSLogger {
	return &WSLogger{base: base.With("websocket")}
}

// LogConnect logs a WebSocket client connection.
func (w *WSLogger) LogConnect(ctx context.Context, clientID string, userID int64, username, remoteAddr string, totalConnections int) {
	w.base.base.LogAttrs(ctx, slog.LevelInfo, "Client connected",
		slog.String("event", "ws.connect"),
		slog.String("socket_id", clientID),
		slog.Int64("user_id", userID),
		slog.String("username", username),
		slog.String("remote_addr", remoteAddr),
		slog.String("transport", "websocket"),
		slog.Int("total_connections", totalConnections),
	)
}

// LogDisconnect logs a WebSocket client disconnection.
func (w *WSLogger) LogDisconnect(ctx context.Context, clientID string, userID int64, username string, totalConnections int) {
	w.base.base.LogAttrs(ctx, slog.LevelInfo, "Client disconnected",
		slog.String("event", "ws.disconnect"),
		slog.String("socket_id", clientID),
		slog.Int64("user_id", userID),
		slog.String("username", username),
		slog.Int("total_connections", totalConnections),
	)
}

// LogBroadcast logs a WebSocket event broadcast.
func (w *WSLogger) LogBroadcast(ctx context.Context, eventName string, payloadBytes, recipientCount int, deviceID int64) {
	w.base.base.LogAttrs(ctx, slog.LevelDebug, "Broadcasting "+eventName,
		slog.String("event", "ws.broadcast"),
		slog.String("event_name", eventName),
		slog.Int("payload_bytes", payloadBytes),
		slog.Int("recipient_count", recipientCount),
		slog.Int64("device_id", deviceID),
	)
}
