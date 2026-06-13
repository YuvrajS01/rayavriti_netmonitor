package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

// ── ProcessMetric: enabled rule that matches ──────────────────────────────────

func TestProcessMetric_EnabledRule_Matches(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    0,
				Conditions: []models.AlertRuleCondition{{
					ID:          1,
					Type:        "threshold",
					MetricField: "cpu_usage",
					Operator:    "gt",
					Value:       "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 100
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: device scope filter ────────────────────────────────────────

func TestProcessMetric_DeviceScopeFilter(t *testing.T) {
	t.Parallel()
otherDeviceID := int64(99)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             1,
				Name:           "Device-specific rule",
				Enabled:        true,
				ScopeType:      "device",
				DeviceID:       &otherDeviceID,
				Severity:       "critical",
				ConditionLogic: "all",
				Conditions: []models.AlertRuleCondition{{
					ID:          1,
					Type:        "state_change",
					MetricField: "status",
					Operator:    "eq",
					Value:       "down",
				}},
			}}, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, Status: "down"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: "any" condition logic ──────────────────────────────────────

func TestProcessMetric_AnyConditionLogic(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "Any condition",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "any",
				Conditions: []models.AlertRuleCondition{
					{ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"},
					{ID: 2, Type: "threshold", MetricField: "memory_usage", Operator: "gt", Value: "90"},
				},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 200
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: multiple rules ─────────────────────────────────────────────

func TestProcessMetric_MultipleRules(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{
				{
					ID: 1, Name: "Disabled", Enabled: false,
					Conditions: []models.AlertRuleCondition{{ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"}},
				},
				{
					ID: 2, Name: "No conditions", Enabled: true,
					Conditions: nil,
				},
				{
					ID: 3, Name: "Matches", Enabled: true, Severity: "info",
					ConditionLogic: "all",
					Conditions: []models.AlertRuleCondition{{ID: 3, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"}},
				},
			}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 300
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: state transitions idle→pending→firing ──────────────────────

func TestProcessMetric_StateTransition_IdleToPending(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	stateReturned := false
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			if stateReturned {
				return nil, assert.AnError
			}
			stateReturned = true
			return &models.AlertRuleState{
				RuleID:   ruleID,
				DeviceID: 1,
				State:    "idle",
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 400
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

func TestProcessMetric_StateTransition_PendingToFiring(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	firstMet := time.Now().Add(-600 * time.Second) // well past cooldown
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:      ruleID,
				DeviceID:    1,
				State:       "pending",
				FirstMetAt:  &firstMet,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 500
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: condition cleared (auto-resolve) ───────────────────────────

func TestProcessMetric_ConditionCleared_AutoResolve(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	alertID := int64(500)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				AutoResolve:    true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:        ruleID,
				DeviceID:      1,
				State:         "firing",
				ActiveAlertID: &alertID,
			}, nil
		},
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0 // below threshold
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: condition cleared (no auto-resolve) ────────────────────────

func TestProcessMetric_ConditionCleared_NoAutoResolve(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	alertID := int64(600)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				AutoResolve:    false,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:        ruleID,
				DeviceID:      1,
				State:         "firing",
				ActiveAlertID: &alertID,
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: createAlert fails ──────────────────────────────────────────

func TestProcessMetric_CreateAlertFails(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    0,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			return nil, fmt.Errorf("db write failed")
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: upsertState fails ──────────────────────────────────────────

func TestProcessMetric_UpsertStateFails(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 700
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return fmt.Errorf("upsert failed")
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: notifier configured ────────────────────────────────────────

func TestProcessMetric_WithNotifier(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "critical",
				ConditionLogic: "all",
				CooldownSec:    0,
				ChannelIDs:     []int64{channelID},
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 800
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{{
				ID:      channelID,
				Name:    "Test Webhook",
				Type:    "webhook",
				Enabled: true,
				Config:  map[string]any{"url": "http://invalid.example.com"},
			}}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	notifier := NewNotifier()
	engine := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: existing active alert (idempotency) ────────────────────────

func TestProcessMetric_ExistingActiveAlert(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	firstMet := time.Now().Add(-600 * time.Second)
	existingAlertID := int64(900)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:      ruleID,
				DeviceID:    1,
				State:       "pending",
				FirstMetAt:  &firstMet,
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return []models.Alert{{
				ID:       existingAlertID,
				RuleID:   &ruleID,
				DeviceID: 1,
				Status:   "active",
			}}, 1, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── findActiveAlertForRule ────────────────────────────────────────────────────

func TestFindActiveAlertForRule_DBError(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, fmt.Errorf("db error")
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	result := engine.findActiveAlertForRule(context.Background(), 1, 1)
	assert.Nil(t, result)
}

func TestFindActiveAlertForRule_Found(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		findActiveAlertByRuleAndDeviceFn: func(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
			id := int64(42)
			return &models.Alert{
				ID:       100,
				RuleID:   &id,
				DeviceID: 5,
				Status:   "active",
			}, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	result := engine.findActiveAlertForRule(context.Background(), 42, 5)
	require.NotNil(t, result)
	assert.Equal(t, int64(100), result.ID)
}

func TestFindActiveAlertForRule_NotFound(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return []models.Alert{}, 0, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	result := engine.findActiveAlertForRule(context.Background(), 1, 1)
	assert.Nil(t, result)
}

func TestFindActiveAlertForRule_RuleIDMismatch(t *testing.T) {
	t.Parallel()
	otherRule := int64(99)
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return []models.Alert{{
				ID:       100,
				RuleID:   &otherRule,
				DeviceID: 1,
				Status:   "active",
			}}, 1, nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	result := engine.findActiveAlertForRule(context.Background(), 42, 1)
	assert.Nil(t, result)
}

// ── snapshotFromResults ───────────────────────────────────────────────────────

func TestSnapshotFromResults_Empty(t *testing.T) {
	t.Parallel()
	snap := snapshotFromResults([]ConditionResult{})
	assert.NotNil(t, snap)
	results, ok := snap["results"].([]ConditionResult)
	require.True(t, ok)
	assert.Empty(t, results)
}

func TestSnapshotFromResults_WithData(t *testing.T) {
	t.Parallel()
	results := []ConditionResult{
		{ConditionID: 1, Type: "threshold", Field: "cpu", Result: true, ActualValue: 95, Threshold: 80},
		{ConditionID: 2, Type: "state_change", Field: "status", Result: false},
	}
	snap := snapshotFromResults(results)
	require.NotNil(t, snap)
	got := snap["results"].([]ConditionResult)
	assert.Len(t, got, 2)
	assert.Equal(t, true, got[0].Result)
}

// ── ProcessMetric: pending state (still pending, not yet fired) ───────────────

func TestProcessMetric_PendingStillPending(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	firstMet := time.Now().Add(-10 * time.Second) // not yet past cooldown
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet,
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: pending cleared → idle ─────────────────────────────────────

func TestProcessMetric_PendingClearedToIdle(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:   ruleID,
				DeviceID: 1,
				State:    "pending",
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0 // below threshold
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: resolved→firing re-fire after cooldown ─────────────────────

func TestProcessMetric_ResolvedToFiring(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	lastResolved := time.Now().Add(-600 * time.Second)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:         ruleID,
				DeviceID:       1,
				State:          "resolved",
				LastResolvedAt: &lastResolved,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 1100
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: resolved state still in cooldown ───────────────────────────

func TestProcessMetric_ResolvedInCooldown(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	lastResolved := time.Now().Add(-10 * time.Second) // still in cooldown
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    300,
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:         ruleID,
				DeviceID:       1,
				State:          "resolved",
				LastResolvedAt: &lastResolved,
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: firing/notified/acknowledged states ────────────────────────

func TestProcessMetric_AlreadyNotified(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:   ruleID,
				DeviceID: 1,
				State:    "notified",
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

func TestProcessMetric_AlreadyAcknowledged(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return &models.AlertRuleState{
				RuleID:   ruleID,
				DeviceID: 1,
				State:    "acknowledged",
			}, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: sendNotifications with channel load error ──────────────────

func TestProcessMetric_NotifierChannelLoadError(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "critical",
				ConditionLogic: "all",
				CooldownSec:    0,
				ChannelIDs:     []int64{channelID},
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 1200
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return nil, fmt.Errorf("channel load error")
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	notifier := NewNotifier()
	engine := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: notify with disabled channel ───────────────────────────────

func TestProcessMetric_NotifierDisabledChannel(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "critical",
				ConditionLogic: "all",
				CooldownSec:    0,
				ChannelIDs:     []int64{channelID},
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 1300
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{{
				ID:      channelID,
				Name:    "Disabled Channel",
				Type:    "webhook",
				Enabled: false,
				Config:  map[string]any{"url": "http://localhost"},
			}}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	notifier := NewNotifier()
	engine := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}
