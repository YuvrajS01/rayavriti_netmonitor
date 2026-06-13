package engine

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
)

type AnomalyEngine struct {
	db     database.Database
	logger *slog.Logger
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewAnomalyEngine(db database.Database, logger *slog.Logger) *AnomalyEngine {
	if logger == nil {
		logger = slog.Default()
	}
	return &AnomalyEngine{db: db, logger: logger}
}

func (e *AnomalyEngine) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.computeHealthScores(ctx); err != nil {
					e.logger.Error("health score computation failed", "error", err)
				}
			}
		}
	}()
}

func (e *AnomalyEngine) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	e.wg.Wait()
}

type DeviceHealthScore struct {
	DeviceID   int64   `json:"deviceId"`
	DeviceName string  `json:"deviceName"`
	Score      float64 `json:"score"`
	Status     string  `json:"status"`
	Factors    []string `json:"factors,omitempty"`
}

func (e *AnomalyEngine) computeHealthScores(ctx context.Context) error {
	devices, err := e.db.GetDevices(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil
	}

	latestMetrics, err := e.db.GetLatestMetrics(ctx)
	if err != nil {
		return err
	}
	metricMap := map[int64]struct {
		ResponseTime float64
		PacketLoss   float64
		Status       string
		Timestamp    time.Time
	}{}
	for _, m := range latestMetrics {
		entry := metricMap[m.DeviceID]
		entry.Status = m.Status
		entry.Timestamp = m.Timestamp
		if m.ResponseTime != nil {
			entry.ResponseTime = *m.ResponseTime
		}
		if m.PacketLoss != nil {
			entry.PacketLoss = *m.PacketLoss
		}
		metricMap[m.DeviceID] = entry
	}

	alertCounts, err := e.db.GetAlertCounts(ctx)
	if err != nil {
		return err
	}

	scores := make([]DeviceHealthScore, 0, len(devices))
	now := time.Now()
	for _, d := range devices {
		score := 100.0
		var factors []string

		if d.Status == "down" {
			score = 0
			factors = append(factors, "device is down")
		} else if d.Status == "warning" {
			score = 50
			factors = append(factors, "device in warning state")
		}

		if m, ok := metricMap[d.ID]; ok {
			if m.ResponseTime > 2000 {
				score = math.Min(score, 30)
				factors = append(factors, "high response time")
			} else if m.ResponseTime > 1000 {
				score = math.Min(score, 60)
				factors = append(factors, "elevated response time")
			}

			if m.PacketLoss > 10 {
				score = math.Min(score, 20)
				factors = append(factors, "high packet loss")
			} else if m.PacketLoss > 5 {
				score = math.Min(score, 50)
				factors = append(factors, "moderate packet loss")
			}

			staleness := now.Sub(m.Timestamp)
			if staleness > 30*time.Minute {
				score = math.Min(score, 40)
				factors = append(factors, "stale metrics")
			} else if staleness > 10*time.Minute {
				score = math.Min(score, 70)
				factors = append(factors, "metrics slightly stale")
			}
		} else {
			score = math.Min(score, 60)
			factors = append(factors, "no recent metrics")
		}

		if !d.Enabled {
			score = math.Min(score, 50)
			factors = append(factors, "device disabled")
		}

		severity := "healthy"
		switch {
		case score < 30:
			severity = "critical"
		case score < 60:
			severity = "warning"
		case score < 80:
			severity = "degraded"
		}

		scores = append(scores, DeviceHealthScore{
			DeviceID:   d.ID,
			DeviceName: d.Name,
			Score:      score,
			Status:     severity,
			Factors:    factors,
		})
	}

	e.logger.Info("health scores computed",
		"device_count", len(scores),
		"active_alerts", alertCounts.Active,
		"acknowledged_alerts", alertCounts.Acknowledged,
	)

	return nil
}
