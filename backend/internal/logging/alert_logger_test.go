package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlertLogger_LogEvaluation(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	assert.NotNil(t, alertLogger)

	conditionResults := []ConditionResult{
		{ConditionID: 1, Type: "threshold", Field: "response_time", Result: true, Threshold: 1000, ActualValue: 1500},
	}
	alertLogger.LogEvaluation(context.Background(), 1, 1, "Device Down", "Server-1",
		2, 1, conditionResults, "triggered", "pending")
}

func TestAlertLogger_LogTriggered(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogTriggered(context.Background(), 1, 1, 1, "Device Down", "Server-1",
		"critical", "conditions sustained for 60s", map[string]any{"response_time": 1500}, 0)
}

func TestAlertLogger_LogResolved(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogResolved(context.Background(), 1, 1, "admin")
}

func TestAlertLogger_LogAutoResolved(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogAutoResolved(context.Background(), 1, 1, 1, "Server-1", 300, "all_conditions_cleared")
}

func TestAlertLogger_LogNotificationSent(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogNotificationSent(context.Background(), 1, 1, "webhook", "My Webhook", 200, 0, 150.5)
}

func TestAlertLogger_LogNotificationFailed(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogNotificationFailed(context.Background(), 1, 1, "webhook", "My Webhook",
		assert.AnError, 0, 3, true, 5000.0)
}

func TestAlertLogger_LogCooldownSkip(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	alertLogger := NewAlertLogger(logger)
	alertLogger.LogCooldownSkip(context.Background(), 1, 1, "Device Down", "Server-1", 30)
}
