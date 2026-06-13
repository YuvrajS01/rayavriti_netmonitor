package logging

import (
	"context"
	"fmt"
	"time"
)

// DBLogger wraps database operations with structured logging.
type DBLogger struct {
	l         *Logger
	slowQuery time.Duration
}

// NewDBLogger creates a database logger with a slow query threshold.
func NewDBLogger(logger *Logger, slowQueryMs int) *DBLogger {
	return &DBLogger{
		l:         logger.With("db"),
		slowQuery: time.Duration(slowQueryMs) * time.Millisecond,
	}
}

// LogQuery logs a database query with full context.
func (d *DBLogger) LogQuery(ctx context.Context, operation, table, method, sql string, params []any, dur time.Duration, rowsReturned, rowsAffected int, err error) {
	durationMs := float64(dur.Microseconds()) / 1000.0
	isSlow := dur >= d.slowQuery

	args := []any{
		"op", operation,
		"table", table,
		"method", method,
		"duration_ms", durationMs,
		"rows_returned", rowsReturned,
		"rows_affected", rowsAffected,
		"request_id", GetRequestID(ctx),
	}

	if traceID := GetTraceID(ctx); traceID != "" {
		args = append(args, "trace_id", traceID)
	}

	if err != nil {
		args = append(args, "event", "query_error", "error", err.Error(), "sql", sql)
		if params != nil {
			args = append(args, "params", fmt.Sprintf("%v", params))
		}
		d.l.ErrorCtx(ctx, fmt.Sprintf("%s failed: %s", operation, method), args...)
		return
	}

	if isSlow {
		args = append(args, "event", "slow_query", "sql", sql, "slow_query", true, "threshold_ms", int(d.slowQuery.Milliseconds()))
		if params != nil {
			args = append(args, "params", fmt.Sprintf("%v", params))
		}
		d.l.WarnCtx(ctx, fmt.Sprintf("Slow query detected (%.1fms): %s", durationMs, method), args...)
		return
	}

	// Normal query — include SQL at debug level
	args = append(args, "event", "query", "sql", sql)
	d.l.DebugCtx(ctx, fmt.Sprintf("%s completed: %s", operation, method), args...)
}

// LogTransaction logs transaction boundary events (BEGIN, COMMIT, ROLLBACK).
func (d *DBLogger) LogTransaction(ctx context.Context, action string, dur time.Duration, err error) {
	args := []any{
		"event", "transaction",
		"action", action,
		"duration_ms", float64(dur.Microseconds()) / 1000.0,
		"request_id", GetRequestID(ctx),
	}
	if err != nil {
		args = append(args, "error", err.Error())
		d.l.ErrorCtx(ctx, fmt.Sprintf("Transaction %s failed", action), args...)
		return
	}
	d.l.DebugCtx(ctx, fmt.Sprintf("Transaction %s", action), args...)
}

// LogPoolStats logs connection pool statistics at info level.
func (d *DBLogger) LogPoolStats(totalConns, idleConns, acquiredConns int32, emptyAcquireCount int64, acquireDurationMs float64) {
	d.l.Info("Connection pool stats",
		"event", "pool_stats",
		"total_connections", totalConns,
		"idle_connections", idleConns,
		"acquired_connections", acquiredConns,
		"empty_acquire_count", emptyAcquireCount,
		"acquire_duration_ms", acquireDurationMs,
	)
}
