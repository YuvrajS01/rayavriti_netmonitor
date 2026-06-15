package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type HealthScorer struct {
	db     database.Database
	logger *slog.Logger
}

func NewHealthScorer(db database.Database, logger *slog.Logger) *HealthScorer {
	if logger == nil {
		logger = slog.Default()
	}
	return &HealthScorer{db: db, logger: logger}
}

type healthFactor struct {
	Score   float64 `json:"score"`
	Weight  float64 `json:"weight"`
	Penalty float64 `json:"penalty"`
}

type healthFactors struct {
	Availability healthFactor `json:"availability"`
	Latency      healthFactor `json:"latency"`
	Alerts       healthFactor `json:"alerts"`
	Stability    healthFactor `json:"stability"`
	Ports        healthFactor `json:"ports"`
}

type healthIssue struct {
	Severity string `json:"severity"`
	Type     string `json:"type"`
	Message  string `json:"message"`
}

func (hs *HealthScorer) ComputeAll(ctx context.Context) error {
	devices, err := hs.db.GetEnabledDevices(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	oneDayAgo := time.Now().Add(-24 * time.Hour)

	alertCounts, err := hs.db.GetAlertCounts(ctx)
	if err != nil {
		return err
	}

	var entries []models.HealthHistoryEntry

	for i := range devices {
		d := &devices[i]
		score, factors, issues := hs.computeDeviceScore(ctx, d, oneHourAgo, oneDayAgo, alertCounts)

		label := scoreToLabel(score)

		// Get previous score for trend.
		prevScores, err := hs.db.GetHealthScoreHistory(ctx, d.ID, 2)
		trend := "stable"
		trendDelta := 0.0
		if err == nil && len(prevScores) > 0 {
			for _, p := range prevScores {
				if p.Score != nil {
					trendDelta = score - *p.Score
					break
				}
			}
			if trendDelta >= 5 {
				trend = "improving"
			} else if trendDelta <= -5 {
				trend = "degrading"
			}
		}

		factorsJSON, _ := json.Marshal(factors)
		if issues == nil {
			issues = []healthIssue{}
		}
		issuesJSON, _ := json.Marshal(issues)

		err = hs.db.UpsertHealthScore(ctx, &models.DeviceHealthScoreRow{
			DeviceID:   d.ID,
			Score:      score,
			Label:      label,
			Trend:      trend,
			TrendDelta: trendDelta,
			Factors:    factorsJSON,
			Issues:     issuesJSON,
			ComputedAt: time.Now(),
		})
		if err != nil {
			hs.logger.Warn("Failed to upsert health score", "device_id", d.ID, "error", err)
		}

		entries = append(entries, models.HealthHistoryEntry{
			DeviceID: d.ID,
			Score:    score,
			Label:    label,
			Factors:  factors,
		})
	}

	if err := hs.db.InsertHealthScoreHistory(ctx, entries); err != nil {
		hs.logger.Warn("Failed to insert health score history", "error", err)
	}

	// Refresh anomaly baselines.
	hs.logger.Info("health scores computed", "device_count", len(devices))
	return nil
}

func (hs *HealthScorer) computeDeviceScore(ctx context.Context, d *models.Device, sinceOneHour, sinceOneDay time.Time, alertCounts models.AlertCounts) (float64, healthFactors, []healthIssue) {
	var issues []healthIssue

	// ── Availability (30%) ────────────────────────────────────────────────────
	availScore := 100.0
	metrics, err := hs.db.GetMetricsSince(ctx, d.ID, sinceOneHour)
	if err == nil && len(metrics) > 0 {
		downCount := 0
		for _, m := range metrics {
			if m.Status == "down" {
				downCount++
			}
		}
		availScore = float64(len(metrics)-downCount) / float64(len(metrics)) * 100
	} else if d.Status == "down" {
		availScore = 0
		issues = append(issues, healthIssue{Severity: "critical", Type: "availability", Message: "Device is offline"})
	} else if d.Status == "warning" {
		availScore = 50
		issues = append(issues, healthIssue{Severity: "warning", Type: "availability", Message: "Device in warning state"})
	}

	// ── Latency (25%) ─────────────────────────────────────────────────────────
	latencyScore := 100.0
	if len(metrics) > 0 {
		var rtValues []float64
		for _, m := range metrics {
			if m.ResponseTime != nil {
				rtValues = append(rtValues, *m.ResponseTime)
			}
		}
		if len(rtValues) > 0 {
			p95 := percentile(rtValues, 95)
			// 100 at ≤50ms, 0 at ≥5000ms, linear scale
			latencyScore = 100 - (p95-50)/(5000-50)*100
			latencyScore = math.Max(0, math.Min(100, latencyScore))
			if p95 > 1000 {
				issues = append(issues, healthIssue{Severity: "warning", Type: "latency", Message: "High latency detected"})
			}
		}
	}

	// ── Alert Load (20%) ──────────────────────────────────────────────────────
	alertScore := 100.0
	if alertCounts.Active > 0 {
		alertScore = 100 - float64(alertCounts.Active)*25
		alertScore = math.Max(0, alertScore)
	}
	deviceAlerts := 0
	for _, m := range metrics {
		_ = m
	}
	_ = deviceAlerts
	if alertScore < 50 {
		issues = append(issues, healthIssue{Severity: "warning", Type: "alerts", Message: "Multiple active alerts"})
	}

	// ── Stability (15%) ───────────────────────────────────────────────────────
	stabilityScore := 100.0
	flaps, err := hs.db.GetStatusFlaps(ctx, d.ID, sinceOneHour)
	if err == nil {
		if flaps >= 10 {
			stabilityScore = 0
			issues = append(issues, healthIssue{Severity: "warning", Type: "stability", Message: "Device status flapping"})
		} else if flaps > 0 {
			stabilityScore = 100 - float64(flaps)*10
		}
	}

	// ── Port Security (10%) ───────────────────────────────────────────────────
	portScore := 100.0
	portChanges, err := hs.db.GetPortChanges(ctx, d.ID, sinceOneDay)
	if err == nil {
		if portChanges >= 5 {
			portScore = 0
			issues = append(issues, healthIssue{Severity: "warning", Type: "ports", Message: "Unexpected port changes"})
		} else if portChanges > 0 {
			portScore = 100 - float64(portChanges)*20
		}
	}

	factors := healthFactors{
		Availability: healthFactor{Score: availScore, Weight: 0.30, Penalty: 100 - availScore},
		Latency:      healthFactor{Score: latencyScore, Weight: 0.25, Penalty: 100 - latencyScore},
		Alerts:       healthFactor{Score: alertScore, Weight: 0.20, Penalty: 100 - alertScore},
		Stability:    healthFactor{Score: stabilityScore, Weight: 0.15, Penalty: 100 - stabilityScore},
		Ports:        healthFactor{Score: portScore, Weight: 0.10, Penalty: 100 - portScore},
	}

	totalScore := availScore*0.30 + latencyScore*0.25 + alertScore*0.20 + stabilityScore*0.15 + portScore*0.10
	totalScore = math.Round(totalScore*10) / 10

	return totalScore, factors, issues
}

func scoreToLabel(score float64) string {
	switch {
	case score < 40:
		return "critical"
	case score < 60:
		return "risk"
	case score < 80:
		return "watch"
	default:
		return "healthy"
	}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
