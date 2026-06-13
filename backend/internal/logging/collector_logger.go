package logging

import (
	"context"
	"fmt"
	"log/slog"
)

// CollectorLogger records collector execution results with per-protocol detail.
type CollectorLogger struct {
	base *Logger
}

// NewCollectorLogger creates a collector logger.
func NewCollectorLogger(base *Logger) *CollectorLogger {
	return &CollectorLogger{base: base}
}

// CollectorEvent holds all fields for a collector execution event.
type CollectorEvent struct {
	DeviceID            int64
	DeviceName          string
	Host                string
	Protocol            string
	SensorID            int64
	Status              string
	PreviousStatus      string
	StatusChanged       bool
	ResponseTimeMs      float64
	Value               float64
	Message             string
	MetricID            int64
	Error               error
	ConsecutiveFailures int
	DurationMs          float64
	// Per-protocol extra fields
	Extra map[string]any
}

// LogStart logs the beginning of a collector run.
func (c *CollectorLogger) LogStart(ctx context.Context, deviceID int64, deviceName, host, protocol string, sensorID int64, intervalSec int) {
	l := c.base.With("collector." + protocol)
	l.DebugCtx(ctx, fmt.Sprintf("Starting collection: %s (%s)", deviceName, host),
		"event", "collector_start",
		"device_id", deviceID,
		"device_name", deviceName,
		"host", host,
		"protocol", protocol,
		"sensor_id", sensorID,
		"interval_seconds", intervalSec,
	)
}

// LogResult logs a successful collector run with full detail.
func (c *CollectorLogger) LogResult(ctx context.Context, evt CollectorEvent) {
	l := c.base.With("collector." + evt.Protocol)

	attrs := []any{
		"event", "collector_result",
		"device_id", evt.DeviceID,
		"device_name", evt.DeviceName,
		"host", evt.Host,
		"protocol", evt.Protocol,
		"status", evt.Status,
		"previous_status", evt.PreviousStatus,
		"status_changed", evt.StatusChanged,
		"response_time_ms", evt.ResponseTimeMs,
		"value", evt.Value,
		"message", evt.Message,
		"metric_id", evt.MetricID,
		"duration_ms", evt.DurationMs,
	}

	if evt.SensorID > 0 {
		attrs = append(attrs, "sensor_id", evt.SensorID)
	}

	// Add per-protocol extra fields
	for k, v := range evt.Extra {
		attrs = append(attrs, k, v)
	}

	msg := fmt.Sprintf("✓ %s (%s) → %s (%.0fms)", evt.DeviceName, evt.Host, evt.Status, evt.ResponseTimeMs)
	l.InfoCtx(ctx, msg, attrs...)
}

// LogFailure logs a failed collector run.
func (c *CollectorLogger) LogFailure(ctx context.Context, evt CollectorEvent) {
	l := c.base.With("collector." + evt.Protocol)

	attrs := []any{
		"event", "collector_error",
		"device_id", evt.DeviceID,
		"device_name", evt.DeviceName,
		"host", evt.Host,
		"protocol", evt.Protocol,
		"status", evt.Status,
		"previous_status", evt.PreviousStatus,
		"status_changed", evt.StatusChanged,
		"duration_ms", evt.DurationMs,
		"consecutive_failures", evt.ConsecutiveFailures,
	}

	if evt.Error != nil {
		attrs = append(attrs, "error", evt.Error.Error())
	}

	// Add per-protocol extra fields
	for k, v := range evt.Extra {
		attrs = append(attrs, k, v)
	}

	msg := fmt.Sprintf("✗ %s (%s) → %s", evt.DeviceName, evt.Host, evt.Status)
	l.ErrorCtx(ctx, msg, attrs...)
}

// LogCycleStart logs the beginning of a complete polling cycle.
func (c *CollectorLogger) LogCycleStart(ctx context.Context, deviceCount int) {
	c.base.base.LogAttrs(ctx, slog.LevelDebug, "collector.cycle_start",
		slog.Int("devices", deviceCount),
	)
}

// LogCycleEnd logs the completion of a polling cycle.
func (c *CollectorLogger) LogCycleEnd(ctx context.Context, deviceCount int, durationMs float64) {
	c.base.base.LogAttrs(ctx, slog.LevelDebug, "collector.cycle_end",
		slog.Int("devices", deviceCount),
		slog.Float64("duration_ms", durationMs),
	)
}

// LogSNMPDetail logs SNMP varbind response data at TRACE level.
func (c *CollectorLogger) LogSNMPDetail(ctx context.Context, deviceID int64, oids []string, varbinds []map[string]any, responseTimeMs float64) {
	l := c.base.With("collector.snmp")
	l.TraceCtx(ctx, "SNMP GET response",
		"event", "snmp_response",
		"device_id", deviceID,
		"oids_requested", oids,
		"varbinds", varbinds,
		"response_time_ms", responseTimeMs,
	)
}
