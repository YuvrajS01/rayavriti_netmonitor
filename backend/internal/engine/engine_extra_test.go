package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pendingState returns a mock that returns "pending" state with old FirstMetAt,
// causing the rule to transition to "firing" and invoke fireAlert → sendNotifications.
func pendingStateWithNotifier(
	ruleID int64,
	channelID int64,
	notifErr error,
) *mockDB {
	return &mockDB{
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
			old := time.Now().Add(-600 * time.Second)
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &old,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 100
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			if notifErr != nil {
				return nil, notifErr
			}
			return []models.NotificationChannel{{
				ID:      channelID,
				Name:    "Test Channel",
				Type:    "webhook",
				Enabled: true,
				Config:  map[string]any{"url": "http://127.0.0.1:1"},
			}}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
}

// ── sendNotifications: webhook with invalid URL ───────────────────────────────

func TestSendNotifications_WebhookInvalidURL(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: webhook success ────────────────────────────────────────

func TestSendNotifications_WebhookSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Test Webhook",
			Type:    "webhook",
			Enabled: true,
			Config:  map[string]any{"url": server.URL},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: Slack channel ──────────────────────────────────────────

func TestSendNotifications_SlackChannel(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Slack Channel",
			Type:    "slack",
			Enabled: true,
			Config:  map[string]any{"webhook_url": server.URL},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: Slack error ────────────────────────────────────────────

func TestSendNotifications_SlackError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Slack Channel",
			Type:    "slack",
			Enabled: true,
			Config:  map[string]any{"webhook_url": server.URL},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: email channel (missing config → error) ─────────────────

func TestSendNotifications_EmailMissingConfig(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Email Channel",
			Type:    "email",
			Enabled: true,
			Config:  map[string]any{},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: no channels configured ─────────────────────────────────

func TestSendNotifications_NoChannels(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := pendingStateWithNotifier(ruleID, 0, nil)
	db.getNotificationChannelsFn = nil
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: nil notifier ───────────────────────────────────────────

func TestSendNotifications_NilNotifier(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	eng := NewAlertEngine(db, nil, nil) // nil notifier
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── recordHistory: success and error paths ────────────────────────────────────

func TestRecordHistory_Success(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	eng := NewAlertEngine(db, nil, nil)
	eng.recordHistory(context.Background(), 1, 2, "fired", "system", map[string]any{"key": "value"})
}

func TestRecordHistory_Error(t *testing.T) {
	t.Parallel()
	db := &mockDB{
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return fmt.Errorf("db error")
		},
	}
	eng := NewAlertEngine(db, nil, nil)
	eng.recordHistory(context.Background(), 1, 2, "fired", "system", map[string]any{"key": "value"})
}

// ── handleConditionCleared: auto-resolve with update error ────────────────────

func TestHandleConditionCleared_AutoResolve_UpdateError(t *testing.T) {
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
			return fmt.Errorf("update failed")
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── handleConditionCleared: notified state with auto-resolve ──────────────────

func TestHandleConditionCleared_Notified_AutoResolve(t *testing.T) {
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
				State:         "notified",
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── handleConditionCleared: acknowledged state with auto-resolve ──────────────

func TestHandleConditionCleared_Acknowledged_AutoResolve(t *testing.T) {
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
				State:         "acknowledged",
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 50.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, Status: "up"}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── evaluateRule with "any" logic, both conditions met ────────────────────────

func TestProcessMetric_AnyLogic_BothConditionsMet(t *testing.T) {
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
				CooldownSec:    0,
				Conditions: []models.AlertRuleCondition{
					{ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"},
					{ID: 2, Type: "threshold", MetricField: "memory_usage", Operator: "gt", Value: "80"},
				},
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
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	memVal := 92.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, MemoryUsage: &memVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── evaluateRule with "all" logic, partial match (no trigger) ─────────────────

func TestProcessMetric_AllLogic_PartialMatch(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "All conditions",
				Enabled:        true,
				Severity:       "warning",
				ConditionLogic: "all",
				CooldownSec:    0,
				Conditions: []models.AlertRuleCondition{
					{ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"},
					{ID: 2, Type: "threshold", MetricField: "memory_usage", Operator: "gt", Value: "90"},
				},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	memVal := 50.0 // below threshold
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, MemoryUsage: &memVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ruleAppliesToDevice: device scope with nil DeviceID ───────────────────────

func TestRuleAppliesToDevice_DeviceScope_NilDeviceID(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "device", DeviceID: nil}
	device := &models.Device{ID: 5}
	assert.False(t, RuleAppliesToDevice(rule, device))
}

// ── ruleAppliesToDevice: unknown scope type ───────────────────────────────────

func TestRuleAppliesToDevice_UnknownScope(t *testing.T) {
	t.Parallel()
	rule := &models.AlertRule{ScopeType: "unknown_scope"}
	device := &models.Device{ID: 1}
	assert.True(t, RuleAppliesToDevice(rule, device))
}

// ── fireAlert: createAlert fails ──────────────────────────────────────────────

func TestFireAlert_CreateAlertFails(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "critical",
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
			return nil, fmt.Errorf("create alert failed")
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: multiple conditions in a rule ──────────────────────────────

func TestProcessMetric_MultipleConditions_AllMet(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "Multi-condition",
				Enabled:        true,
				Severity:       "critical",
				ConditionLogic: "all",
				CooldownSec:    0,
				Conditions: []models.AlertRuleCondition{
					{ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80"},
					{ID: 2, Type: "threshold", MetricField: "memory_usage", Operator: "gt", Value: "80"},
					{ID: 3, Type: "state_change", MetricField: "status", Operator: "eq", Value: "up"},
				},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			return nil, assert.AnError
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 900
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	memVal := 92.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal, MemoryUsage: &memVal, Status: "up"}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── ProcessMetric: sendNotifications with multiple channels ───────────────────

func TestSendNotifications_MultipleChannels(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ruleID := int64(1)
	channelID1 := int64(10)
	channelID2 := int64(20)
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{
				ID:             ruleID,
				Name:           "High CPU",
				Enabled:        true,
				Severity:       "critical",
				ConditionLogic: "all",
				CooldownSec:    0,
				ChannelIDs:     []int64{channelID1, channelID2},
				Conditions: []models.AlertRuleCondition{{
					ID: 1, Type: "threshold", MetricField: "cpu_usage", Operator: "gt", Value: "80",
				}},
			}}, nil
		},
		getAlertRuleStateFn: func(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
			old := time.Now().Add(-600 * time.Second)
			return &models.AlertRuleState{
				RuleID:     ruleID,
				DeviceID:   1,
				State:      "pending",
				FirstMetAt: &old,
			}, nil
		},
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			a.ID = 1000
			return a, nil
		},
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{
				{ID: channelID1, Name: "Webhook 1", Type: "webhook", Enabled: true, Config: map[string]any{"url": server.URL}},
				{ID: channelID2, Name: "Webhook 2", Type: "webhook", Enabled: true, Config: map[string]any{"url": server.URL}},
			}, nil
		},
		upsertAlertRuleStateFn: func(ctx context.Context, s *models.AlertRuleState) error {
			return nil
		},
		recordAlertHistoryFn: func(ctx context.Context, h *models.AlertHistory) error {
			return nil
		},
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: Slack without webhook_url ──────────────────────────────

func TestSendNotifications_SlackNoURL(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Slack Channel",
			Type:    "slack",
			Enabled: true,
			Config:  map[string]any{},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: unsupported channel type ───────────────────────────────

func TestSendNotifications_UnsupportedChannelType(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "SMS Channel",
			Type:    "sms",
			Enabled: true,
			Config:  map[string]any{},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── EvaluateCondition: anomaly with nil value ──────────────────────────────────

func TestEvaluateCondition_Anomaly_NilValue_Extra(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{
		ID:          1,
		Type:        "anomaly",
		MetricField: "response_time",
		Value:       "invalid",
	}
	metric := &models.Metric{}
	result := EvaluateCondition(cond, metric, "up", nil)
	assert.Equal(t, "anomaly", result.Type)
	assert.False(t, result.Result)
}

// ── handleConditionMet: pending with no FirstMetAt ────────────────────────────

func TestHandleConditionMet_PendingNoFirstMet(t *testing.T) {
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
				// FirstMetAt is nil
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
	eng := NewAlertEngine(db, nil, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── fireAlert: with hub broadcasting ──────────────────────────────────────────

func TestFireAlert_WithHub(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	db := pendingStateWithNotifier(ruleID, 0, nil)
	db.getNotificationChannelsFn = nil
	hub := newTestHub()
	eng := NewAlertEngine(db, hub, nil)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: channel load error ─────────────────────────────────────

func TestSendNotifications_ChannelLoadError(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, fmt.Errorf("db error"))
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

// ── sendNotifications: disabled channel ───────────────────────────────────────

func TestSendNotifications_DisabledChannel(t *testing.T) {
	t.Parallel()
	ruleID := int64(1)
	channelID := int64(10)
	db := pendingStateWithNotifier(ruleID, channelID, nil)
	db.getNotificationChannelsFn = func(ctx context.Context) ([]models.NotificationChannel, error) {
		return []models.NotificationChannel{{
			ID:      channelID,
			Name:    "Disabled Channel",
			Type:    "webhook",
			Enabled: false,
			Config:  map[string]any{"url": "http://localhost"},
		}}, nil
	}
	notifier := NewNotifier()
	eng := NewAlertEngine(db, nil, notifier)
	cpuVal := 95.0
	device := &models.Device{ID: 1, Name: "Server-1"}
	metric := &models.Metric{DeviceID: 1, CPUUsage: &cpuVal}

	err := eng.ProcessMetric(context.Background(), device, metric, "up")
	require.NoError(t, err)
}

func newTestHub() *websocket.Hub {
	return websocket.NewHub("test-secret", nil, nil)
}
