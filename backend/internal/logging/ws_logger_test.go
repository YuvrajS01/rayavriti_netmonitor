package logging

import (
	"context"
	"testing"
)

func TestWSLogger_LogConnect(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	wsLogger := NewWSLogger(logger)
	wsLogger.LogConnect(context.Background(), "client-1", 1, "admin", "127.0.0.1:12345", 5)
}

func TestWSLogger_LogDisconnect(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	wsLogger := NewWSLogger(logger)
	wsLogger.LogDisconnect(context.Background(), "client-1", 1, "admin", 4)
}

func TestWSLogger_LogBroadcast(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	wsLogger := NewWSLogger(logger)
	wsLogger.LogBroadcast(context.Background(), "metric:update", 256, 3, 1)
}
