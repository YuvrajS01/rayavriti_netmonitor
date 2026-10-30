package engine

import (
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestStateChangeDescription(t *testing.T) {
	cond := models.AlertRuleCondition{MetricField: "status", Operator: "eq", Value: "down"}
	tests := []struct{ prev, want string }{
		{"up", "status changed: up → down"},
		{"down", "status is down"},
		{"", "status is down"},
	}
	for _, tt := range tests {
		got := evaluateStateChange(cond, &models.Metric{Status: "down"}, tt.prev)
		if !got.Result || got.Description != tt.want {
			t.Errorf("prev=%q: got (%v, %q), want %q", tt.prev, got.Result, got.Description, tt.want)
		}
	}
}
