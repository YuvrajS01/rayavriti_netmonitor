package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type Postgres struct {
	pool *pgxpool.Pool
	dsn  string
}

func NewPostgres(dsn string) *Postgres {
	return &Postgres{dsn: dsn}
}

func (p *Postgres) Connect(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(p.dsn)
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	p.pool = pool
	return p.Ping(ctx)
}

func (p *Postgres) Close() error {
	if p.pool != nil {
		p.pool.Close()
	}
	return nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Postgres) RunMigrations(ctx context.Context) error {
	// ensure tracking table exists first
	if _, err := p.pool.Exec(ctx, migrations[0]); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for i, sql := range migrations[1:] {
		version := int64(i + 2) // 1-based, but index 0 is already applied above
		var exists bool
		err := p.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists {
			continue
		}
		// split on semicolons for multi-statement migrations
		for _, stmt := range splitStatements(sql) {
			if _, err := p.pool.Exec(ctx, stmt); err != nil {
				// TimescaleDB hypertable errors are non-fatal if table already partitioned
				if strings.Contains(err.Error(), "already a hypertable") {
					continue
				}
				return fmt.Errorf("migration %d: %w\nSQL: %s", version, err, stmt)
			}
		}
		if _, err := p.pool.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1) ON CONFLICT DO NOTHING`, version); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}
	return nil
}

func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	var out []string
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ── Devices ──────────────────────────────────────────────────────────────────

const deviceSelectCols = `
		SELECT id,name,ip_address,protocol,enabled,status,tags,
		       COALESCE(snmp_community,''),COALESCE(snmp_version,''),COALESCE(snmp_port,0),COALESCE(http_path,''),COALESCE(http_expected_status,0),
		       interval_sec,location_id,parent_device_id,COALESCE(rack_position,''),COALESCE(asset_tag,''),
		       COALESCE(mac_address,''),COALESCE(manufacturer,''),COALESCE(model,''),COALESCE(device_category,''),COALESCE(notes,''),created_at,updated_at
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
		INSERT INTO devices(name,ip_address,protocol,enabled,tags,snmp_community,snmp_version,
		                    snmp_port,http_path,http_expected_status,interval_sec,
		                    location_id,rack_position,asset_tag,mac_address,manufacturer,
		                    model,device_category,notes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id`,
		d.Name, d.IPAddress, d.Protocol, d.Enabled, tags,
		nullStr(d.SNMPCommunity), nullStr(d.SNMPVersion), nullInt(d.SNMPPort),
		nullStr(d.HTTPPath), nullInt(d.HTTPExpectedStatus), d.Interval,
		d.LocationID, nullStr(d.RackPosition), nullStr(d.AssetTag),
		nullStr(d.MACAddress), nullStr(d.Manufacturer), nullStr(d.Model),
		nullStr(d.DeviceCategory), nullStr(d.Notes),
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return p.GetDevice(ctx, id)
}

func (p *Postgres) UpdateDevice(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
	tags, _ := json.Marshal(d.Tags)
	_, err := p.pool.Exec(ctx, `
		UPDATE devices SET name=$1,ip_address=$2,protocol=$3,enabled=$4,tags=$5,
		    snmp_community=$6,snmp_version=$7,snmp_port=$8,http_path=$9,
		    http_expected_status=$10,interval_sec=$11,location_id=$12,
		    rack_position=$13,asset_tag=$14,mac_address=$15,manufacturer=$16,
		    model=$17,device_category=$18,notes=$19,updated_at=NOW()
		WHERE id=$20`,
		d.Name, d.IPAddress, d.Protocol, d.Enabled, tags,
		nullStr(d.SNMPCommunity), nullStr(d.SNMPVersion), nullInt(d.SNMPPort),
		nullStr(d.HTTPPath), nullInt(d.HTTPExpectedStatus), d.Interval,
		d.LocationID, nullStr(d.RackPosition), nullStr(d.AssetTag),
		nullStr(d.MACAddress), nullStr(d.Manufacturer), nullStr(d.Model),
		nullStr(d.DeviceCategory), nullStr(d.Notes), id)
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
		var tagsRaw []byte
		err := rows.Scan(
			&d.ID, &d.Name, &d.IPAddress, &d.Protocol, &d.Enabled, &d.Status, &tagsRaw,
			&d.SNMPCommunity, &d.SNMPVersion, &d.SNMPPort, &d.HTTPPath, &d.HTTPExpectedStatus,
			&d.Interval, &d.LocationID, &d.ParentDeviceID, &d.RackPosition, &d.AssetTag,
			&d.MACAddress, &d.Manufacturer, &d.Model, &d.DeviceCategory, &d.Notes,
			&d.CreatedAt, &d.UpdatedAt,
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
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateDeviceStatus updates only the status field
func (p *Postgres) UpdateDeviceStatus(ctx context.Context, id int64, status string) error {
	_, err := p.pool.Exec(ctx, `UPDATE devices SET status=$1,updated_at=NOW() WHERE id=$2`, status, id)
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

func (p *Postgres) GetLatestMetrics(ctx context.Context) ([]models.Metric, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT DISTINCT ON (m.device_id)
		    m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		    m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		    d.protocol, d.name
		FROM metrics m
		LEFT JOIN devices d ON d.id = m.device_id
		ORDER BY m.device_id, m.timestamp DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
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
		LEFT JOIN devices d ON d.id = m.device_id
		WHERE m.device_id=$1 AND m.timestamp BETWEEN $2 AND $3
		ORDER BY m.timestamp DESC LIMIT $4`,
		deviceID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
}

func (p *Postgres) GetMetricsSummary(ctx context.Context, from, to time.Time) (map[string]any, error) {
	var total int64
	var avgRT *float64
	err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*), AVG(response_time)
		FROM metrics WHERE timestamp BETWEEN $1 AND $2`, from, to).Scan(&total, &avgRT)
	if err != nil {
		return nil, err
	}
	return map[string]any{"total": total, "avgResponseTime": avgRT}, nil
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

func (p *Postgres) GetAlerts(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
	if limit <= 0 {
		limit = 50
	}
	base := `FROM alerts`
	args := []any{}
	if status != "" {
		base += ` WHERE status=$1`
		args = append(args, status)
	}
	var total int
	countSQL := `SELECT COUNT(*) ` + base
	if err := p.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listSQL := `SELECT id,device_id,device_name,severity,message,status,rule_id,
	                   created_at,acknowledged_at,resolved_at,acknowledged_by,resolved_by ` +
		base + ` ORDER BY created_at DESC`
	n := len(args)
	listSQL += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, n+1, n+2)
	args = append(args, limit, offset)
	rows, err := p.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	alerts, err := scanAlerts(rows)
	return alerts, total, err
}

func (p *Postgres) GetAlert(ctx context.Context, id int64) (*models.Alert, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,device_id,device_name,severity,message,status,rule_id,
		       created_at,acknowledged_at,resolved_at,acknowledged_by,resolved_by
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
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		a.DeviceID, a.DeviceName, a.Severity, a.Message, a.Status, a.RuleID,
	).Scan(&id)
	if err != nil {
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

func scanAlerts(rows pgx.Rows) ([]models.Alert, error) {
	var out []models.Alert
	for rows.Next() {
		var a models.Alert
		err := rows.Scan(
			&a.ID, &a.DeviceID, &a.DeviceName, &a.Severity, &a.Message, &a.Status, &a.RuleID,
			&a.CreatedAt, &a.AcknowledgedAt, &a.ResolvedAt, &a.AcknowledgedBy, &a.ResolvedBy,
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
		SELECT id,user_id,key_hash,description,created_at,last_used_at
		FROM api_keys WHERE key_hash=$1`, keyHash).Scan(
		&k.ID, &k.UserID, &k.KeyHash, &k.Description, &k.CreatedAt, &k.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (p *Postgres) CreateAPIKey(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO api_keys(user_id,key_hash,description) VALUES($1,$2,$3) RETURNING id`,
		k.UserID, k.KeyHash, nullStr(k.Description)).Scan(&id)
	if err != nil {
		return nil, err
	}
	k.ID = id
	return k, nil
}

func (p *Postgres) GetAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,user_id,key_hash,description,created_at,last_used_at
		FROM api_keys WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Description, &k.CreatedAt, &k.LastUsedAt); err != nil {
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

// ── Flows ─────────────────────────────────────────────────────────────────────

func (p *Postgres) RecordFlows(ctx context.Context, flows []models.Flow) error {
	if len(flows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, f := range flows {
		batch.Queue(`
			INSERT INTO flows(src_ip,dst_ip,src_port,dst_port,protocol,bytes,packets,duration,created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			f.SrcIP, f.DstIP, f.SrcPort, f.DstPort, f.Protocol, f.Bytes, f.Packets, f.Duration, f.Timestamp)
	}
	br := p.pool.SendBatch(ctx, batch)
	return br.Close()
}

func (p *Postgres) GetFlows(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
	if limit <= 0 {
		limit = 100
	}
	var total int
	if err := p.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM flows WHERE created_at BETWEEN $1 AND $2`, from, to).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id,src_ip,dst_ip,src_port,dst_port,protocol,bytes,packets,duration,created_at
		FROM flows WHERE created_at BETWEEN $1 AND $2
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, from, to, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Flow
	for rows.Next() {
		var f models.Flow
		if err := rows.Scan(&f.ID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort,
			&f.Protocol, &f.Bytes, &f.Packets, &f.Duration, &f.Timestamp); err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

func (p *Postgres) GetTopTalkers(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
	if n <= 0 {
		n = 10
	}
	rows, err := p.pool.Query(ctx, `
		SELECT src_ip::text, SUM(bytes) AS total
		FROM flows WHERE created_at BETWEEN $1 AND $2
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

func (p *Postgres) PruneMetrics(ctx context.Context, olderThan time.Time) (int64, error) {
	t, err := p.pool.Exec(ctx, `DELETE FROM metrics WHERE timestamp < $1`, olderThan)
	return t.RowsAffected(), err
}

func (p *Postgres) PruneFlows(ctx context.Context, olderThan time.Time) (int64, error) {
	t, err := p.pool.Exec(ctx, `DELETE FROM flows WHERE created_at < $1`, olderThan)
	return t.RowsAffected(), err
}

func (p *Postgres) PruneAlerts(ctx context.Context, olderThan time.Time) (int64, error) {
	t, err := p.pool.Exec(ctx, `DELETE FROM alerts WHERE status='resolved' AND created_at < $1`, olderThan)
	return t.RowsAffected(), err
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func (p *Postgres) GetDashboardStats(ctx context.Context) (map[string]any, error) {
	var totalDevices, onlineDevices, offlineDevices, activeAlerts int64
	var totalMetrics24h int64
	var avgRT *float64

	since := time.Now().Add(-24 * time.Hour)

	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices`).Scan(&totalDevices); err != nil {
		slog.Error("dashboard_stats: count devices", "error", err)
	}
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE status='up'`).Scan(&onlineDevices); err != nil {
		slog.Error("dashboard_stats: count online devices", "error", err)
	}
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE status='down'`).Scan(&offlineDevices); err != nil {
		slog.Error("dashboard_stats: count offline devices", "error", err)
	}
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alerts WHERE status='active'`).Scan(&activeAlerts); err != nil {
		slog.Error("dashboard_stats: count active alerts", "error", err)
	}
	if err := p.pool.QueryRow(ctx, `SELECT COUNT(*) FROM metrics WHERE timestamp > $1`, since).Scan(&totalMetrics24h); err != nil {
		slog.Error("dashboard_stats: count metrics 24h", "error", err)
	}
	if err := p.pool.QueryRow(ctx, `SELECT AVG(response_time) FROM metrics WHERE timestamp > $1`, since).Scan(&avgRT); err != nil {
		slog.Error("dashboard_stats: avg response time", "error", err)
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
