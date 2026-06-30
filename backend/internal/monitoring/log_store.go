package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
)

type LogEvent struct {
	ID               int64          `json:"id"`
	Timestamp        time.Time      `json:"timestamp"`
	Level            string         `json:"level"`
	Component        string         `json:"component"`
	EventType        string         `json:"eventType,omitempty"`
	Message          string         `json:"message"`
	RequestID        string         `json:"requestId,omitempty"`
	TraceID          string         `json:"traceId,omitempty"`
	UserID           string         `json:"userId,omitempty"`
	Actor            string         `json:"actor,omitempty"`
	RemoteAddr       string         `json:"remoteAddr,omitempty"`
	DeviceID         *int64         `json:"deviceId,omitempty"`
	SensorID         *int64         `json:"sensorId,omitempty"`
	Protocol         string         `json:"protocol,omitempty"`
	Method           string         `json:"method,omitempty"`
	Path             string         `json:"path,omitempty"`
	StatusCode       *int           `json:"statusCode,omitempty"`
	DurationMs       *float64       `json:"durationMs,omitempty"`
	Error            string         `json:"error,omitempty"`
	Hostname         string         `json:"hostname,omitempty"`
	PID              int            `json:"pid,omitempty"`
	Version          string         `json:"version,omitempty"`
	VerboseSessionID *int64         `json:"verboseSessionId,omitempty"`
	Attrs            map[string]any `json:"attrs,omitempty"`
}

type LogQuery struct {
	Level     string
	Component string
	EventType string
	From      *time.Time
	To        *time.Time
	DeviceID  *int64
	UserID    string
	RequestID string
	TraceID   string
	Search    string
	Limit     int
	Offset    int
}

type LogStats struct {
	Total        int            `json:"total"`
	ByLevel      map[string]int `json:"byLevel"`
	ByComponent  map[string]int `json:"byComponent"`
	Errors       int            `json:"errors"`
	SlowRequests int            `json:"slowRequests"`
	SlowQueries  int            `json:"slowQueries"`
}

type VerboseSession struct {
	ID         int64      `json:"id"`
	Level      string     `json:"level"`
	Components []string   `json:"components"`
	DeviceIDs  []int64    `json:"deviceIds"`
	UserIDs    []string   `json:"userIds"`
	Reason     string     `json:"reason"`
	StartedBy  *int64     `json:"startedBy,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) RecordLogEvent(ctx context.Context, e logging.PersistedEvent) error {
	if s == nil || s.pool == nil {
		return nil
	}
	attrs, _ := json.Marshal(e.Attrs)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO system_log_events (
			timestamp, level, component, event_type, message, request_id, trace_id, user_id,
			actor, remote_addr, device_id, sensor_id, protocol, method, path, status_code,
			duration_ms, error, hostname, pid, version, verbose_session_id, attrs
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23
		)`,
		e.Timestamp, e.Level, e.Component, e.EventType, e.Message, emptyToNil(e.RequestID), emptyToNil(e.TraceID), emptyToNil(e.UserID),
		emptyToNil(e.Actor), emptyToNil(e.RemoteAddr), e.DeviceID, e.SensorID, emptyToNil(e.Protocol), emptyToNil(e.Method), emptyToNil(e.Path),
		e.StatusCode, e.DurationMs, emptyToNil(e.Error), emptyToNil(e.Hostname), e.PID, emptyToNil(e.Version), e.VerboseSessionID, attrs)
	return err
}

func (s *Store) QueryLogs(ctx context.Context, q LogQuery) ([]LogEvent, int, error) {
	where, args := buildLogWhere(q)
	countSQL := `SELECT COUNT(*) FROM system_log_events` + where
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	sql := `SELECT id,timestamp,level,component,COALESCE(event_type,''),message,COALESCE(request_id,''),COALESCE(trace_id,''),
		COALESCE(user_id,''),COALESCE(actor,''),COALESCE(remote_addr,''),device_id,sensor_id,COALESCE(protocol,''),COALESCE(method,''),
		COALESCE(path,''),status_code,duration_ms,COALESCE(error,''),COALESCE(hostname,''),COALESCE(pid,0),COALESCE(version,''),
		verbose_session_id,attrs
		FROM system_log_events` + where + fmt.Sprintf(` ORDER BY timestamp DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	events, err := scanLogEvents(rows)
	return events, total, err
}

func (s *Store) LogStats(ctx context.Context, q LogQuery) (LogStats, error) {
	events, total, err := s.QueryLogs(ctx, LogQuery{
		Level: q.Level, Component: q.Component, EventType: q.EventType, From: q.From, To: q.To,
		DeviceID: q.DeviceID, UserID: q.UserID, RequestID: q.RequestID, TraceID: q.TraceID,
		Search: q.Search, Limit: 1000,
	})
	if err != nil {
		return LogStats{}, err
	}
	stats := LogStats{Total: total, ByLevel: map[string]int{}, ByComponent: map[string]int{}}
	for _, e := range events {
		stats.ByLevel[e.Level]++
		stats.ByComponent[e.Component]++
		if e.Level == "error" || e.Error != "" {
			stats.Errors++
		}
		if e.EventType == "request_end" && e.DurationMs != nil && *e.DurationMs >= 1000 {
			stats.SlowRequests++
		}
		if e.EventType == "slow_query" {
			stats.SlowQueries++
		}
	}
	return stats, nil
}

func (s *Store) CreateVerboseSession(ctx context.Context, level string, components []string, deviceIDs []int64, userIDs []string, reason string, startedBy *int64, expiresAt time.Time) (*VerboseSession, error) {
	var out VerboseSession
	err := s.pool.QueryRow(ctx, `
		INSERT INTO verbose_log_sessions(level, components, device_ids, user_ids, reason, started_by, expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, level, components, device_ids, user_ids, reason, started_by, created_at, expires_at, ended_at`,
		level, components, deviceIDs, userIDs, reason, startedBy, expiresAt).Scan(
		&out.ID, &out.Level, &out.Components, &out.DeviceIDs, &out.UserIDs, &out.Reason, &out.StartedBy, &out.CreatedAt, &out.ExpiresAt, &out.EndedAt)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ListVerboseSessions(ctx context.Context, activeOnly bool) ([]VerboseSession, error) {
	sql := `SELECT id, level, components, device_ids, user_ids, reason, started_by, created_at, expires_at, ended_at FROM verbose_log_sessions`
	if activeOnly {
		sql += ` WHERE ended_at IS NULL AND expires_at > NOW()`
	}
	sql += ` ORDER BY created_at DESC LIMIT 200`
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VerboseSession
	for rows.Next() {
		var s VerboseSession
		if err := rows.Scan(&s.ID, &s.Level, &s.Components, &s.DeviceIDs, &s.UserIDs, &s.Reason, &s.StartedBy, &s.CreatedAt, &s.ExpiresAt, &s.EndedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (s *Store) StopVerboseSession(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE verbose_log_sessions SET ended_at = NOW() WHERE id = $1 AND ended_at IS NULL`, id)
	return err
}

func (s *Store) LoadActiveVerboseSessions(ctx context.Context) ([]VerboseSession, error) {
	return s.ListVerboseSessions(ctx, true)
}

func buildLogWhere(q LogQuery) (string, []any) {
	var where []string
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if q.Level != "" {
		add("level = $%d", q.Level)
	}
	if q.Component != "" {
		args = append(args, q.Component)
		idx := len(args)
		where = append(where, fmt.Sprintf("(component = $%d OR component LIKE $%d || '.%%')", idx, idx))
	}
	if q.EventType != "" {
		add("event_type = $%d", q.EventType)
	}
	if q.From != nil {
		add("timestamp >= $%d", *q.From)
	}
	if q.To != nil {
		add("timestamp <= $%d", *q.To)
	}
	if q.DeviceID != nil {
		add("device_id = $%d", *q.DeviceID)
	}
	if q.UserID != "" {
		add("user_id = $%d", q.UserID)
	}
	if q.RequestID != "" {
		add("request_id = $%d", q.RequestID)
	}
	if q.TraceID != "" {
		add("trace_id = $%d", q.TraceID)
	}
	if q.Search != "" {
		args = append(args, q.Search)
		idx := len(args)
		where = append(where, fmt.Sprintf("(message ILIKE '%%' || $%d || '%%' OR error ILIKE '%%' || $%d || '%%' OR attrs::text ILIKE '%%' || $%d || '%%')", idx, idx, idx))
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func scanLogEvents(rows pgx.Rows) ([]LogEvent, error) {
	var out []LogEvent
	for rows.Next() {
		var e LogEvent
		var attrs []byte
		if err := rows.Scan(
			&e.ID, &e.Timestamp, &e.Level, &e.Component, &e.EventType, &e.Message, &e.RequestID, &e.TraceID,
			&e.UserID, &e.Actor, &e.RemoteAddr, &e.DeviceID, &e.SensorID, &e.Protocol, &e.Method, &e.Path,
			&e.StatusCode, &e.DurationMs, &e.Error, &e.Hostname, &e.PID, &e.Version, &e.VerboseSessionID, &attrs,
		); err != nil {
			return nil, err
		}
		if len(attrs) > 0 {
			_ = json.Unmarshal(attrs, &e.Attrs)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
