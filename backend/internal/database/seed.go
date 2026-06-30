package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// SeedDefaults creates the admin user, default devices, sensors, and alert rules
// if they don't already exist. Safe to call on every startup.
func SeedDefaults(ctx context.Context, db Database, adminUsername, adminPasswordHash string) error {
	pg, ok := db.(*Postgres)
	if !ok {
		return fmt.Errorf("seed only supports Postgres")
	}

	// ── 1. Update admin password hash (V11 inserted a placeholder) ──
	_, err := pg.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1 WHERE username=$2 AND password_hash='PLACEHOLDER'`,
		adminPasswordHash, adminUsername)
	if err != nil {
		return fmt.Errorf("seed admin password: %w", err)
	}
	slog.Info("Seed: admin user password updated", "username", adminUsername)

	// ── 2. Seed default devices ──
	if err := seedDefaultDevices(ctx, pg); err != nil {
		return fmt.Errorf("seed devices: %w", err)
	}

	// ── 3. Seed default sensors for all devices ──
	if err := seedDefaultSensors(ctx, pg); err != nil {
		return fmt.Errorf("seed sensors: %w", err)
	}

	// ── 4. Seed default alert rules with conditions ──
	if err := seedDefaultAlertRules(ctx, pg); err != nil {
		return fmt.Errorf("seed alert rules: %w", err)
	}

	return nil
}

// seedDefaultDevices creates 5 default monitoring targets if no devices exist.
func seedDefaultDevices(ctx context.Context, pg *Postgres) error {
	var count int64
	if err := pg.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		slog.Debug("Seed: devices already exist, skipping", "count", count)
		return nil
	}

	defaults := []struct {
		name     string
		ip       string
		protocol string
		interval int
		tags     []string
		httpPath string
	}{
		{name: "Gateway", ip: "1.1.1.1", protocol: "ping", interval: 30, tags: []string{"network", "infrastructure"}},
		{name: "Google DNS", ip: "8.8.8.8", protocol: "ping", interval: 30, tags: []string{"network", "dns"}},
		{name: "Rayavriti Website", ip: "rayavriti.com", protocol: "https", interval: 60, tags: []string{"service", "web"}, httpPath: "/"},
		{name: "Localhost API Port", ip: "127.0.0.1", protocol: "port", interval: 30, tags: []string{"service", "local"}},
		{name: "Local System", ip: "localhost", protocol: "system", interval: 20, tags: []string{"server", "local"}},
	}

	for _, d := range defaults {
		tags, _ := json.Marshal(d.tags)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO devices (name, ip_address, protocol, interval_sec, tags, http_path, enabled, status)
			VALUES ($1, $2, $3, $4, $5, $6, TRUE, 'unknown')
			ON CONFLICT DO NOTHING`,
			d.name, d.ip, d.protocol, d.interval, tags, nullStr(d.httpPath))
		if err != nil {
			return fmt.Errorf("insert device %s: %w", d.name, err)
		}
	}
	slog.Info("Seed: default devices created", "count", len(defaults))
	return nil
}

// seedDefaultSensors creates a default sensor for each device that doesn't have one.
func seedDefaultSensors(ctx context.Context, pg *Postgres) error {
	rows, err := pg.pool.Query(ctx, `
		SELECT d.id, d.name, d.protocol, d.interval_sec
		FROM devices d
		WHERE NOT EXISTS (SELECT 1 FROM sensors s WHERE s.device_id = d.id)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id int64
		var name, protocol string
		var interval int
		if err := rows.Scan(&id, &name, &protocol, &interval); err != nil {
			return err
		}

		sensorName := fmt.Sprintf("%s %s Sensor", name, strings.ToUpper(protocol))
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO sensors (device_id, name, type, interval, config, enabled)
			VALUES ($1, $2, $3, $4, '{}', TRUE)
			ON CONFLICT DO NOTHING`,
			id, sensorName, protocol, interval)
		if err != nil {
			return fmt.Errorf("insert sensor for device %s: %w", name, err)
		}
		count++
	}
	if count > 0 {
		slog.Info("Seed: default sensors created", "count", count)
	}
	return rows.Err()
}

// seedDefaultAlertRules creates built-in alert rules with conditions.
func seedDefaultAlertRules(ctx context.Context, pg *Postgres) error {
	var count int64
	if err := pg.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alert_rules`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		slog.Debug("Seed: alert rules already exist, skipping", "count", count)
		return nil
	}

	type ruleWithConditions struct {
		name        string
		description string
		severity    string
		conditions  []models.AlertRuleCondition
	}

	rules := []ruleWithConditions{
		{
			name:        "Device Down",
			description: "Fires when a device goes down for more than 60 seconds",
			severity:    "critical",
			conditions: []models.AlertRuleCondition{{
				Type:            "state_change",
				MetricField:     "status",
				Operator:        "eq",
				Value:           "down",
				DurationSeconds: 60,
			}},
		},
		{
			name:        "High Latency",
			description: "Fires when response time exceeds 500ms for 5 minutes",
			severity:    "warning",
			conditions: []models.AlertRuleCondition{{
				Type:            "threshold",
				MetricField:     "response_time",
				Operator:        "gt",
				Value:           "500",
				DurationSeconds: 300,
			}},
		},
		{
			name:        "Critical Latency",
			description: "Fires when response time exceeds 2000ms for 2 minutes",
			severity:    "critical",
			conditions: []models.AlertRuleCondition{{
				Type:            "threshold",
				MetricField:     "response_time",
				Operator:        "gt",
				Value:           "2000",
				DurationSeconds: 120,
			}},
		},
		{
			name:        "Device Degraded",
			description: "Fires when a device enters degraded or warning state",
			severity:    "warning",
			conditions: []models.AlertRuleCondition{
				{
					Type:        "state_change",
					MetricField: "status",
					Operator:    "eq",
					Value:       "degraded",
				},
				{
					Type:        "state_change",
					MetricField: "status",
					Operator:    "eq",
					Value:       "warning",
				},
			},
		},
		{
			name:        "No Data Received",
			description: "Fires when no metrics are received for 3x the polling interval",
			severity:    "warning",
			conditions: []models.AlertRuleCondition{{
				Type:            "absence",
				MetricField:     "status",
				DurationSeconds: 180,
			}},
		},
		{
			name:        "Latency Anomaly",
			description: "Fires when response time deviates more than 2.5σ from 24h baseline",
			severity:    "warning",
			conditions: []models.AlertRuleCondition{{
				Type:        "anomaly",
				MetricField: "response_time",
				Value:       "2.5",
			}},
		},
		{
			name:        "Port State Change",
			description: "Fires when any port open/close state change is detected",
			severity:    "info",
			conditions: []models.AlertRuleCondition{{
				Type:        "state_change",
				MetricField: "port_state",
				Operator:    "neq",
				Value:       "",
			}},
		},
	}

	tx, err := pg.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, r := range rules {
		var ruleID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO alert_rules (name, description, severity, scope_type, condition_logic, cooldown_seconds, auto_resolve)
			VALUES ($1, $2, $3, 'global', 'any', 300, TRUE)
			RETURNING id`,
			r.name, r.description, r.severity).Scan(&ruleID)
		if err != nil {
			return fmt.Errorf("insert rule %s: %w", r.name, err)
		}

		for _, c := range r.conditions {
			configJSON, _ := json.Marshal(c.Config)
			if c.Config == nil {
				configJSON = []byte("{}")
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO alert_rule_conditions (rule_id, type, metric_field, operator, value, duration_seconds, config)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				ruleID, c.Type, c.MetricField, c.Operator, c.Value, c.DurationSeconds, configJSON)
			if err != nil {
				return fmt.Errorf("insert condition for rule %s: %w", r.name, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("Seed: default alert rules created", "count", len(rules))
	return nil
}

// SeedAPIKey creates a default API key if the raw key is provided and no keys exist.
func SeedAPIKey(ctx context.Context, db Database, keyHash, description string) error {
	pg, ok := db.(*Postgres)
	if !ok {
		return fmt.Errorf("seed only supports Postgres")
	}

	// Get admin user ID for FK
	var userID int64
	err := pg.pool.QueryRow(ctx, `SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&userID)
	if err != nil {
		return fmt.Errorf("find admin user: %w", err)
	}

	_, err = pg.pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, key_hash, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (key_hash) DO NOTHING`,
		userID, keyHash, description)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}

	slog.Info("Seed: API key seeded", "description", description)
	return nil
}
