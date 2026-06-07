package logging

import (
	"context"
	"log/slog"
	"time"
)

// CollectorLogger records collector execution results.
type CollectorLogger struct {
	base *Logger
}

func NewCollectorLogger(base *Logger) *CollectorLogger {
	return &CollectorLogger{base: base}
}

func (c *CollectorLogger) LogResult(ctx context.Context, deviceID int64, deviceName, protocol, status string, durationMs float64, err error) {
	level := slog.LevelDebug
	if status == "down" || err != nil {
		level = slog.LevelWarn
	}
	attrs := []slog.Attr{
		slog.Int64("device_id", deviceID),
		slog.String("device", deviceName),
		slog.String("protocol", protocol),
		slog.String("status", status),
		slog.Float64("duration_ms", durationMs),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	c.base.base.LogAttrs(ctx, level, "collector.result", attrs...)
}

func (c *CollectorLogger) LogCycleStart(ctx context.Context, deviceCount int) {
	c.base.base.LogAttrs(ctx, slog.LevelDebug, "collector.cycle_start",
		slog.Int("devices", deviceCount),
		slog.Time("at", time.Now()),
	)
}

func (c *CollectorLogger) LogCycleEnd(ctx context.Context, deviceCount int, durationMs float64) {
	c.base.base.LogAttrs(ctx, slog.LevelDebug, "collector.cycle_end",
		slog.Int("devices", deviceCount),
		slog.Float64("duration_ms", durationMs),
	)
}
