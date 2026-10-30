package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type Postgres struct {
	pool           *pgxpool.Pool
	dsn            string
	cfg            DatabaseConfig
	hasTimescaleDB bool
}

type DatabaseConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func NewPostgres(dsn string, cfg *DatabaseConfig) *Postgres {
	if cfg == nil {
		cfg = &DatabaseConfig{
			MaxConns:          20,
			MinConns:          2,
			MaxConnLifetime:   time.Hour,
			MaxConnIdleTime:   5 * time.Minute,
			HealthCheckPeriod: 30 * time.Second,
		}
	}
	return &Postgres{dsn: dsn, cfg: *cfg}
}

func (p *Postgres) Connect(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(p.dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = p.cfg.MaxConns
	cfg.MinConns = p.cfg.MinConns
	cfg.MaxConnLifetime = p.cfg.MaxConnLifetime
	cfg.MaxConnIdleTime = p.cfg.MaxConnIdleTime
	cfg.HealthCheckPeriod = p.cfg.HealthCheckPeriod
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	p.pool = pool
	if err := p.Ping(ctx); err != nil {
		return err
	}
	var hasTS bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='timescaledb')`).Scan(&hasTS); err != nil {
		slog.Warn("failed to check TimescaleDB extension; falling back to unbounded DELETE pruning", "error", err)
	}
	p.hasTimescaleDB = hasTS
	return nil
}

func (p *Postgres) Close() error {
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

// Pool returns the underlying pgx connection pool for use by domain-specific
// service packages (campus, importer, etc.) that need direct SQL access.
func (p *Postgres) Pool() *pgxpool.Pool {
	return p.pool
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Postgres) RunMigrations(ctx context.Context) error {
	// Ensure the tracking table exists first (migration V1). Use a raw
	// exec since the table may not exist yet and we can't query it.
	if _, err := p.pool.Exec(ctx, migrations[0].SQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Add the checksum column if it doesn't exist (for deployments that
	// already have schema_migrations from before C7). This is idempotent.
	_, _ = p.pool.Exec(ctx, `ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT`)

	// TimescaleDB is required: the schema uses hypertables from V5 onward,
	// and continuous aggregates/compression/retention depend on it.
	if _, err := p.pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`); err != nil {
		return fmt.Errorf("TimescaleDB extension is required (use the timescale/timescaledb image or install the extension): %w", err)
	}

	// Session-level advisory lock so only one instance runs migrations at a
	// time (C8). Lock and unlock must run on the same connection (B20): via
	// the pool they could land on different connections, leaving the lock
	// held by an idle pooled connection and blocking other instances.
	lockConn, err := p.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration lock: %w", err)
	}
	defer lockConn.Release()
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock(727280)`); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = lockConn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(727280)`)
	}()

	// Validate strict version ordering (C7 — positional indexing is gone;
	// versions are explicit and must be strictly increasing).
	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			return fmt.Errorf("migration version ordering violation: V%d after V%d",
				migrations[i].Version, migrations[i-1].Version)
		}
	}

	for _, m := range migrations {
		version := int64(m.Version)

		// Check if this migration was already applied.
		var exists bool
		var recordedChecksum *string
		err := p.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1),
			        (SELECT checksum FROM schema_migrations WHERE version=$1)`,
			version).Scan(&exists, &recordedChecksum)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists {
			// C7: Verify the checksum of an already-applied migration
			// matches what we have now. A mismatch means someone edited
			// an already-applied migration, which is silent schema drift.
			if recordedChecksum != nil && *recordedChecksum != "" {
				if *recordedChecksum != m.Checksum() {
					return fmt.Errorf("migration %d checksum mismatch: the SQL was modified after it was applied (recorded=%s, current=%s) — this indicates schema drift",
						version, *recordedChecksum, m.Checksum())
				}
			}
			continue
		}

		// Apply the entire migration in a single transaction (C8) and let
		// Postgres parse multi-statement SQL natively (N6).
		tx, err := p.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, m.SQL); err != nil {
			// Never record a failed migration as applied: the transaction
			// rolled back every statement in it.
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %d: %w\nSQL: %s", version, err, m.SQL)
		}
		// Record the version with its checksum (C7).
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations(version, checksum) VALUES($1, $2) ON CONFLICT DO NOTHING`,
			version, m.Checksum()); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %d: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}
	return nil
}

// ── Devices ──────────────────────────────────────────────────────────────────

const deviceSelectCols = `
	SELECT id,name,ip_address,protocol,port,enabled,status,priority,tags,
	       COALESCE(snmp_community,''),COALESCE(snmp_version,''),COALESCE(snmp_port,0),COALESCE(http_path,''),COALESCE(http_expected_status,0),
	       interval_sec,location_id,parent_device_id,COALESCE(dependency_port,''),COALESCE(rack_position,''),COALESCE(asset_tag,''),
	       COALESCE(mac_address,''),COALESCE(serial_number,''),COALESCE(manufacturer,''),COALESCE(model,''),COALESCE(device_category,''),COALESCE(monitor_config,'{}'::jsonb),COALESCE(notes,''),snmp_v3,created_at,updated_at
		FROM devices`

func (p *Postgres) GetDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelectCols+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

func (p *Postgres) GetDevice(ctx context.Context, id int64) (*models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelectCols+` WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices, err := scanDevices(rows)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &devices[0], nil
}

func (p *Postgres) CreateDevice(ctx context.Context, d *models.Device) (*models.Device, error) {
	tags, _ := json.Marshal(d.Tags)
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO devices(name,ip_address,protocol,port,enabled,priority,tags,snmp_community,snmp_version,
		                    snmp_port,http_path,http_expected_status,interval_sec,
		                    location_id,parent_device_id,dependency_port,rack_position,asset_tag,mac_address,serial_number,manufacturer,
		                    model,device_category,monitor_config,notes,snmp_v3)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
		RETURNING id`,
		d.Name, d.IPAddress, d.Protocol, d.Port, d.Enabled, d.Priority, tags,
		nullStr(d.SNMPCommunity), nullStr(d.SNMPVersion), nullInt(d.SNMPPort),
		nullStr(d.HTTPPath), nullInt(d.HTTPExpectedStatus), d.Interval,
		d.LocationID, d.ParentDeviceID, nullStr(d.DependencyPort), nullStr(d.RackPosition), nullStr(d.AssetTag),
		nullStr(d.MACAddress), nullStr(d.SerialNumber), nullStr(d.Manufacturer), nullStr(d.Model),
		nullStr(d.DeviceCategory), jsonConfig(d.MonitorConfig), nullStr(d.Notes), snmpV3JSON(d.SNMPv3),
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return p.GetDevice(ctx, id)
}

func (p *Postgres) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	tags, _ := json.Marshal(d.Tags)
	_, err := p.pool.Exec(ctx, `
		UPDATE devices SET name=$1,ip_address=$2,protocol=$3,port=$4,enabled=$5,priority=$6,tags=$7,
		    snmp_community=$8,snmp_version=$9,snmp_port=$10,http_path=$11,
		    http_expected_status=$12,interval_sec=$13,location_id=$14,
		    parent_device_id=$15,dependency_port=$16,rack_position=$17,asset_tag=$18,mac_address=$19,serial_number=$20,manufacturer=$21,
		    model=$22,device_category=$23,monitor_config=$24,notes=$25,snmp_v3=$26,updated_at=NOW()
		WHERE id=$27`,
		d.Name, d.IPAddress, d.Protocol, d.Port, d.Enabled, d.Priority, tags,
		nullStr(d.SNMPCommunity), nullStr(d.SNMPVersion), nullInt(d.SNMPPort),
		nullStr(d.HTTPPath), nullInt(d.HTTPExpectedStatus), d.Interval,
		d.LocationID, d.ParentDeviceID, nullStr(d.DependencyPort), nullStr(d.RackPosition), nullStr(d.AssetTag),
		nullStr(d.MACAddress), nullStr(d.SerialNumber), nullStr(d.Manufacturer), nullStr(d.Model),
		nullStr(d.DeviceCategory), jsonConfig(d.MonitorConfig), nullStr(d.Notes), snmpV3JSON(d.SNMPv3), id)
	if err != nil {
		return nil, err
	}
	return p.GetDevice(ctx, id)
}

func (p *Postgres) DeleteDevice(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM devices WHERE id=$1`, id)
	return err
}

func scanDevices(rows pgx.Rows) ([]models.Device, error) {
	var out []models.Device
	for rows.Next() {
		var d models.Device
		var tagsRaw, monitorConfigRaw, snmpV3Raw []byte
		err := rows.Scan(
			&d.ID, &d.Name, &d.IPAddress, &d.Protocol, &d.Port, &d.Enabled, &d.Status, &d.Priority, &tagsRaw,
			&d.SNMPCommunity, &d.SNMPVersion, &d.SNMPPort, &d.HTTPPath, &d.HTTPExpectedStatus,
			&d.Interval, &d.LocationID, &d.ParentDeviceID, &d.DependencyPort, &d.RackPosition, &d.AssetTag,
			&d.MACAddress, &d.SerialNumber, &d.Manufacturer, &d.Model, &d.DeviceCategory, &monitorConfigRaw, &d.Notes,
			&snmpV3Raw, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if tagsRaw != nil {
			_ = json.Unmarshal(tagsRaw, &d.Tags)
		}
		if d.Tags == nil {
			d.Tags = []string{}
		}
		if len(monitorConfigRaw) > 0 {
			_ = json.Unmarshal(monitorConfigRaw, &d.MonitorConfig)
		}
		if d.MonitorConfig == nil {
			d.MonitorConfig = map[string]any{}
		}
		if len(snmpV3Raw) > 0 {
			var v3 models.SNMPv3Config
			if json.Unmarshal(snmpV3Raw, &v3) == nil {
				d.SNMPv3 = &v3
			}
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// snmpV3JSON encodes USM credentials for the snmp_v3 column; nil stores NULL.
func snmpV3JSON(c *models.SNMPv3Config) []byte {
	if c == nil {
		return nil
	}
	encoded, _ := json.Marshal(c)
	return encoded
}

func jsonConfig(value map[string]any) []byte {
	if value == nil {
		return []byte("{}")
	}
	encoded, _ := json.Marshal(value)
	return encoded
}

// UpdateDeviceStatus updates only the status field
func (p *Postgres) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	_, err := p.pool.Exec(ctx, `UPDATE devices SET status=$1,updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (p *Postgres) UpdateDeviceStatuses(ctx context.Context, ids []int64, statuses []string) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// Unchanged rows are skipped: rewriting devices on every poll was a
	// non-HOT update (status is indexed) that bloated every devices index.
	rows, err := p.pool.Query(ctx, `
		UPDATE devices d SET status = v.status, updated_at = NOW()
		FROM unnest($1::bigint[], $2::text[]) AS v(id, status)
		WHERE d.id = v.id AND d.status IS DISTINCT FROM v.status
		RETURNING d.id`, ids, statuses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var changed []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		changed = append(changed, id)
	}
	return changed, rows.Err()
}

func (p *Postgres) RecordInterfaceSamples(ctx context.Context, samples []models.InterfaceSample) error {
	if len(samples) == 0 {
		return nil
	}
	_, err := p.pool.CopyFrom(ctx, pgx.Identifier{"interface_metrics"},
		[]string{"time", "device_id", "if_index", "if_name", "in_bps", "out_bps", "utilization", "oper_status", "speed_bps"},
		pgx.CopyFromSlice(len(samples), func(i int) ([]any, error) {
			s := samples[i]
			var util *float32
			if s.Utilization != nil {
				u := float32(*s.Utilization)
				util = &u
			}
			return []any{s.Time, s.DeviceID, s.IfIndex, s.Name, s.InBps, s.OutBps, util, int16(s.OperStatus), s.SpeedBps}, nil //nolint:gosec // ifOperStatus is 1-7
		}))
	return err
}

// ── Metrics ───────────────────────────────────────────────────────────────────

func (p *Postgres) RecordMetric(ctx context.Context, m *models.Metric) error {
	details, _ := json.Marshal(m.Details)
	_, err := p.pool.Exec(ctx, `
		INSERT INTO metrics(device_id,timestamp,status,response_time,packet_loss,
		                    cpu_usage,memory_usage,bandwidth,custom_value,details)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		m.DeviceID, m.Timestamp, m.Status,
		m.ResponseTime, m.PacketLoss, m.CPUUsage, m.MemoryUsage, m.Bandwidth, m.CustomValue,
		details)
	return err
}

func (p *Postgres) RecordMetricsBatch(ctx context.Context, metrics []*models.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	rows := make([][]any, len(metrics))
	for i, m := range metrics {
		details, _ := json.Marshal(m.Details)
		rows[i] = []any{
			m.DeviceID, m.Timestamp, m.Status,
			m.ResponseTime, m.PacketLoss, m.CPUUsage, m.MemoryUsage,
			m.Bandwidth, m.CustomValue, details,
		}
	}

	_, err := p.pool.CopyFrom(ctx,
		pgx.Identifier{"metrics"},
		[]string{"device_id", "timestamp", "status", "response_time", "packet_loss",
			"cpu_usage", "memory_usage", "bandwidth", "custom_value", "details"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func (p *Postgres) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	// One index probe per device on (device_id, timestamp DESC), limited to
	// recent chunks. DISTINCT ON over all of metrics scanned every chunk, so
	// its cost grew with total history (B17). Devices with no poll in the
	// last 7 days are stale and omitted.
	rows, err := p.pool.Query(ctx, `
		SELECT m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		       m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		       d.protocol, d.name
		FROM devices d
		CROSS JOIN LATERAL (
		    SELECT * FROM metrics m
		    WHERE m.device_id = d.id AND m.timestamp > NOW() - INTERVAL '7 days'
		    ORDER BY m.timestamp DESC
		    LIMIT 1
		) m
		ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
}

func (p *Postgres) GetLatestMetricForDevice(ctx context.Context, deviceID int64) (*models.Metric, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		       m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		       d.protocol, d.name
		FROM metrics m
		INNER JOIN devices d ON d.id = m.device_id
		WHERE m.device_id=$1
		ORDER BY m.timestamp DESC LIMIT 1`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	metrics, err := scanMetricsWithDevice(rows)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &metrics[0], nil
}

func (p *Postgres) GetDeviceMetrics(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := p.pool.Query(ctx, `
		SELECT m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		       m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		       d.protocol, d.name
		FROM metrics m
		INNER JOIN devices d ON d.id = m.device_id
		WHERE m.device_id=$1 AND m.timestamp BETWEEN $2 AND $3
		ORDER BY m.timestamp DESC LIMIT $4`,
		deviceID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
}

func (p *Postgres) GetMetricsSummary(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
	var totalSamples, downSamples, warningSamples int64
	var avgRT *float64
	query := `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'down'),
			COUNT(*) FILTER (WHERE status IN ('warning','degraded')),
			AVG(response_time)
		FROM metrics WHERE timestamp BETWEEN $1 AND $2`
	args := []any{from, to}
	if deviceID != nil {
		query += ` AND device_id=$3`
		args = append(args, *deviceID)
	}
	err := p.pool.QueryRow(ctx, query, args...).Scan(&totalSamples, &downSamples, &warningSamples, &avgRT)
	if err != nil {
		return nil, err
	}
	availabilityPercent := float64(0)
	if totalSamples > 0 {
		availabilityPercent = float64(totalSamples-downSamples) / float64(totalSamples) * 100
	}
	averageResponseMs := float64(0)
	if avgRT != nil {
		averageResponseMs = *avgRT
	}
	return map[string]any{
		"totalSamples":        totalSamples,
		"downSamples":         downSamples,
		"warningSamples":      warningSamples,
		"availabilityPercent": math.Round(availabilityPercent*100) / 100,
		"averageResponseMs":   math.Round(averageResponseMs*100) / 100,
	}, nil
}

func scanMetrics(rows pgx.Rows) ([]models.Metric, error) {
	var out []models.Metric
	for rows.Next() {
		var m models.Metric
		var detailsRaw []byte
		err := rows.Scan(
			&m.ID, &m.DeviceID, &m.Timestamp, &m.Status,
			&m.ResponseTime, &m.PacketLoss, &m.CPUUsage, &m.MemoryUsage,
			&m.Bandwidth, &m.CustomValue, &detailsRaw,
		)
		if err != nil {
			return nil, err
		}
		if detailsRaw != nil {
			_ = json.Unmarshal(detailsRaw, &m.Details)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMetricsWithDevice(rows pgx.Rows) ([]models.Metric, error) {
	var out []models.Metric
	for rows.Next() {
		var m models.Metric
		var detailsRaw []byte
		var protocol, deviceName string
		err := rows.Scan(
			&m.ID, &m.DeviceID, &m.Timestamp, &m.Status,
			&m.ResponseTime, &m.PacketLoss, &m.CPUUsage, &m.MemoryUsage,
			&m.Bandwidth, &m.CustomValue, &detailsRaw,
			&protocol, &deviceName,
		)
		if err != nil {
			return nil, err
		}
		if detailsRaw != nil {
			_ = json.Unmarshal(detailsRaw, &m.Details)
		}
		m.Protocol = protocol
		m.DeviceName = deviceName
		m.CreatedAt = m.Timestamp
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── Alerts ────────────────────────────────────────────────────────────────────

func (p *Postgres) GetAlerts(ctx context.Context, status string, limit, offset int, scope *ScopeFilter) ([]models.Alert, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	conditions := []string{}
	argN := 1
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status=$%d", argN))
		args = append(args, status)
		argN++
	}
	if scope != nil {
		if cond, ok := buildAlertScopeCondition(scope, &args, &argN); ok {
			conditions = append(conditions, cond)
		}
	}
	base := "FROM alerts"
	if len(conditions) > 0 {
		base += " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	countSQL := `SELECT COUNT(*) ` + base
	if err := p.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listSQL := `SELECT ` + alertCols + ` ` +
		base + ` ORDER BY created_at DESC, id DESC`
	listSQL += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argN, argN+1)
	args = append(args, limit, offset)
	rows, err := p.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	alerts, err := scanAlerts(rows)
	return alerts, total, err
}

// buildAlertScopeCondition renders a scope filter as a WHERE condition against
// the alerts table. Alerts only carry device_id, so the device scope
// condition (shared with device listing) is applied through devices.
func buildAlertScopeCondition(scope *ScopeFilter, args *[]any, argN *int) (string, bool) {
	cond, scopeArgs := scopeCondition(scope, *argN)
	*args = append(*args, scopeArgs...)
	*argN += len(scopeArgs)
	return fmt.Sprintf("device_id IN (SELECT id FROM devices WHERE %s)", cond), true
}

func (p *Postgres) GetAlert(ctx context.Context, id int64) (*models.Alert, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT `+alertCols+`
		FROM alerts WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts, err := scanAlerts(rows)
	if err != nil {
		return nil, err
	}
	if len(alerts) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &alerts[0], nil
}

func (p *Postgres) CreateAlert(ctx context.Context, a *models.Alert) (*models.Alert, error) {
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO alerts(device_id,device_name,severity,message,status,rule_id)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT (rule_id, device_id) WHERE status = 'active' AND rule_id IS NOT NULL
		DO NOTHING
		RETURNING id`,
		a.DeviceID, a.DeviceName, a.Severity, a.Message, a.Status, a.RuleID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A duplicate active alert already exists for this rule/device —
			// the keyed mutex in the engine normally prevents this, but a
			// restart or multi-instance deployment can still race. The caller
			// treats this as "already fired" and must not double-notify.
			return nil, ErrDuplicateActiveAlert
		}
		return nil, err
	}
	return p.GetAlert(ctx, id)
}

func (p *Postgres) UpdateAlertStatus(ctx context.Context, id int64, status, by string) error {
	switch status {
	case "acknowledged":
		_, err := p.pool.Exec(ctx,
			`UPDATE alerts SET status=$1,acknowledged_at=NOW(),acknowledged_by=$2 WHERE id=$3`,
			status, by, id)
		return err
	case "resolved":
		_, err := p.pool.Exec(ctx,
			`UPDATE alerts SET status=$1,resolved_at=NOW(),resolved_by=$2 WHERE id=$3`,
			status, by, id)
		return err
	default:
		_, err := p.pool.Exec(ctx, `UPDATE alerts SET status=$1 WHERE id=$2`, status, id)
		return err
	}
}

func (p *Postgres) DeleteAlert(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM alerts WHERE id=$1`, id)
	return err
}

func (p *Postgres) FindActiveAlertByRuleAndDevice(ctx context.Context, ruleID, deviceID int64) (*models.Alert, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT `+alertCols+`
		FROM alerts WHERE status='active' AND rule_id=$1 AND device_id=$2
		LIMIT 1`, ruleID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts, err := scanAlerts(rows)
	if err != nil {
		return nil, err
	}
	if len(alerts) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &alerts[0], nil
}

// alertCols is the column list every alert query selects; scanAlerts reads it.
const alertCols = `id,COALESCE(device_id,0),COALESCE(device_name,''),severity,message,status,rule_id,
	created_at,acknowledged_at,resolved_at,acknowledged_by,resolved_by,assigned_to,
	(SELECT COALESCE(NULLIF(u.display_name,''),u.username) FROM users u WHERE u.id = alerts.assigned_to)`

func scanAlerts(rows pgx.Rows) ([]models.Alert, error) {
	var out []models.Alert
	for rows.Next() {
		var a models.Alert
		err := rows.Scan(
			&a.ID, &a.DeviceID, &a.DeviceName, &a.Severity, &a.Message, &a.Status, &a.RuleID,
			&a.CreatedAt, &a.AcknowledgedAt, &a.ResolvedAt, &a.AcknowledgedBy, &a.ResolvedBy,
			&a.AssignedTo, &a.AssignedToName,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── Users & API Keys ──────────────────────────────────────────────────────────

func (p *Postgres) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := p.pool.QueryRow(ctx, `
		SELECT id,username,password_hash,role,COALESCE(display_name,''),COALESCE(email,''),COALESCE(phone,''),enabled,last_login_at,created_at
		FROM users WHERE username=$1`, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role,
		&u.DisplayName, &u.Email, &u.Phone, &u.Enabled, &u.LastLoginAt, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (p *Postgres) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := p.pool.QueryRow(ctx, `
		SELECT id,username,password_hash,role,COALESCE(display_name,''),COALESCE(email,''),COALESCE(phone,''),enabled,last_login_at,created_at
		FROM users WHERE id=$1`, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.Role,
		&u.DisplayName, &u.Email, &u.Phone, &u.Enabled, &u.LastLoginAt, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (p *Postgres) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO users(username,password_hash,role,display_name,email,phone,enabled)
		VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		u.Username, u.PasswordHash, u.Role, nullStr(u.DisplayName),
		nullStr(u.Email), nullStr(u.Phone), u.Enabled).Scan(&id)
	if err != nil {
		return nil, err
	}
	return p.GetUserByID(ctx, id)
}

func (p *Postgres) UpdateUser(ctx context.Context, id int64, u *models.User) (*models.User, error) {
	_, err := p.pool.Exec(ctx, `
		UPDATE users SET username=$1,role=$2,display_name=$3,email=$4,phone=$5,enabled=$6
		WHERE id=$7`,
		u.Username, u.Role, nullStr(u.DisplayName),
		nullStr(u.Email), nullStr(u.Phone), u.Enabled, id)
	if err != nil {
		return nil, err
	}
	return p.GetUserByID(ctx, id)
}

func (p *Postgres) DeleteUser(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

func (p *Postgres) GetAPIKey(ctx context.Context, keyHash string) (*models.APIKey, error) {
	var k models.APIKey
	err := p.pool.QueryRow(ctx, `
		SELECT id,user_id,key_hash,description,created_at,last_used_at,expires_at,revoked_at
		FROM api_keys
		WHERE key_hash=$1 AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())`, keyHash).Scan(
		&k.ID, &k.UserID, &k.KeyHash, &k.Description, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (p *Postgres) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	var k models.APIKey
	err := p.pool.QueryRow(ctx, `
		SELECT id,user_id,key_hash,description,created_at,last_used_at,expires_at,revoked_at
		FROM api_keys WHERE id=$1`, id).Scan(
		&k.ID, &k.UserID, &k.KeyHash, &k.Description, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (p *Postgres) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO api_keys(user_id,key_hash,description,expires_at)
		VALUES($1,$2,$3,$4) RETURNING id`,
		k.UserID, k.KeyHash, nullStr(k.Description), nullTime(k.ExpiresAt)).Scan(&id)
	if err != nil {
		return nil, err
	}
	k.ID = id
	return k, nil
}

func (p *Postgres) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,user_id,key_hash,description,created_at,last_used_at,expires_at,revoked_at
		FROM api_keys WHERE user_id=$1 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Description, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (p *Postgres) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	return err
}

func (p *Postgres) RevokeAPIKey(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `UPDATE api_keys SET revoked_at=NOW(), updated_at=NOW() WHERE id=$1 AND revoked_at IS NULL`, id)
	return err
}

// ── Flows ─────────────────────────────────────────────────────────────────────

// flowColumns is the column list RecordFlows copies into flows.
var flowColumns = []string{"src_ip", "dst_ip", "src_port", "dst_port", "protocol", "bytes", "packets", "duration", "created_at",
	"exporter", "device_id", "in_if", "out_if", "tcp_flags", "tos", "sampling_rate"}

// RecordFlows bulk-inserts flows with COPY. Empty IP strings are stored as
// NULL; a malformed IP fails the whole batch.
func (p *Postgres) RecordFlows(ctx context.Context, flows []models.Flow) error {
	if len(flows) == 0 {
		return nil
	}
	now := time.Now()
	_, err := p.pool.CopyFrom(ctx, pgx.Identifier{"flows"}, flowColumns,
		pgx.CopyFromSlice(len(flows), func(i int) ([]any, error) {
			f := flows[i]
			src, err := flowAddr(f.SrcIP)
			if err != nil {
				return nil, err
			}
			dst, err := flowAddr(f.DstIP)
			if err != nil {
				return nil, err
			}
			var exporter any
			if f.Exporter != nil {
				if exporter, err = flowAddr(*f.Exporter); err != nil {
					return nil, err
				}
			}
			ts := f.Timestamp
			if ts.IsZero() {
				ts = now
			}
			return []any{src, dst, f.SrcPort, f.DstPort, f.Protocol, f.Bytes, f.Packets, f.Duration, ts,
				exporter, f.DeviceID, f.InIf, f.OutIf, smallint(f.TCPFlags), smallint(f.ToS), f.SamplingRate}, nil
		}))
	return err
}

// flowAddr parses an IP for an INET column; "" becomes NULL.
func flowAddr(s string) (any, error) {
	if s == "" {
		return nil, nil
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("flow: invalid IP %q: %w", s, err)
	}
	return a, nil
}

func smallint(v *int) *int16 {
	if v == nil {
		return nil
	}
	n := int16(*v) //nolint:gosec // TCP flags and ToS are single bytes
	return &n
}

func (p *Postgres) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM flows WHERE created_at BETWEEN $1 AND $2`, from, to).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id,COALESCE(host(src_ip),''),COALESCE(host(dst_ip),''),src_port,dst_port,protocol,bytes,packets,duration,created_at,
		       host(exporter),device_id,in_if,out_if,tcp_flags,tos,sampling_rate
		FROM flows WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, from, to, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Flow
	for rows.Next() {
		var f models.Flow
		var tcpFlags, tos *int16
		if err := rows.Scan(&f.ID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort,
			&f.Protocol, &f.Bytes, &f.Packets, &f.Duration, &f.Timestamp,
			&f.Exporter, &f.DeviceID, &f.InIf, &f.OutIf, &tcpFlags, &tos, &f.SamplingRate); err != nil {
			return nil, 0, err
		}
		f.TCPFlags, f.ToS = widen(tcpFlags), widen(tos)
		out = append(out, f)
	}
	return out, total, rows.Err()
}

func widen(v *int16) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func (p *Postgres) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	if n <= 0 {
		n = 10
	}
	rows, err := p.pool.Query(ctx, `
		SELECT host(src_ip), SUM(bytes) AS total
		FROM flows WHERE created_at BETWEEN $1 AND $2 AND src_ip IS NOT NULL
		GROUP BY src_ip ORDER BY total DESC LIMIT $3`, from, to, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.IPCount
	for rows.Next() {
		var c models.IPCount
		if err := rows.Scan(&c.IP, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *Postgres) GetProtocolStats(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT protocol, SUM(bytes) FROM flows
		WHERE created_at BETWEEN $1 AND $2
		GROUP BY protocol`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var proto string
		var bytes int64
		if err := rows.Scan(&proto, &bytes); err != nil {
			return nil, err
		}
		out[proto] = bytes
	}
	return out, rows.Err()
}

// ── Dashboards ────────────────────────────────────────────────────────────────

func (p *Postgres) GetDashboards(ctx context.Context, userID int64) ([]models.Dashboard, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,user_id,name,layout,created_at,updated_at
		FROM dashboards WHERE user_id=$1 ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDashboards(rows)
}

func (p *Postgres) GetDashboard(ctx context.Context, id int64) (*models.Dashboard, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,user_id,name,layout,created_at,updated_at
		FROM dashboards WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ds, err := scanDashboards(rows)
	if err != nil {
		return nil, err
	}
	if len(ds) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &ds[0], nil
}

func (p *Postgres) SaveDashboard(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
	if d.ID == 0 {
		var id int64
		err := p.pool.QueryRow(ctx, `
			INSERT INTO dashboards(user_id,name,layout) VALUES($1,$2,$3) RETURNING id`,
			d.UserID, d.Name, d.Layout).Scan(&id)
		if err != nil {
			return nil, err
		}
		return p.GetDashboard(ctx, id)
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE dashboards SET name=$1,layout=$2,updated_at=NOW() WHERE id=$3`,
		d.Name, d.Layout, d.ID)
	if err != nil {
		return nil, err
	}
	return p.GetDashboard(ctx, d.ID)
}

func (p *Postgres) DeleteDashboard(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM dashboards WHERE id=$1`, id)
	return err
}

func scanDashboards(rows pgx.Rows) ([]models.Dashboard, error) {
	var out []models.Dashboard
	for rows.Next() {
		var d models.Dashboard
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Layout, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ── Retention ─────────────────────────────────────────────────────────────────

// baselineFields lists the metric columns that get anomaly baselines.
var baselineFields = []string{"response_time", "packet_loss", "cpu_usage", "memory_usage", "bandwidth"}

// GetMetricBaselines aggregates the window in one pass in the database
// instead of shipping every raw row (including details JSONB) to Go for each
// device (B18).
func (p *Postgres) GetMetricBaselines(ctx context.Context, since time.Time) ([]MetricBaseline, error) {
	var cols strings.Builder
	for _, f := range baselineFields {
		fmt.Fprintf(&cols, ", count(%[1]s), COALESCE(avg(%[1]s), 0), COALESCE(stddev_pop(%[1]s), 0)", f)
	}
	rows, err := p.pool.Query(ctx, `
		SELECT m.device_id`+cols.String()+`
		FROM metrics m
		JOIN devices d ON d.id = m.device_id AND d.enabled
		WHERE m.timestamp > $1
		GROUP BY m.device_id`, since) //nolint:gosec // columns come from the constant baselineFields list
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricBaseline
	n := len(baselineFields)
	counts := make([]int, n)
	means := make([]float64, n)
	stddevs := make([]float64, n)
	dest := make([]any, 0, 1+3*n)
	var deviceID int64
	dest = append(dest, &deviceID)
	for i := range baselineFields {
		dest = append(dest, &counts[i], &means[i], &stddevs[i])
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		for i, f := range baselineFields {
			if counts[i] > 0 {
				out = append(out, MetricBaseline{DeviceID: deviceID, Field: f, Mean: means[i], StdDev: stddevs[i], Count: counts[i]})
			}
		}
	}
	return out, rows.Err()
}

// pruneBatched deletes rows older than cutoff from a plain table in batches
// so one sweep never holds a huge transaction (M35).
func (p *Postgres) pruneBatched(ctx context.Context, query string, cutoff time.Time) (int64, error) {
	var total int64
	const batchSize = 10000
	for {
		t, err := p.pool.Exec(ctx, query, cutoff, batchSize)
		if err != nil {
			return total, err
		}
		n := t.RowsAffected()
		total += n
		if n < batchSize {
			return total, nil
		}
	}
}

func (p *Postgres) PruneHealthScoreHistory(ctx context.Context, olderThan time.Time) (int64, error) {
	return p.pruneBatched(ctx,
		`DELETE FROM health_score_history WHERE id IN (SELECT id FROM health_score_history WHERE computed_at < $1 LIMIT $2)`,
		olderThan)
}

// PruneCaptureSessions deletes finished capture sessions; their packets go
// with them via ON DELETE CASCADE.
func (p *Postgres) PruneCaptureSessions(ctx context.Context, olderThan time.Time) (int64, error) {
	return p.pruneBatched(ctx,
		`DELETE FROM capture_sessions WHERE id IN (SELECT id FROM capture_sessions WHERE status <> 'running' AND started_at < $1 LIMIT $2)`,
		olderThan)
}

func (p *Postgres) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	t, err := p.pool.Exec(ctx, `DELETE FROM alerts WHERE status='resolved' AND created_at < $1`, olderThan)
	return t.RowsAffected(), err
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func (p *Postgres) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	since := time.Now().Add(-24 * time.Hour)
	var totalDevices, onlineDevices, offlineDevices, activeAlerts, totalMetrics24h int64
	var avgRT *float64

	err := p.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM devices),
			(SELECT COUNT(*) FROM devices WHERE status='up'),
			(SELECT COUNT(*) FROM devices WHERE status='down'),
			(SELECT COUNT(*) FROM alerts WHERE status='active'),
			(SELECT COALESCE(SUM(samples), 0) FROM metrics_hourly WHERE bucket >= date_trunc('hour', $1::timestamptz)),
			(SELECT SUM(rt_sum) / NULLIF(SUM(rt_count), 0) FROM metrics_hourly WHERE bucket >= date_trunc('hour', $1::timestamptz))
	`, since).Scan(&totalDevices, &onlineDevices, &offlineDevices, &activeAlerts, &totalMetrics24h, &avgRT)
	if err != nil {
		return nil, fmt.Errorf("dashboard_stats query failed: %w", err)
	}

	return map[string]any{
		"totalDevices":    totalDevices,
		"onlineDevices":   onlineDevices,
		"offlineDevices":  offlineDevices,
		"activeAlerts":    activeAlerts,
		"totalMetrics24h": totalMetrics24h,
		"avgResponseTime": avgRT,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

func nullTime(t *time.Time) *time.Time {
	return t
}
