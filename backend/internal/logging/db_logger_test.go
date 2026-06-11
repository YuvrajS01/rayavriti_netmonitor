package logging

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func testLogger() *Logger {
	cfg := &config.Config{
		App: config.AppConfig{Version: "1.1.0", NodeEnv: "development"},
		Logging: config.LoggingConfig{Level: "debug", Format: "pretty", FileEnabled: false},
	}
	return New(cfg)
}

func TestDBLogger_LogQuery_Success(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	ctx := context.Background()
	dbLogger.LogQuery(ctx, "SELECT", "devices", "GetDevices",
		"SELECT * FROM devices", nil, 10*time.Millisecond, 5, 0, nil)
}

func TestDBLogger_LogQuery_Error(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	ctx := context.Background()
	dbLogger.LogQuery(ctx, "INSERT", "devices", "CreateDevice",
		"INSERT INTO devices", []any{"test"}, 5*time.Millisecond, 0, 0, assert.AnError)
}

func TestDBLogger_LogQuery_Slow(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 1) // 1ms threshold
	ctx := context.Background()
	dbLogger.LogQuery(ctx, "SELECT", "metrics", "GetMetrics",
		"SELECT * FROM metrics", nil, 10*time.Millisecond, 100, 0, nil)
}

func TestDBLogger_LogTransaction_Begin(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	ctx := context.Background()
	dbLogger.LogTransaction(ctx, "BEGIN", 1*time.Millisecond, nil)
}

func TestDBLogger_LogTransaction_CommitError(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	ctx := context.Background()
	dbLogger.LogTransaction(ctx, "COMMIT", 1*time.Millisecond, assert.AnError)
}

func TestDBLogger_LogPoolStats(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	dbLogger := NewDBLogger(logger, 100)
	dbLogger.LogPoolStats(10, 5, 3, 0, 1.5)
}
