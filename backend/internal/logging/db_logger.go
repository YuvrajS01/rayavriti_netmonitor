package logging

import (
	"context"
	"time"
)

type DBLogger struct {
	l         *Logger
	slowQuery time.Duration
}

func NewDBLogger(logger *Logger, slowQueryMs int) *DBLogger {
	return &DBLogger{
		l:         logger.With("db"),
		slowQuery: time.Duration(slowQueryMs) * time.Millisecond,
	}
}

func (d *DBLogger) LogQuery(ctx context.Context, operation, table, method, sql string, dur time.Duration, rows int, err error) {
	args := []any{
		"op", operation,
		"table", table,
		"method", method,
		"duration_ms", float64(dur.Milliseconds()),
		"rows", rows,
		"request_id", GetRequestID(ctx),
	}
	if err != nil {
		d.l.ErrorCtx(ctx, "query failed: "+method, append(args, "error", err.Error())...)
		return
	}
	if dur >= d.slowQuery {
		d.l.WarnCtx(ctx, "slow query: "+method, append(args, "sql", sql)...)
		return
	}
	d.l.DebugCtx(ctx, "query ok: "+method, args...)
}
