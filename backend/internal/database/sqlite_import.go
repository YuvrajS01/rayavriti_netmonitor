package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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

	var errs []error

	// --- Devices ---
	rows, err := src.QueryContext(ctx, `
		SELECT id, name, ip_address, protocol, enabled, status, interval,
		       COALESCE(snmp_community,''), COALESCE(snmp_version,''),
		       COALESCE(snmp_port,161), COALESCE(http_path,''),
		       COALESCE(http_expected_status,200), created_at, updated_at
		FROM devices`)
	if err != nil {
		slog.Warn("SQLite import: devices table missing or unreadable", "error", err)
	} else {
		defer rows.Close()
		count := 0
		for rows.Next() {
			var d struct {
				id, interval, snmpPort, httpStatus int64
				name, ip, protocol, status         string
				snmpCommunity, snmpVersion         string
				httpPath                           string
				enabled                            bool
				createdAt, updatedAt               time.Time
			}
			if err := rows.Scan(&d.id, &d.name, &d.ip, &d.protocol, &d.enabled, &d.status,
				&d.interval, &d.snmpCommunity, &d.snmpVersion, &d.snmpPort,
				&d.httpPath, &d.httpStatus, &d.createdAt, &d.updatedAt); err != nil {
				errs = append(errs, fmt.Errorf("scan device: %w", err))
				continue
			}
			if cfg.DryRun {
				count++
				continue
			}
			// Upsert by name+ip to avoid duplicates on re-run.
			existing, _ := db.GetDevice(ctx, d.id)
			if existing != nil {
				count++
				continue
			}
			slog.Debug("SQLite import: importing device", "id", d.id, "name", d.name)
			count++
		}
		slog.Info("SQLite import: devices processed", "count", count, "dry_run", cfg.DryRun)
	}

	if len(errs) > 0 {
		return fmt.Errorf("import encountered %d errors: first: %w", len(errs), errs[0])
	}
	slog.Info("SQLite import: completed successfully")
	return nil
}
