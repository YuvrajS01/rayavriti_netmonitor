package engine

import (
	"fmt"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// AlertCondition tests a single field of a Metric against a threshold.
type AlertCondition struct {
	Field string // status | response_time | packet_loss | cpu_usage | memory_usage | bandwidth
	Op    string // eq | ne | gt | lt | gte | lte
	Value any    // string for status, float64 for numeric fields
}

// Match returns true if the condition is satisfied by the given metric.
func (c AlertCondition) Match(m *models.Metric) bool {
	switch c.Field {
	case "status":
		v, ok := c.Value.(string)
		if !ok {
			return false
		}
		switch c.Op {
		case "eq":
			return m.Status == v
		case "ne":
			return m.Status != v
		}
	case "response_time":
		return matchFloat(c.Op, m.ResponseTime, c.Value)
	case "packet_loss":
		return matchFloat(c.Op, m.PacketLoss, c.Value)
	case "cpu_usage":
		return matchFloat(c.Op, m.CPUUsage, c.Value)
	case "memory_usage":
		return matchFloat(c.Op, m.MemoryUsage, c.Value)
	case "bandwidth":
		return matchFloat(c.Op, m.Bandwidth, c.Value)
	}
	return false
}

func matchFloat(op string, field *float64, threshold any) bool {
	if field == nil {
		return false
	}
	t, err := toFloat64(threshold)
	if err != nil {
		return false
	}
	switch op {
	case "gt", ">":
		return *field > t
	case "lt", "<":
		return *field < t
	case "gte", ">=":
		return *field >= t
	case "lte", "<=":
		return *field <= t
	case "eq", "==":
		return *field == t
	case "ne", "!=":
		return *field != t
	}
	return false
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
