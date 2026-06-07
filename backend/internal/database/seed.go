package database

import (
	"context"
	"fmt"
)

// SeedDefaults creates the admin user and default alert rules if they don't exist.
func SeedDefaults(ctx context.Context, db Database, adminUsername, adminPasswordHash string) error {
	// Update admin password hash (V11 inserted a placeholder)
	pg, ok := db.(*Postgres)
	if !ok {
		return fmt.Errorf("seed only supports Postgres")
	}
	_, err := pg.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1 WHERE username=$2 AND password_hash='PLACEHOLDER'`,
		adminPasswordHash, adminUsername)
	if err != nil {
		return fmt.Errorf("seed admin password: %w", err)
	}

	rules := []struct {
		name, condition, severity, template string
		threshold                           float64
	}{
		{"Device Down", "device_down", "critical", "Device {{.DeviceName}} is DOWN", 0},
		{"High CPU", "cpu_usage_gt", "warning", "Device {{.DeviceName}} CPU at {{.Value}}%", 90},
		{"High Memory", "memory_usage_gt", "warning", "Device {{.DeviceName}} memory at {{.Value}}%", 90},
		{"High Packet Loss", "packet_loss_gt", "warning", "Device {{.DeviceName}} packet loss {{.Value}}%", 10},
		{"Slow Response", "response_time_gt", "warning", "Device {{.DeviceName}} response {{.Value}}ms", 5000},
	}

	for _, r := range rules {
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO alert_rules(name,condition,threshold,severity,message_template)
			VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
			r.name, r.condition, r.threshold, r.severity, r.template)
		if err != nil {
			return fmt.Errorf("seed rule %s: %w", r.name, err)
		}
	}
	return nil
}
