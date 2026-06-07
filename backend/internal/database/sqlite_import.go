package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; optional — only needed for import
)

// SQLiteImportConfig holds paths and options for the one-time migration.
type SQLiteImportConfig struct {
	SQLitePath string // path to the existing .db file
	DryRun     bool   // if true, parse but do not write to Postgres
}

// ImportFromSQLite reads the existing SQLite database and writes all records
// to the PostgreSQL backend via the Database interface.
// This is a one-time migration helper — it is safe to call repeatedly; already-
// present rows (by ID or unique key) are skipped.
func ImportFromSQLite(ctx context.Context, db Database, cfg SQLiteImportConfig) error {
	slog.Info("SQLite import: opening source", "path", cfg.SQLitePath)
	src, err := sql.Open("sqlite", cfg.SQLitePath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer src.Close()

	if err := src.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}

	pg, ok := db.(*Postgres)
	if !ok {
		return fmt.Errorf("import target must be Postgres")
	}

	var totalImported, totalSkipped, totalErrors int

	// ── Users ──
	userCount, userSkipped, userErrs := importUsers(ctx, src, pg, cfg.DryRun)
	totalImported += userCount
	totalSkipped += userSkipped
	totalErrors += userErrs
	slog.Info("SQLite import: users", "imported", userCount, "skipped", userSkipped, "errors", userErrs)

	// ── Devices ──
	devCount, devSkipped, devErrs := importDevices(ctx, src, pg, cfg.DryRun)
	totalImported += devCount
	totalSkipped += devSkipped
	totalErrors += devErrs
	slog.Info("SQLite import: devices", "imported", devCount, "skipped", devSkipped, "errors", devErrs)

	// ── Metrics ──
	metCount, metSkipped, metErrs := importMetrics(ctx, src, pg, cfg.DryRun)
	totalImported += metCount
	totalSkipped += metSkipped
	totalErrors += metErrs
	slog.Info("SQLite import: metrics", "imported", metCount, "skipped", metSkipped, "errors", metErrs)

	// ── Alerts ──
	alertCount, alertSkipped, alertErrs := importAlerts(ctx, src, pg, cfg.DryRun)
	totalImported += alertCount
	totalSkipped += alertSkipped
	totalErrors += alertErrs
	slog.Info("SQLite import: alerts", "imported", alertCount, "skipped", alertSkipped, "errors", alertErrs)

	// ── Dashboards ──
	dashCount, dashSkipped, dashErrs := importDashboards(ctx, src, pg, cfg.DryRun)
	totalImported += dashCount
	totalSkipped += dashSkipped
	totalErrors += dashErrs
	slog.Info("SQLite import: dashboards", "imported", dashCount, "skipped", dashSkipped, "errors", dashErrs)

	// ── Reset sequences ──
	if !cfg.DryRun {
		resetSequences(ctx, pg)
	}

	slog.Info("SQLite import: completed",
		"total_imported", totalImported,
		"total_skipped", totalSkipped,
		"total_errors", totalErrors,
		"dry_run", cfg.DryRun)

	if totalErrors > 0 {
		return fmt.Errorf("import completed with %d errors", totalErrors)
	}
	return nil
}

func importUsers(ctx context.Context, src *sql.DB, pg *Postgres, dryRun bool) (imported, skipped, errors int) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, username, password_hash, COALESCE(role,'viewer'),
		       COALESCE(display_name,''), COALESCE(email,''), COALESCE(phone,''),
		       COALESCE(enabled,1), created_at
		FROM users`)
	if err != nil {
		slog.Warn("SQLite import: users table missing", "error", err)
		return 0, 0, 0
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var username, hash, role, display, email, phone string
		var enabled bool
		var createdAt string
		if err := rows.Scan(&id, &username, &hash, &role, &display, &email, &phone, &enabled, &createdAt); err != nil {
			errors++
			continue
		}
		if dryRun {
			imported++
			continue
		}
		ts := parseTimestamp(createdAt)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO users (id, username, password_hash, role, display_name, email, phone, enabled, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (username) DO NOTHING`,
			id, username, hash, role, nullStr(display), nullStr(email), nullStr(phone), enabled, ts)
		if err != nil {
			slog.Debug("SQLite import: skip user", "username", username, "error", err)
			skipped++
		} else {
			imported++
		}
	}
	return
}

func importDevices(ctx context.Context, src *sql.DB, pg *Postgres, dryRun bool) (imported, skipped, errors int) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, name, COALESCE(ip_address, COALESCE(host,'')), COALESCE(protocol,'ping'),
		       COALESCE(enabled,1), COALESCE(status,'unknown'),
		       COALESCE(interval, interval_seconds, 60),
		       COALESCE(snmp_community,''), COALESCE(snmp_version,''),
		       COALESCE(snmp_port,161), COALESCE(http_path,''),
		       COALESCE(http_expected_status,200),
		       created_at, COALESCE(updated_at, created_at)
		FROM devices`)
	if err != nil {
		slog.Warn("SQLite import: devices table missing", "error", err)
		return 0, 0, 0
	}
	defer rows.Close()

	for rows.Next() {
		var id, interval int64
		var snmpPort, httpStatus int
		var name, ip, protocol, status string
		var snmpCommunity, snmpVersion, httpPath string
		var enabled bool
		var createdAt, updatedAt string
		if err := rows.Scan(&id, &name, &ip, &protocol, &enabled, &status,
			&interval, &snmpCommunity, &snmpVersion, &snmpPort,
			&httpPath, &httpStatus, &createdAt, &updatedAt); err != nil {
			errors++
			continue
		}
		if dryRun {
			imported++
			continue
		}
		ts := parseTimestamp(createdAt)
		us := parseTimestamp(updatedAt)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO devices (id, name, ip_address, protocol, enabled, status, interval_sec,
			                     snmp_community, snmp_version, snmp_port, http_path, http_expected_status,
			                     tags, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'[]',$13,$14)
			ON CONFLICT (id) DO NOTHING`,
			id, name, ip, protocol, enabled, status, interval,
			nullStr(snmpCommunity), nullStr(snmpVersion), snmpPort,
			nullStr(httpPath), httpStatus, ts, us)
		if err != nil {
			slog.Debug("SQLite import: skip device", "name", name, "error", err)
			skipped++
		} else {
			imported++
		}
	}
	return
}

func importMetrics(ctx context.Context, src *sql.DB, pg *Postgres, dryRun bool) (imported, skipped, errors int) {
	rows, err := src.QueryContext(ctx, `
		SELECT device_id, timestamp, status,
		       response_time, packet_loss, cpu_usage, memory_usage,
		       bandwidth, custom_value, COALESCE(details,'{}'), COALESCE(message,'')
		FROM metrics ORDER BY timestamp ASC`)
	if err != nil {
		slog.Warn("SQLite import: metrics table missing", "error", err)
		return 0, 0, 0
	}
	defer rows.Close()

	batch := 0
	for rows.Next() {
		var deviceID int64
		var timestamp, status, message string
		var responseTime, packetLoss, cpuUsage, memoryUsage, bandwidth, customValue sql.NullFloat64
		var details string
		if err := rows.Scan(&deviceID, &timestamp, &status,
			&responseTime, &packetLoss, &cpuUsage, &memoryUsage,
			&bandwidth, &customValue, &details, &message); err != nil {
			errors++
			continue
		}
		if dryRun {
			imported++
			continue
		}

		// Parse details JSON - merge message into it if present
		var detailsMap map[string]any
		_ = json.Unmarshal([]byte(details), &detailsMap)
		if detailsMap == nil {
			detailsMap = map[string]any{}
		}
		if message != "" {
			detailsMap["message"] = message
		}
		detailsJSON, _ := json.Marshal(detailsMap)

		ts := parseTimestamp(timestamp)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO metrics (device_id, timestamp, status, response_time, packet_loss,
			                     cpu_usage, memory_usage, bandwidth, custom_value, details)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			deviceID, ts, status,
			nullFloat(responseTime), nullFloat(packetLoss),
			nullFloat(cpuUsage), nullFloat(memoryUsage),
			nullFloat(bandwidth), nullFloat(customValue), detailsJSON)
		if err != nil {
			errors++
			if batch < 5 { // only log first few
				slog.Debug("SQLite import: metric error", "device_id", deviceID, "error", err)
			}
		} else {
			imported++
		}
		batch++
	}
	return
}

func importAlerts(ctx context.Context, src *sql.DB, pg *Postgres, dryRun bool) (imported, skipped, errors int) {
	rows, err := src.QueryContext(ctx, `
		SELECT a.id, a.device_id, COALESCE(d.name,'Unknown'), a.severity, a.message,
		       a.status, a.created_at,
		       a.acknowledged_at, a.resolved_at, a.acknowledged_by, a.resolved_by
		FROM alerts a
		LEFT JOIN devices d ON d.id = a.device_id`)
	if err != nil {
		slog.Warn("SQLite import: alerts table missing", "error", err)
		return 0, 0, 0
	}
	defer rows.Close()

	for rows.Next() {
		var id, deviceID int64
		var deviceName, severity, message, status, createdAt string
		var ackedAt, resolvedAt, ackedBy, resolvedBy sql.NullString
		if err := rows.Scan(&id, &deviceID, &deviceName, &severity, &message,
			&status, &createdAt, &ackedAt, &resolvedAt, &ackedBy, &resolvedBy); err != nil {
			errors++
			continue
		}
		if dryRun {
			imported++
			continue
		}
		ts := parseTimestamp(createdAt)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO alerts (id, device_id, device_name, severity, message, status, created_at,
			                    acknowledged_at, resolved_at, acknowledged_by, resolved_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (id) DO NOTHING`,
			id, deviceID, deviceName, severity, message, status, ts,
			nullTimestamp(ackedAt), nullTimestamp(resolvedAt),
			nullNullStr(ackedBy), nullNullStr(resolvedBy))
		if err != nil {
			slog.Debug("SQLite import: skip alert", "id", id, "error", err)
			skipped++
		} else {
			imported++
		}
	}
	return
}

func importDashboards(ctx context.Context, src *sql.DB, pg *Postgres, dryRun bool) (imported, skipped, errors int) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, COALESCE(user_id,1), name, COALESCE(layout,'{}'),
		       created_at, COALESCE(updated_at, created_at)
		FROM dashboards`)
	if err != nil {
		slog.Warn("SQLite import: dashboards table missing", "error", err)
		return 0, 0, 0
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID int64
		var name, layout, createdAt, updatedAt string
		if err := rows.Scan(&id, &userID, &name, &layout, &createdAt, &updatedAt); err != nil {
			errors++
			continue
		}
		if dryRun {
			imported++
			continue
		}

		// Ensure layout is valid JSON
		var layoutJSON json.RawMessage
		if err := json.Unmarshal([]byte(layout), &layoutJSON); err != nil {
			layoutJSON = []byte("{}")
		}

		ts := parseTimestamp(createdAt)
		us := parseTimestamp(updatedAt)
		_, err := pg.pool.Exec(ctx, `
			INSERT INTO dashboards (id, user_id, name, layout, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (id) DO NOTHING`,
			id, userID, name, layoutJSON, ts, us)
		if err != nil {
			slog.Debug("SQLite import: skip dashboard", "id", id, "error", err)
			skipped++
		} else {
			imported++
		}
	}
	return
}

// resetSequences resets PostgreSQL sequences to max(id)+1 for each table
// to avoid primary key conflicts after importing with explicit IDs.
func resetSequences(ctx context.Context, pg *Postgres) {
	tables := []struct {
		table, seq string
	}{
		{"users", "users_id_seq"},
		{"devices", "devices_id_seq"},
		{"alerts", "alerts_id_seq"},
		{"dashboards", "dashboards_id_seq"},
		{"api_keys", "api_keys_id_seq"},
	}
	for _, t := range tables {
		_, err := pg.pool.Exec(ctx, fmt.Sprintf(
			`SELECT setval('%s', COALESCE((SELECT MAX(id) FROM %s), 0) + 1, false)`,
			t.seq, t.table))
		if err != nil {
			slog.Warn("SQLite import: reset sequence", "table", t.table, "error", err)
		}
	}
	slog.Info("SQLite import: sequences reset")
}

// ── timestamp helpers ──

func parseTimestamp(s string) time.Time {
	// Try common SQLite timestamp formats
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	slog.Debug("SQLite import: unparseable timestamp, using now", "value", s)
	return time.Now()
}

func nullFloat(nf sql.NullFloat64) *float64 {
	if nf.Valid {
		return &nf.Float64
	}
	return nil
}

func nullTimestamp(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTimestamp(ns.String)
	return &t
}

func nullNullStr(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	return &ns.String
}
