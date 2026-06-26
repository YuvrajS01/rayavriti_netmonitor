package engine

import (
	"fmt"
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
	Description         string  `json:"description,omitempty"`
}

// AnomalyBaseline provides pre-computed statistics for anomaly detection.
type AnomalyBaseline struct {
	Mean        float64
	StdDev      float64
	SampleCount int
}

// EvaluateCondition evaluates a single alert rule condition against a metric.
// For anomaly conditions, pass baseline (may be nil if unavailable).
func EvaluateCondition(condition models.AlertRuleCondition, metric *models.Metric, previousStatus string, baseline *AnomalyBaseline) ConditionResult {
	switch condition.Type {
	case "threshold":
		return evaluateThreshold(condition, metric)
	case "state_change":
		return evaluateStateChange(condition, metric, previousStatus)
	case "absence":
		return evaluateAbsence(condition, metric)
	case "anomaly":
		return evaluateAnomaly(condition, metric, baseline)
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
		result.Description = fmt.Sprintf("%s: no value available", condition.MetricField)
		return result
	}
	result.ActualValue = *actual
	result.Result = compareFloat(condition.Operator, *actual, threshold)
	result.Description = fmt.Sprintf("%s %s %s %s (actual: %.2f)", condition.MetricField, opSymbol(condition.Operator), formatThreshold(condition), formatActual(*actual), *actual)
	return result
}

func evaluateStateChange(condition models.AlertRuleCondition, metric *models.Metric, previousStatus string) ConditionResult {
	result := ConditionResult{
		ConditionID: condition.ID,
		Type:        "state_change",
		Field:       condition.MetricField,
		Operator:    condition.Operator,
	}

	switch condition.MetricField {
	case "status":
		currentStatus := metric.Status
		expectedValue := condition.Value
		switch condition.Operator {
		case "eq", "==":
			result.Result = currentStatus == expectedValue
		case "neq", "!=":
			result.Result = currentStatus != expectedValue
		}
		if result.Result {
			result.Description = fmt.Sprintf("status changed: %s → %s (expected %s)", previousStatus, currentStatus, expectedValue)
		}

	case "port_state":
		result.ActualValue = 0
		result.Threshold = 0
		if metric.Details != nil {
			if changes, ok := metric.Details["port_changes"]; ok {
				if changeCount, ok := changes.(float64); ok {
					result.ActualValue = changeCount
				}
			}
		}
		threshold, err := strconv.ParseFloat(condition.Value, 64)
		if err != nil {
			threshold = 1
		}
		result.Threshold = threshold
		result.Result = result.ActualValue >= threshold
		if result.Result {
			result.Description = fmt.Sprintf("port state change detected: %.0f changes (threshold: %.0f)", result.ActualValue, threshold)
		}
	}

	return result
}

func evaluateAbsence(condition models.AlertRuleCondition, metric *models.Metric) ConditionResult {
	result := ConditionResult{
		ConditionID: condition.ID,
		Type:        "absence",
		Field:       condition.MetricField,
		Operator:    condition.Operator,
	}

	durationSec, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil || durationSec <= 0 {
		durationSec = 300
	}
	result.Threshold = durationSec

	// Absence is evaluated by the background loop using the metric's timestamp.
	// If we have a metric, check staleness against the configured duration.
	if metric != nil {
		staleness := time.Since(metric.Timestamp).Seconds()
		result.ActualValue = staleness
		result.Result = staleness > durationSec
		if result.Result {
			result.Description = fmt.Sprintf("no data for %.0fs (threshold: %.0fs)", staleness, durationSec)
		}
	}

	return result
}

func evaluateAnomaly(condition models.AlertRuleCondition, metric *models.Metric, baseline *AnomalyBaseline) ConditionResult {
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
		result.Description = fmt.Sprintf("%s: no value available for anomaly check", condition.MetricField)
		return result
	}
	result.ActualValue = *actual

	if baseline == nil || baseline.SampleCount < 10 {
		result.Description = fmt.Sprintf("%s: insufficient baseline data (%d samples)", condition.MetricField, baselineSampleCount(baseline))
		return result
	}

	zScore := (*actual - baseline.Mean) / baseline.StdDev
	result.Result = math.Abs(zScore) > sensitivity
	if result.Result {
		result.Description = fmt.Sprintf("%s anomaly: %.2f (z-score: %.2f, mean: %.2f, stddev: %.2f)", condition.MetricField, *actual, zScore, baseline.Mean, baseline.StdDev)
	} else {
		result.Description = fmt.Sprintf("%s normal: %.2f (z-score: %.2f)", condition.MetricField, *actual, zScore)
	}
	return result
}

func baselineSampleCount(b *AnomalyBaseline) int {
	if b == nil {
		return 0
	}
	return b.SampleCount
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

func opSymbol(op string) string {
	switch op {
	case "gt", ">":
		return ">"
	case "lt", "<":
		return "<"
	case "gte", ">=":
		return ">="
	case "lte", "<=":
		return "<="
	case "eq", "==":
		return "=="
	case "neq", "!=":
		return "!="
	}
	return op
}

func formatThreshold(condition models.AlertRuleCondition) string {
	v, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil {
		return condition.Value
	}
	return fmt.Sprintf("%.2f", v)
}

func formatActual(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
