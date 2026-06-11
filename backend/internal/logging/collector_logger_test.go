package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectorLogger_LogStart(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collectorLogger := NewCollectorLogger(logger)
	collectorLogger.LogStart(context.Background(), 1, "Server-1", "192.168.1.1", "ping", 0, 60)
}

func TestCollectorLogger_LogResult(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collectorLogger := NewCollectorLogger(logger)
	collectorLogger.LogResult(context.Background(), CollectorEvent{
		DeviceID:       1,
		DeviceName:     "Server-1",
		Host:           "192.168.1.1",
		Protocol:       "ping",
		Status:         "up",
		ResponseTimeMs: 50,
		DurationMs:     55,
	})
}

func TestCollectorLogger_LogFailure(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collectorLogger := NewCollectorLogger(logger)
	collectorLogger.LogFailure(context.Background(), CollectorEvent{
		DeviceID:   1,
		DeviceName: "Server-1",
		Host:       "192.168.1.1",
		Protocol:   "ping",
		Status:     "down",
		Error:      assert.AnError,
		DurationMs: 5000,
	})
}

func TestCollectorLogger_LogCycleStartEnd(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	collectorLogger := NewCollectorLogger(logger)
	collectorLogger.LogCycleStart(context.Background(), 10)
	collectorLogger.LogCycleEnd(context.Background(), 10, 1500.5)
}
