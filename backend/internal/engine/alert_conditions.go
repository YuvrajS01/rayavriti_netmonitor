package engine

import (
	"math"
	"strconv"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ConditionResult holds the evaluation result for a single alert condition.
type ConditionResult struct {
	ConditionID         int64   `json:"condition_id"`
	Type                string  `json:"type"`
	Field               string  `json:"field"`
	Operator            string  `json:"operator"`
	Threshold           float64 `json:"threshold"`
	ActualValue         float64 `json:"actual_value"`
	Result              bool    `json:"result"`
	SustainedSeconds    int     `json:"sustained_seconds"`
	RequiredDurationSec int     `json:"required_duration_seconds"`
}

// EvaluateCondition evaluates a single alert rule condition against a metric.
func EvaluateCondition(condition models.AlertRuleCondition, metric *models.Metric, previousStatus string) ConditionResult {
	switch condition.Type {
	case "threshold":
		return evaluateThreshold(condition, metric)
	case "state_change":
		return evaluateStateChange(condition, metric, previousStatus)
	case "absence":
		// Absence is evaluated by the background loop, not on new metric arrival.
		return ConditionResult{ConditionID: condition.ID, Type: "absence", Result: false}
	case "anomaly":
		return evaluateAnomaly(condition, metric)
	default:
		return ConditionResult{ConditionID: condition.ID, Type: condition.Type, Result: false}
	}
}

func evaluateThreshold(condition models.AlertRuleCondition, metric *models.Metric) ConditionResult {
	result := ConditionResult{
		ConditionID: condition.ID,
		Type:        "threshold",
		Field:       condition.MetricField,
		Operator:    condition.Operator,
	}

	threshold, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil {
		return result
	}
	result.Threshold = threshold

	var actual *float64
	switch condition.MetricField {
	case "response_time":
		actual = metric.ResponseTime
	case "packet_loss":
		actual = metric.PacketLoss
	case "cpu_usage":
		actual = metric.CPUUsage
	case "memory_usage":
		actual = metric.MemoryUsage
	case "bandwidth":
		actual = metric.Bandwidth
	case "custom_value":
		actual = metric.CustomValue
	}

	if actual == nil {
		return result
	}
	result.ActualValue = *actual
	result.Result = compareFloat(condition.Operator, *actual, threshold)
	return result
}

func evaluateStateChange(condition models.AlertRuleCondition, metric *models.Metric, previousStatus string) ConditionResult {
	result := ConditionResult{
		ConditionID: condition.ID,
		Type:        "state_change",
		Field:       condition.MetricField,
		Operator:    condition.Operator,
	}

	if condition.MetricField != "status" {
		return result
	}

	currentStatus := metric.Status
	expectedValue := condition.Value

	switch condition.Operator {
	case "eq", "==":
		result.Result = currentStatus == expectedValue
	case "neq", "!=":
		result.Result = currentStatus != expectedValue
	}

	return result
}

func evaluateAnomaly(condition models.AlertRuleCondition, metric *models.Metric) ConditionResult {
	result := ConditionResult{
		ConditionID: condition.ID,
		Type:        "anomaly",
		Field:       condition.MetricField,
		Result:      false,
	}

	sensitivity, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil || sensitivity <= 0 {
		sensitivity = 2.5
	}
	result.Threshold = sensitivity

	// Get the actual metric value for the anomaly field
	var actual *float64
	switch condition.MetricField {
	case "response_time":
		actual = metric.ResponseTime
	case "packet_loss":
		actual = metric.PacketLoss
	case "cpu_usage":
		actual = metric.CPUUsage
	case "memory_usage":
		actual = metric.MemoryUsage
	case "bandwidth":
		actual = metric.Bandwidth
	}

	if actual == nil {
		return result
	}
	result.ActualValue = *actual

	// Anomaly detection requires historical baseline data.
	// For now, this is a stub that never fires. A future enhancement can compute
	// a rolling mean/stddev from recent metrics and compare against the sensitivity.
	_ = time.Now()
	result.Result = false
	return result
}

func compareFloat(op string, actual, threshold float64) bool {
	switch op {
	case "gt", ">":
		return actual > threshold
	case "lt", "<":
		return actual < threshold
	case "gte", ">=":
		return actual >= threshold
	case "lte", "<=":
		return actual <= threshold
	case "eq", "==":
		return math.Abs(actual-threshold) < 1e-9
	case "neq", "!=":
		return math.Abs(actual-threshold) >= 1e-9
	}
	return false
}
