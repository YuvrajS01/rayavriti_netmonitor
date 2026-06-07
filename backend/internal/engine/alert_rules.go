package engine

import (
	"context"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// AlertRule defines when an alert should fire.
type AlertRule struct {
	ID         int64
	Name       string
	DeviceID   *int64 // nil = apply to all devices
	Severity   string
	Conditions []AlertCondition
	Message    string
	Enabled    bool
}

// Evaluate returns true if all conditions in the rule match the given metric.
func (r *AlertRule) Evaluate(ctx context.Context, m *models.Metric) bool {
	if !r.Enabled {
		return false
	}
	if r.DeviceID != nil && *r.DeviceID != m.DeviceID {
		return false
	}
	for _, c := range r.Conditions {
		if !c.Match(m) {
			return false
		}
	}
	return true
}

// DefaultRules returns the built-in alert rules shipped with the application.
func DefaultRules() []AlertRule {
	return []AlertRule{
		{
			ID:       1,
			Name:     "Device Down",
			Severity: "critical",
			Message:  "Device is unreachable",
			Enabled:  true,
			Conditions: []AlertCondition{
				{Field: "status", Op: "eq", Value: "down"},
			},
		},
		{
			ID:       2,
			Name:     "High Packet Loss",
			Severity: "warning",
			Message:  "Packet loss exceeded 20%",
			Enabled:  true,
			Conditions: []AlertCondition{
				{Field: "packet_loss", Op: "gt", Value: 20.0},
			},
		},
		{
			ID:       3,
			Name:     "Slow Response",
			Severity: "warning",
			Message:  "Response time exceeded 1000ms",
			Enabled:  true,
			Conditions: []AlertCondition{
				{Field: "response_time", Op: "gt", Value: 1000.0},
			},
		},
		{
			ID:       4,
			Name:     "High CPU",
			Severity: "warning",
			Message:  "CPU usage exceeded 90%",
			Enabled:  true,
			Conditions: []AlertCondition{
				{Field: "cpu_usage", Op: "gt", Value: 90.0},
			},
		},
		{
			ID:       5,
			Name:     "High Memory",
			Severity: "warning",
			Message:  "Memory usage exceeded 95%",
			Enabled:  true,
			Conditions: []AlertCondition{
				{Field: "memory_usage", Op: "gt", Value: 95.0},
			},
		},
	}
}
