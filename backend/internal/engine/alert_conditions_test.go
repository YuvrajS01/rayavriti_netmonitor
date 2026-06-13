package engine

import (
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func fptr(v float64) *float64 { return &v }

func TestEvaluateCondition_Threshold_StatusEq(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		field    string
		op       string
		value    string
		metric   *models.Metric
		expected bool
	}{
		{
			name:  "response_time eq matches",
			field: "response_time", op: "eq", value: "100",
			metric:   &models.Metric{ResponseTime: fptr(100)},
			expected: true,
		},
		{
			name:  "response_time eq no match",
			field: "response_time", op: "eq", value: "100",
			metric:   &models.Metric{ResponseTime: fptr(200)},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cond := models.AlertRuleCondition{Type: "threshold", MetricField: tt.field, Operator: tt.op, Value: tt.value}
			result := EvaluateCondition(cond, tt.metric, "")
			assert.Equal(t, tt.expected, result.Result)
		})
	}
}

func TestEvaluateCondition_Threshold_ResponseTime_GT(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "1000"}

	// Above threshold
	metric := &models.Metric{ResponseTime: fptr(1500)}
	result := EvaluateCondition(cond, metric, "")
	assert.True(t, result.Result)
	assert.Equal(t, 1500.0, result.ActualValue)
	assert.Equal(t, 1000.0, result.Threshold)

	// Below threshold
	metric = &models.Metric{ResponseTime: fptr(500)}
	result = EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Threshold_ResponseTime_LT(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "lt", Value: "100"}
	metric := &models.Metric{ResponseTime: fptr(50)}
	result := EvaluateCondition(cond, metric, "")
	assert.True(t, result.Result)
}

func TestEvaluateCondition_Threshold_ResponseTime_GTE_Boundary(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "gte", Value: "100"}

	metric := &models.Metric{ResponseTime: fptr(100)}
	result := EvaluateCondition(cond, metric, "")
	assert.True(t, result.Result)

	metric = &models.Metric{ResponseTime: fptr(99.99)}
	result = EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Threshold_ResponseTime_LTE_Boundary(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "lte", Value: "100"}
	metric := &models.Metric{ResponseTime: fptr(100)}
	result := EvaluateCondition(cond, metric, "")
	assert.True(t, result.Result)
}

func TestEvaluateCondition_Threshold_AllNumericFields(t *testing.T) {
	t.Parallel()
	fields := []string{"packet_loss", "cpu_usage", "memory_usage", "bandwidth"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			cond := models.AlertRuleCondition{Type: "threshold", MetricField: field, Operator: "gt", Value: "50"}
			metric := &models.Metric{}
			switch field {
			case "packet_loss":
				metric.PacketLoss = fptr(75)
			case "cpu_usage":
				metric.CPUUsage = fptr(75)
			case "memory_usage":
				metric.MemoryUsage = fptr(75)
			case "bandwidth":
				metric.Bandwidth = fptr(75)
			}
			result := EvaluateCondition(cond, metric, "")
			assert.True(t, result.Result)
		})
	}
}

func TestEvaluateCondition_Threshold_NilField(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "100"}
	metric := &models.Metric{ResponseTime: nil}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Threshold_UnknownField(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "unknown_field", Operator: "gt", Value: "100"}
	metric := &models.Metric{ResponseTime: fptr(200)}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Threshold_InvalidThresholdType(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: "gt", Value: "not_a_number"}
	metric := &models.Metric{ResponseTime: fptr(200)}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Threshold_AlternateOperatorSyntax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		op       string
		expected bool
	}{
		{">", true},
		{"<", false},
		{">=", true},
		{"<=", false},
		{"==", false},
		{"!=", true},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			t.Parallel()
			cond := models.AlertRuleCondition{Type: "threshold", MetricField: "response_time", Operator: tt.op, Value: "100"}
			metric := &models.Metric{ResponseTime: fptr(150)}
			result := EvaluateCondition(cond, metric, "")
			assert.Equal(t, tt.expected, result.Result)
		})
	}
}

func TestEvaluateCondition_StateChange(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "state_change", MetricField: "status", Operator: "eq", Value: "down"}
	metric := &models.Metric{Status: "down"}
	result := EvaluateCondition(cond, metric, "up")
	assert.True(t, result.Result)

	metric = &models.Metric{Status: "up"}
	result = EvaluateCondition(cond, metric, "down")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_StateChange_Neq(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "state_change", MetricField: "status", Operator: "neq", Value: "up"}
	metric := &models.Metric{Status: "down"}
	result := EvaluateCondition(cond, metric, "up")
	assert.True(t, result.Result)
}

func TestEvaluateCondition_StateChange_NonStatusField(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "state_change", MetricField: "response_time", Operator: "eq", Value: "100"}
	metric := &models.Metric{ResponseTime: fptr(100)}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Absence(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "absence", MetricField: "status", Operator: "eq", Value: "down"}
	metric := &models.Metric{Status: "down"}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestEvaluateCondition_Anomaly(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "anomaly", MetricField: "response_time", Operator: "gt", Value: "2.5"}
	metric := &models.Metric{ResponseTime: fptr(500)}
	result := EvaluateCondition(cond, metric, "")
	// Anomaly is a stub that always returns false
	assert.False(t, result.Result)
	assert.Equal(t, 500.0, result.ActualValue)
}

func TestEvaluateCondition_UnknownType(t *testing.T) {
	t.Parallel()
	cond := models.AlertRuleCondition{Type: "unknown_type", MetricField: "status", Operator: "eq", Value: "down"}
	metric := &models.Metric{Status: "down"}
	result := EvaluateCondition(cond, metric, "")
	assert.False(t, result.Result)
}

func TestCompareFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		op        string
		actual    float64
		threshold float64
		expected  bool
	}{
		{"gt true", "gt", 10, 5, true},
		{"gt false equal", "gt", 5, 5, false},
		{"gt false less", "gt", 3, 5, false},
		{"lt true", "lt", 3, 5, true},
		{"lt false equal", "lt", 5, 5, false},
		{"gte true equal", "gte", 5, 5, true},
		{"gte true greater", "gte", 6, 5, true},
		{"lte true equal", "lte", 5, 5, true},
		{"lte true less", "lte", 4, 5, true},
		{"eq true", "eq", 5, 5, true},
		{"eq nearly equal", "eq", 5.00000000001, 5, true},
		{"neq false", "neq", 5, 5, false},
		{"neq true", "neq", 6, 5, true},
		{"gt alt", ">", 10, 5, true},
		{"lt alt", "<", 3, 5, true},
		{"gte alt", ">=", 5, 5, true},
		{"lte alt", "<=", 5, 5, true},
		{"eq alt", "==", 5, 5, true},
		{"neq alt", "!=", 6, 5, true},
		{"unknown op", "unknown", 5, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, compareFloat(tt.op, tt.actual, tt.threshold))
		})
	}
}
