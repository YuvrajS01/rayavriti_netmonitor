package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func (p *Postgres) UpsertHealthScore(ctx context.Context, score *models.DeviceHealthScoreRow) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO health_scores (device_id, score, label, trend, trend_delta, factors, issues, computed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (device_id) DO UPDATE SET
			score = EXCLUDED.score,
			label = EXCLUDED.label,
			trend = EXCLUDED.trend,
			trend_delta = EXCLUDED.trend_delta,
			factors = EXCLUDED.factors,
			issues = EXCLUDED.issues,
			computed_at = EXCLUDED.computed_at`,
		score.DeviceID, score.Score, score.Label, score.Trend, score.TrendDelta,
		score.Factors, score.Issues, score.ComputedAt)
	return err
}

func (p *Postgres) GetHealthScores(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT hs.device_id, hs.score, hs.label, hs.trend, hs.trend_delta,
		       hs.factors, hs.issues, hs.computed_at
		FROM health_scores hs
		ORDER BY hs.score ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []models.DeviceHealthScoreRow
	for rows.Next() {
		var s models.DeviceHealthScoreRow
		if err := rows.Scan(&s.DeviceID, &s.Score, &s.Label, &s.Trend, &s.TrendDelta,
			&s.Factors, &s.Issues, &s.ComputedAt); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	if scores == nil {
		scores = []models.DeviceHealthScoreRow{}
	}
	return scores, nil
}

func (p *Postgres) GetHealthScoreHistory(ctx context.Context, deviceID int64, hours int) ([]models.HealthHistoryPoint, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := p.pool.Query(ctx, `
		SELECT computed_at, score, label
		FROM health_score_history
		WHERE device_id = $1 AND computed_at >= $2
		ORDER BY computed_at ASC`, deviceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.HealthHistoryPoint
	for rows.Next() {
		var p models.HealthHistoryPoint
		var ts time.Time
		if err := rows.Scan(&ts, &p.Score, &p.Label); err != nil {
			return nil, err
		}
		p.Timestamp = ts.Format(time.RFC3339)
		points = append(points, p)
	}
	if points == nil {
		points = []models.HealthHistoryPoint{}
	}
	return points, nil
}

func (p *Postgres) GetNetworkHealthHistory(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := p.pool.Query(ctx, `
		SELECT date_trunc('minute', computed_at) AS bucket,
		       AVG(score) AS avg_score,
		       MODE() WITHIN GROUP (ORDER BY label) AS dominant_label
		FROM health_score_history
		WHERE computed_at >= $1
		GROUP BY bucket
		ORDER BY bucket ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.HealthHistoryPoint
	for rows.Next() {
		var p models.HealthHistoryPoint
		var ts time.Time
		if err := rows.Scan(&ts, &p.Score, &p.Label); err != nil {
			return nil, err
		}
		p.Timestamp = ts.Format(time.RFC3339)
		points = append(points, p)
	}
	if points == nil {
		points = []models.HealthHistoryPoint{}
	}
	return points, nil
}

func (p *Postgres) InsertHealthScoreHistory(ctx context.Context, entries []models.HealthHistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	for _, e := range entries {
		var factorsJSON []byte
		if e.Factors != nil {
			var err error
			factorsJSON, err = json.Marshal(e.Factors)
			if err != nil {
				factorsJSON = []byte("{}")
			}
		} else {
			factorsJSON = []byte("{}")
		}
		_, err := p.pool.Exec(ctx, `
			INSERT INTO health_score_history (device_id, score, label, factors, computed_at)
			VALUES ($1, $2, $3, $4, NOW())`,
			e.DeviceID, e.Score, e.Label, factorsJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) GetMetricsSince(ctx context.Context, deviceID int64, since time.Time) ([]models.Metric, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		       m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		       d.protocol, d.name
		FROM metrics m
		INNER JOIN devices d ON d.id = m.device_id
		WHERE m.device_id = $1 AND m.timestamp >= $2
		ORDER BY m.timestamp DESC`, deviceID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
}

func (p *Postgres) GetStatusFlaps(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT status FROM metrics
		WHERE device_id = $1 AND timestamp >= $2
		ORDER BY timestamp ASC`, deviceID, since)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var flaps int
	var prevStatus string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return 0, err
		}
		if prevStatus != "" && s != prevStatus {
			flaps++
		}
		prevStatus = s
	}
	return flaps, nil
}

func (p *Postgres) GetPortChanges(ctx context.Context, deviceID int64, since time.Time) (int, error) {
	var count int
	err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM port_scan_results
		WHERE device_id = $1 AND last_changed_at >= $2`, deviceID, since).Scan(&count)
	return count, err
}

func (p *Postgres) GetAlertsByRuleSince(ctx context.Context, ruleID int64, since time.Time) (int, error) {
	var count int
	err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM alerts
		WHERE rule_id = $1 AND created_at >= $2`, ruleID, since).Scan(&count)
	return count, err
}
