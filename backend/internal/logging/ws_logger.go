package logging

import (
	"context"
	"log/slog"
)

// WSLogger logs WebSocket events.
type WSLogger struct {
	base *Logger
}

func NewWSLogger(base *Logger) *WSLogger {
	return &WSLogger{base: base}
}

func (w *WSLogger) LogConnect(ctx context.Context, clientID string, userID int64) {
	w.base.base.LogAttrs(ctx, slog.LevelInfo, "ws.connected",
		slog.String("client_id", clientID),
		slog.Int64("user_id", userID),
	)
}

func (w *WSLogger) LogDisconnect(ctx context.Context, clientID string, userID int64) {
	w.base.base.LogAttrs(ctx, slog.LevelInfo, "ws.disconnected",
		slog.String("client_id", clientID),
		slog.Int64("user_id", userID),
	)
}

func (w *WSLogger) LogBroadcast(ctx context.Context, event string, count int) {
	w.base.base.LogAttrs(ctx, slog.LevelDebug, "ws.broadcast",
		slog.String("event", event),
		slog.Int("recipients", count),
	)
}
