package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── Device extended ──────────────────────────────────────────────────────────

// GetEnabledDevices returns all devices with enabled=TRUE, ordered by id.
func (p *Postgres) GetEnabledDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelectCols+` WHERE enabled=TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices, err := scanDevices(rows)
	if err != nil {
		return nil, err
	}
	if devices == nil {
		devices = []models.Device{}
	}
	return devices, nil
}

// GetDevicesByStatus returns all devices matching the given status, ordered by id.
func (p *Postgres) GetDevicesByStatus(ctx context.Context, status string) ([]models.Device, error) {
	rows, err := p.pool.Query(ctx, deviceSelectCols+` WHERE status=$1 ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices, err := scanDevices(rows)
	if err != nil {
		return nil, err
	}
	if devices == nil {
		devices = []models.Device{}
	}
	return devices, nil
}

func (p *Postgres) GetDevicesFiltered(ctx context.Context, f DeviceFilter) ([]models.Device, int, error) {
	where := []string{}
	args := []any{}
	argN := 1

	if f.Status != "" {
		where = append(where, fmt.Sprintf("status=$%d", argN))
		args = append(args, f.Status)
		argN++
	}
	if f.Protocol != "" {
		where = append(where, fmt.Sprintf("protocol=$%d", argN))
		args = append(args, f.Protocol)
		argN++
	}
	if f.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled=$%d", argN))
		args = append(args, *f.Enabled)
		argN++
	}
	if f.Search != "" {
		where = append(where, fmt.Sprintf("(LOWER(name) LIKE $%d OR LOWER(ip_address) LIKE $%d)", argN, argN))
		args = append(args, "%"+strings.ToLower(f.Search)+"%")
		argN++
	}
	if f.LocationID != nil {
		where = append(where, fmt.Sprintf("location_id=$%d", argN))
		args = append(args, *f.LocationID)
		argN++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM devices" + whereClause
	if err := p.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortCol := "id"
	switch f.SortBy {
	case "name":
		sortCol = "name"
	case "status":
		sortCol = "status"
	case "protocol":
		sortCol = "protocol"
	case "ipAddress":
		sortCol = "ip_address"
	}
	sortDir := "ASC"
	if strings.EqualFold(f.SortDir, "desc") {
		sortDir = "DESC"
	}

	query := deviceSelectCols + whereClause + fmt.Sprintf(" ORDER BY %s %s", sortCol, sortDir)
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", f.Offset)
	}

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	devices, err := scanDevices(rows)
	if err != nil {
		return nil, 0, err
	}
	if devices == nil {
		devices = []models.Device{}
	}
	return devices, total, nil
}

// ── Alert extended ───────────────────────────────────────────────────────────

// GetAlertCounts returns counts of alerts grouped by status.
func (p *Postgres) GetAlertCounts(ctx context.Context) (models.AlertCounts, error) {
	var counts models.AlertCounts

	rows, err := p.pool.Query(ctx, `SELECT status, COUNT(*) FROM alerts GROUP BY status`)
	if err != nil {
		return counts, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return counts, err
		}
		switch status {
		case "active":
			counts.Active = n
		case "acknowledged":
			counts.Acknowledged = n
		case "resolved":
			counts.Resolved = n
		}
	}
	return counts, rows.Err()
}

// FindActiveAlert returns the first active alert for a device with the given message,
// or nil if none exists.
func (p *Postgres) FindActiveAlert(ctx context.Context, deviceID int64, message string) (*models.Alert, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,device_id,device_name,severity,message,status,rule_id,
		       created_at,acknowledged_at,resolved_at,acknowledged_by,resolved_by
		FROM alerts
		WHERE device_id=$1 AND message=$2 AND status='active'
		LIMIT 1`, deviceID, message)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	alerts, err := scanAlerts(rows)
	if err != nil {
		return nil, err
	}
	if len(alerts) == 0 {
		return nil, nil
	}
	return &alerts[0], nil
}

// GetAlertsForReport returns alerts for reporting within a time range.
func (p *Postgres) GetAlertsForReport(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
	args := []any{from, to}
	query := `
		SELECT id,COALESCE(device_id,0),COALESCE(device_name,''),severity,message,status,COALESCE(rule_id,0),
		       created_at,acknowledged_at,resolved_at,acknowledged_by,resolved_by
		FROM alerts
		WHERE created_at BETWEEN $1 AND $2`

	paramIdx := 3
	if deviceID != nil {
		query += fmt.Sprintf(` AND device_id=$%d`, paramIdx)
		args = append(args, *deviceID)
	}
	query += ` ORDER BY created_at DESC LIMIT 5000`

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAlerts(rows)
}

// ── Metrics extended ─────────────────────────────────────────────────────────

// GetMetricsForReport returns metric rows joined with device info for reporting.
// Results are filtered by time range and optionally by device_id.
func (p *Postgres) GetMetricsForReport(ctx context.Context, from, to time.Time, deviceID *int64, interval string) ([]models.ReportMetricRow, error) {
	args := []any{from, to}
	query := `
		SELECT m.device_id, d.name, d.protocol, m.status,
		       m.response_time, m.custom_value, '', m.timestamp
		FROM metrics m
		JOIN devices d ON d.id = m.device_id
		WHERE m.timestamp BETWEEN $1 AND $2`

	paramIdx := 3
	if deviceID != nil {
		query += fmt.Sprintf(` AND m.device_id=$%d`, paramIdx)
		args = append(args, *deviceID)
	}
	query += ` ORDER BY m.timestamp DESC LIMIT 5000`

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ReportMetricRow
	for rows.Next() {
		var r models.ReportMetricRow
		var ts time.Time
		if err := rows.Scan(
			&r.DeviceID, &r.DeviceName, &r.Protocol, &r.Status,
			&r.ResponseTime, &r.Value, &r.Message, &ts,
		); err != nil {
			return nil, err
		}
		r.Timestamp = ts.Format(time.RFC3339)
		out = append(out, r)
	}
	if out == nil {
		out = []models.ReportMetricRow{}
	}
	return out, rows.Err()
}

func (p *Postgres) GetReportTimeseries(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 60
	}
	bucketSec := int64(bucketMinutes) * 60

	query := `
		SELECT to_timestamp(EXTRACT(EPOCH FROM timestamp)::bigint / $1 * $1) AS bucket,
		       COUNT(*)::int,
		       COALESCE(AVG(response_time), 0),
		       COUNT(*) FILTER (WHERE status = 'down')::int,
		       COUNT(*) FILTER (WHERE status IN ('warning','degraded'))::int
		FROM metrics
		WHERE timestamp BETWEEN $2 AND $3`
	args := []any{bucketSec, from, to}
	if deviceID != nil {
		query += ` AND device_id=$4`
		args = append(args, *deviceID)
	}
	query += ` GROUP BY bucket ORDER BY bucket ASC`

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.ReportTimeseriesPoint
	for rows.Next() {
		var p models.ReportTimeseriesPoint
		var bucket time.Time
		if err := rows.Scan(&bucket, &p.SampleCount, &p.AvgResponse, &p.DownCount, &p.WarnCount); err != nil {
			return nil, err
		}
		p.BucketTime = bucket.Format(time.RFC3339)
		if p.SampleCount > 0 {
			p.AvailabilityPercent = float64(p.SampleCount-p.DownCount) / float64(p.SampleCount) * 100
		}
		out = append(out, p)
	}
	if out == nil {
		out = []models.ReportTimeseriesPoint{}
	}
	return out, rows.Err()
}

func (p *Postgres) GetReportDeviceBreakdown(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
	query := `
		SELECT m.device_id, COALESCE(d.name, 'Unknown'), COALESCE(d.protocol, ''),
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE m.status = 'down')::int,
		       COUNT(*) FILTER (WHERE m.status IN ('warning','degraded'))::int,
		       COALESCE(AVG(m.response_time), 0),
		       COALESCE(MIN(m.response_time), 0),
		       COALESCE(MAX(m.response_time), 0)
		FROM metrics m
		LEFT JOIN devices d ON d.id = m.device_id
		WHERE m.timestamp BETWEEN $1 AND $2`
	args := []any{from, to}
	if deviceID != nil {
		query += ` AND m.device_id=$3`
		args = append(args, *deviceID)
	}
	query += ` GROUP BY m.device_id, d.name, d.protocol ORDER BY COUNT(*) DESC`

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.DeviceBreakdown
	for rows.Next() {
		var db models.DeviceBreakdown
		if err := rows.Scan(
			&db.DeviceID, &db.DeviceName, &db.Protocol,
			&db.SampleCount, &db.DownCount, &db.WarnCount,
			&db.AvgResponse, &db.MinResponse, &db.MaxResponse,
		); err != nil {
			return nil, err
		}
		if db.SampleCount > 0 {
			db.AvailabilityPercent = float64(db.SampleCount-db.DownCount) / float64(db.SampleCount) * 100
		} else {
			db.AvailabilityPercent = 100
		}
		out = append(out, db)
	}
	if out == nil {
		out = []models.DeviceBreakdown{}
	}
	return out, rows.Err()
}

// QueryMetrics builds a dynamic query to retrieve metrics with optional filters.
func (p *Postgres) QueryMetrics(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
	var clauses []string
	args := []any{}
	paramIdx := 1

	if q.DeviceID != nil {
		clauses = append(clauses, fmt.Sprintf("device_id=$%d", paramIdx))
		args = append(args, *q.DeviceID)
		paramIdx++
	}
	if !q.From.IsZero() {
		clauses = append(clauses, fmt.Sprintf("timestamp >= $%d", paramIdx))
		args = append(args, q.From)
		paramIdx++
	}
	if !q.To.IsZero() {
		clauses = append(clauses, fmt.Sprintf("timestamp <= $%d", paramIdx))
		args = append(args, q.To)
		paramIdx++
	}
	if q.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status=$%d", paramIdx))
		args = append(args, q.Status)
		paramIdx++
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}

	var query string
	if q.BucketMin > 0 {
		bucketSec := int64(q.BucketMin) * 60
		switch q.Aggregation {
		case "avg":
			query = fmt.Sprintf(`
				SELECT 0,device_id,
				       to_timestamp(EXTRACT(EPOCH FROM timestamp)::bigint / %d * %d),
				       '',AVG(response_time),AVG(packet_loss),
				       AVG(cpu_usage),AVG(memory_usage),AVG(bandwidth),AVG(custom_value),NULL
				FROM metrics`, bucketSec, bucketSec)
		case "max":
			query = fmt.Sprintf(`
				SELECT 0,device_id,
				       to_timestamp(EXTRACT(EPOCH FROM timestamp)::bigint / %d * %d),
				       '',MAX(response_time),MAX(packet_loss),
				       MAX(cpu_usage),MAX(memory_usage),MAX(bandwidth),MAX(custom_value),NULL
				FROM metrics`, bucketSec, bucketSec)
		case "min":
			query = fmt.Sprintf(`
				SELECT 0,device_id,
				       to_timestamp(EXTRACT(EPOCH FROM timestamp)::bigint / %d * %d),
				       '',MIN(response_time),MIN(packet_loss),
				       MIN(cpu_usage),MIN(memory_usage),MIN(bandwidth),MIN(custom_value),NULL
				FROM metrics`, bucketSec, bucketSec)
		default:
			query = fmt.Sprintf(`
				SELECT 0,device_id,
				       to_timestamp(EXTRACT(EPOCH FROM timestamp)::bigint / %d * %d),
				       '',AVG(response_time),AVG(packet_loss),
				       AVG(cpu_usage),AVG(memory_usage),AVG(bandwidth),AVG(custom_value),NULL
				FROM metrics`, bucketSec, bucketSec)
		}
		if len(clauses) > 0 {
			query += " WHERE " + strings.Join(clauses, " AND ")
		}
		query += fmt.Sprintf(" GROUP BY device_id, bucket ORDER BY bucket DESC LIMIT $%d", paramIdx)
		args = append(args, limit)
	} else {
		query = `SELECT id,device_id,timestamp,status,response_time,packet_loss,
		                cpu_usage,memory_usage,bandwidth,custom_value,details
		         FROM metrics`
		if len(clauses) > 0 {
			query += " WHERE " + strings.Join(clauses, " AND ")
		}
		query += " ORDER BY timestamp DESC"
		query += fmt.Sprintf(" LIMIT $%d", paramIdx)
		args = append(args, limit)
	}

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics, err := scanMetrics(rows)
	if err != nil {
		return nil, err
	}
	if metrics == nil {
		metrics = []models.Metric{}
	}
	return metrics, nil
}

func (p *Postgres) ExportMetrics(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
	if limit <= 0 {
		limit = 5000
	}
	query := `
		SELECT m.id, m.device_id, m.timestamp, m.status, m.response_time, m.packet_loss,
		       m.cpu_usage, m.memory_usage, m.bandwidth, m.custom_value, m.details,
		       d.protocol, d.name
		FROM metrics m
		LEFT JOIN devices d ON d.id = m.device_id
		WHERE m.timestamp BETWEEN $1 AND $2`
	args := []any{from, to}
	paramIdx := 3
	if deviceID != nil {
		query += fmt.Sprintf(` AND m.device_id=$%d`, paramIdx)
		args = append(args, *deviceID)
		paramIdx++
	}
	query += fmt.Sprintf(` ORDER BY m.timestamp DESC LIMIT $%d`, paramIdx)
	args = append(args, limit)

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetricsWithDevice(rows)
}

// GetMetricsInWindow extracts a specific numeric field's values from metrics
// within a time window for a device. Returns the values as []float64.
func (p *Postgres) GetMetricsInWindow(ctx context.Context, deviceID int64, field string, from, to time.Time) ([]float64, error) {
	col, err := metricFieldToColumn(field)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT %s FROM metrics
		WHERE device_id=$1 AND timestamp BETWEEN $2 AND $3
		  AND %s IS NOT NULL
		ORDER BY timestamp ASC`, col, col)

	rows, err := p.pool.Query(ctx, query, deviceID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if out == nil {
		out = []float64{}
	}
	return out, rows.Err()
}

// metricFieldToColumn maps user-facing field names to actual column names.
// Returns an error for unrecognized fields to prevent SQL injection.
func metricFieldToColumn(field string) (string, error) {
	switch field {
	case "response_time":
		return "response_time", nil
	case "cpu_usage":
		return "cpu_usage", nil
	case "memory_usage":
		return "memory_usage", nil
	case "packet_loss":
		return "packet_loss", nil
	case "bandwidth":
		return "bandwidth", nil
	case "custom_value":
		return "custom_value", nil
	default:
		return "", fmt.Errorf("unknown metric field: %s", field)
	}
}

// ── Flow extended ────────────────────────────────────────────────────────────

// GetFlowTimeseries returns flow data bucketed into time intervals.
// The interval string supports formats like "5m", "1h", "1d".
func (p *Postgres) GetFlowTimeseries(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
	bucketSec, err := parseIntervalSeconds(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", interval, err)
	}

	rows, err := p.pool.Query(ctx, `
		SELECT
		    to_timestamp(EXTRACT(EPOCH FROM created_at)::bigint / $1 * $1) AS bucket,
		    COALESCE(SUM(bytes), 0),
		    COALESCE(SUM(packets), 0),
		    COUNT(*)
		FROM flows
		WHERE created_at BETWEEN $2 AND $3
		GROUP BY bucket
		ORDER BY bucket ASC`, bucketSec, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.FlowTimeseriesPoint
	for rows.Next() {
		var pt models.FlowTimeseriesPoint
		var ts time.Time
		if err := rows.Scan(&ts, &pt.TotalBytes, &pt.TotalPackets, &pt.FlowCount); err != nil {
			return nil, err
		}
		pt.BucketTime = ts.Format(time.RFC3339)
		out = append(out, pt)
	}
	if out == nil {
		out = []models.FlowTimeseriesPoint{}
	}
	return out, rows.Err()
}

// GetFlowStats returns aggregate statistics for flows within a time range.
func (p *Postgres) GetFlowStats(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
	var stats models.FlowSummaryStats

	err := p.pool.QueryRow(ctx, `
		SELECT
		    COUNT(*),
		    COALESCE(SUM(bytes), 0),
		    COALESCE(SUM(packets), 0),
		    COUNT(DISTINCT src_ip),
		    COUNT(DISTINCT dst_ip)
		FROM flows
		WHERE created_at BETWEEN $1 AND $2`, from, to).Scan(
		&stats.TotalFlows,
		&stats.TotalBytes,
		&stats.TotalPackets,
		&stats.UniqueSources,
		&stats.UniqueDestinations,
	)
	if err != nil {
		return stats, err
	}
	return stats, nil
}

// parseIntervalSeconds converts a duration string like "5m", "1h", "1d" to seconds.
func parseIntervalSeconds(interval string) (int64, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("interval too short: %s", interval)
	}

	suffix := interval[len(interval)-1]
	numStr := interval[:len(interval)-1]
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in interval: %w", err)
	}
	if num <= 0 {
		return 0, fmt.Errorf("interval must be positive")
	}

	switch suffix {
	case 's':
		return num, nil
	case 'm':
		return num * 60, nil
	case 'h':
		return num * 3600, nil
	case 'd':
		return num * 86400, nil
	default:
		return 0, fmt.Errorf("unsupported interval suffix: %c (use s, m, h, or d)", suffix)
	}
}

// ── Refresh Tokens ────────────────────────────────────────────────────────────

func (p *Postgres) CreateRefreshToken(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO refresh_tokens(token_hash, user_id, expires_at) VALUES($1, $2, $3)`,
		tokenHash, userID, expiresAt)
	return err
}

func (p *Postgres) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := p.pool.QueryRow(ctx,
		`SELECT id, token_hash, user_id, expires_at, created_at FROM refresh_tokens WHERE token_hash=$1`,
		tokenHash).Scan(&rt.ID, &rt.TokenHash, &rt.UserID, &rt.ExpiresAt, &rt.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (p *Postgres) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, tokenHash)
	return err
}

func (p *Postgres) DeleteRefreshTokensByUser(ctx context.Context, userID int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id=$1`, userID)
	return err
}

func (p *Postgres) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error) {
	ct, err := p.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}
