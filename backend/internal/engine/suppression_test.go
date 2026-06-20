package engine

import (
	"context"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/campus"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var firstMet600 = time.Now().Add(-600 * time.Second)

// ── mock suppression checkers ───────────────────────────────────────────────

type mockSuppressionChecker struct {
	result *campus.SuppressionResult
	err    error
}

func (m *mockSuppressionChecker) CheckSuppression(_ context.Context, _ int64) (*campus.SuppressionResult, error) {
	return m.result, m.err
}

type mockMaintenanceChecker struct {
	status *campus.MaintenanceStatus
	err    error
}

func (m *mockMaintenanceChecker) IsUnderMaintenance(_ context.Context, _ int64, _ *int64, _ string) (*campus.MaintenanceStatus, error) {
	return m.status, m.err
}

type mockSuppressedRecorder struct {
	calls []recordCall
}

type recordCall struct {
	deviceID int64
	ruleID   *int64
	reason   string
	rootID   *int64
}

func (m *mockSuppressedRecorder) RecordSuppressedAlert(_ context.Context, deviceID int64, ruleID *int64, reason string, rootCauseDeviceID *int64) error {
	m.calls = append(m.calls, recordCall{deviceID, ruleID, reason, rootCauseDeviceID})
	return nil
}

// ── topology suppression ────────────────────────────────────────────────────

func TestFireAlert_SuppressedByTopology(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
	}
	recorder := &mockSuppressedRecorder{}
	checker := &mockSuppressionChecker{
		result: &campus.SuppressionResult{
			ShouldSuppress:  true,
			Reason:          "parent_down",
			RootCauseDevice: &campus.DeviceNode{DeviceID: 10, Name: "Router", Host: "10.0.0.1", Status: "down"},
			Message:         "Suppressed: parent device Router (10.0.0.1) is down",
		},
	}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(checker),
		WithSuppressedAlertRecorder(recorder),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Switch", IPAddress: "10.0.0.2"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)

	// Alert should NOT have been created, but suppression should be recorded
	assert.Len(t, recorder.calls, 1)
	assert.Equal(t, "parent_down", recorder.calls[0].reason)
}

func TestFireAlert_NoSuppression_AllowAlert(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 100
			return a, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	checker := &mockSuppressionChecker{
		result: &campus.SuppressionResult{ShouldSuppress: false, Message: "No suppression"},
	}
	engine := NewAlertEngine(db, nil, nil, WithSuppressionChecker(checker))

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── maintenance suppression ─────────────────────────────────────────────────

func TestFireAlert_SuppressedByMaintenance(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
	}
	recorder := &mockSuppressedRecorder{}
	topoChecker := &mockSuppressionChecker{
		result: &campus.SuppressionResult{ShouldSuppress: false},
	}
	maintChecker := &mockMaintenanceChecker{
		status: &campus.MaintenanceStatus{
			UnderMaintenance: true,
			SuppressAlerts:   true,
			SuppressNotify:   true,
			Window:           &campus.MaintenanceWindow{Name: "Scheduled Update"},
		},
	}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(topoChecker),
		WithMaintenanceChecker(maintChecker),
		WithSuppressedAlertRecorder(recorder),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)

	// Alert suppressed, recorder should have the entry
	assert.Len(t, recorder.calls, 1)
	assert.Equal(t, "maintenance_window", recorder.calls[0].reason)
}

func TestFireAlert_MaintenanceNotSuppressAlerts(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 200
			return a, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	topoChecker := &mockSuppressionChecker{
		result: &campus.SuppressionResult{ShouldSuppress: false},
	}
	maintChecker := &mockMaintenanceChecker{
		status: &campus.MaintenanceStatus{
			UnderMaintenance: true,
			SuppressAlerts:   false, // maintenance active but NOT suppressing alerts
		},
	}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(topoChecker),
		WithMaintenanceChecker(maintChecker),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── suppression check error handling ────────────────────────────────────────

func TestFireAlert_SuppressionCheckError_StillFires(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 300
			return a, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	checker := &mockSuppressionChecker{err: assert.AnError}
	engine := NewAlertEngine(db, nil, nil, WithSuppressionChecker(checker))

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

func TestFireAlert_MaintenanceCheckError_StillFires(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 400
			return a, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	topoChecker := &mockSuppressionChecker{result: &campus.SuppressionResult{ShouldSuppress: false}}
	maintChecker := &mockMaintenanceChecker{err: assert.AnError}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(topoChecker),
		WithMaintenanceChecker(maintChecker),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── suppressed alert recording ──────────────────────────────────────────────

func TestSuppressedAlertRecorded_WithRootCause(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	recorder := &mockSuppressedRecorder{}
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
	}
	checker := &mockSuppressionChecker{
		result: &campus.SuppressionResult{
			ShouldSuppress:  true,
			Reason:          "parent_down",
			RootCauseDevice: &campus.DeviceNode{DeviceID: 10},
		},
	}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(checker),
		WithSuppressedAlertRecorder(recorder),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Switch", IPAddress: "10.0.0.2"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	_ = engine.ProcessMetric(context.Background(), device, metric, "up")
	require.Len(t, recorder.calls, 1)
	assert.Equal(t, int64(1), recorder.calls[0].deviceID)
	assert.Equal(t, &ruleID, recorder.calls[0].ruleID)
	assert.Equal(t, "parent_down", recorder.calls[0].reason)
	assert.Equal(t, int64Ptr(10), recorder.calls[0].rootID)
}

func TestSuppressedAlertRecorded_Maintenance(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	recorder := &mockSuppressedRecorder{}
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
	}
	topoChecker := &mockSuppressionChecker{result: &campus.SuppressionResult{ShouldSuppress: false}}
	maintChecker := &mockMaintenanceChecker{
		status: &campus.MaintenanceStatus{
			UnderMaintenance: true,
			SuppressAlerts:   true,
		},
	}
	engine := NewAlertEngine(db, nil, nil,
		WithSuppressionChecker(topoChecker),
		WithMaintenanceChecker(maintChecker),
		WithSuppressedAlertRecorder(recorder),
	)

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	_ = engine.ProcessMetric(context.Background(), device, metric, "up")
	require.Len(t, recorder.calls, 1)
	assert.Equal(t, "maintenance_window", recorder.calls[0].reason)
	assert.Nil(t, recorder.calls[0].rootID)
}

// ── no checkers configured ──────────────────────────────────────────────────

func TestFireAlert_NoCheckers_AlertAllowed(t *testing.T) {
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
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &firstMet600,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 500
			return a, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	engine := NewAlertEngine(db, nil, nil) // no checkers

	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server", IPAddress: "10.0.0.1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := engine.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}
