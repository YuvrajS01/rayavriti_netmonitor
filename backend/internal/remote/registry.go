package remote

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
	aead cipher.AEAD
}

func NewStore(pool *pgxpool.Pool, secret string) (*Store, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, aead: aead}, nil
}

func (s *Store) encrypt(value string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(value), nil), nil
}

func (s *Store) decrypt(value []byte) (string, error) {
	if len(value) < s.aead.NonceSize() {
		return "", fmt.Errorf("invalid encrypted remote credential")
	}
	plain, err := s.aead.Open(nil, value[:s.aead.NonceSize()], value[s.aead.NonceSize():], nil)
	return string(plain), err
}

func Validate(input CreateInstance) error {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.APIKey) == "" {
		return fmt.Errorf("name and apiKey are required")
	}
	u, err := url.ParseRequestURI(strings.TrimSpace(input.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("url must be an absolute http(s) URL")
	}
	if input.PollIntervalS != 0 && (input.PollIntervalS < 10 || input.PollIntervalS > 3600) {
		return fmt.Errorf("pollIntervalS must be between 10 and 3600")
	}
	return nil
}

func normalize(input *CreateInstance) {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimRight(strings.TrimSpace(input.URL), "/")
	if input.PollIntervalS == 0 {
		input.PollIntervalS = 60
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
}

func scanInstance(row pgx.Row) (*Instance, error) {
	var value Instance
	err := row.Scan(&value.ID, &value.Name, &value.URL, &value.LocationLabel, &value.Tags, &value.PollIntervalS, &value.TLSSkipVerify, &value.Status, &value.LastSeenAt, &value.LastError, &value.Fingerprint, &value.ServiceMode, &value.CreatedAt, &value.UpdatedAt)
	return &value, err
}

const instanceColumns = "id, name, url, location_label, tags, poll_interval_s, tls_skip_verify, status, last_seen_at, last_error, sync_fingerprint, service_mode, created_at, updated_at"

func (s *Store) List(ctx context.Context) ([]Instance, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+instanceColumns+" FROM remote_instances ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Instance{}
	for rows.Next() {
		item, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int64) (*Instance, error) {
	return scanInstance(s.pool.QueryRow(ctx, "SELECT "+instanceColumns+" FROM remote_instances WHERE id=$1", id))
}

func (s *Store) Create(ctx context.Context, input CreateInstance) (*Instance, error) {
	if err := Validate(input); err != nil {
		return nil, err
	}
	normalize(&input)
	credential, err := s.encrypt(input.APIKey)
	if err != nil {
		return nil, err
	}
	return scanInstance(s.pool.QueryRow(ctx, `INSERT INTO remote_instances (name,url,api_key_enc,location_label,tags,poll_interval_s,tls_skip_verify)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+instanceColumns, input.Name, input.URL, credential, input.LocationLabel, input.Tags, input.PollIntervalS, input.TLSSkipVerify))
}

func (s *Store) Update(ctx context.Context, id int64, input CreateInstance) (*Instance, error) {
	if input.APIKey == "" {
		existing, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		input.APIKey = "unchanged"
		input.URL = first(input.URL, existing.URL)
		input.Name = first(input.Name, existing.Name)
	}
	if input.PollIntervalS == 0 {
		input.PollIntervalS = 60
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	if input.APIKey == "unchanged" {
		return scanInstance(s.pool.QueryRow(ctx, `UPDATE remote_instances SET name=$2,url=$3,location_label=$4,tags=$5,poll_interval_s=$6,tls_skip_verify=$7,updated_at=NOW() WHERE id=$1 RETURNING `+instanceColumns, id, input.Name, strings.TrimRight(input.URL, "/"), input.LocationLabel, input.Tags, input.PollIntervalS, input.TLSSkipVerify))
	}
	if err := Validate(input); err != nil {
		return nil, err
	}
	normalize(&input)
	credential, err := s.encrypt(input.APIKey)
	if err != nil {
		return nil, err
	}
	return scanInstance(s.pool.QueryRow(ctx, `UPDATE remote_instances SET name=$2,url=$3,api_key_enc=$4,location_label=$5,tags=$6,poll_interval_s=$7,tls_skip_verify=$8,updated_at=NOW() WHERE id=$1 RETURNING `+instanceColumns, id, input.Name, input.URL, credential, input.LocationLabel, input.Tags, input.PollIntervalS, input.TLSSkipVerify))
}

func first(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
func (s *Store) Delete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM remote_instances WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
func (s *Store) credential(ctx context.Context, id int64) (string, error) {
	var value []byte
	err := s.pool.QueryRow(ctx, "SELECT api_key_enc FROM remote_instances WHERE id=$1", id).Scan(&value)
	if err != nil {
		return "", err
	}
	return s.decrypt(value)
}
func (s *Store) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO remote_snapshots (instance_id,device_count,device_up_count,device_down_count,alert_count,critical_alerts,health_score,latency_ms,version,raw_data) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, snapshot.InstanceID, snapshot.DeviceCount, snapshot.DeviceUpCount, snapshot.DeviceDownCount, snapshot.AlertCount, snapshot.CriticalAlerts, snapshot.HealthScore, snapshot.LatencyMS, snapshot.Version, snapshot.RawData)
	return err
}
func (s *Store) Snapshots(ctx context.Context, id int64, limit int) ([]Snapshot, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,instance_id,timestamp,device_count,device_up_count,device_down_count,alert_count,critical_alerts,health_score,latency_ms,version,raw_data FROM remote_snapshots WHERE instance_id=$1 ORDER BY timestamp DESC LIMIT $2`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snapshot{}
	for rows.Next() {
		var v Snapshot
		if err := rows.Scan(&v.ID, &v.InstanceID, &v.Timestamp, &v.DeviceCount, &v.DeviceUpCount, &v.DeviceDownCount, &v.AlertCount, &v.CriticalAlerts, &v.HealthScore, &v.LatencyMS, &v.Version, &v.RawData); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) SetStatus(ctx context.Context, id int64, status, errorMessage string, seen bool) error {
	if seen {
		_, err := s.pool.Exec(ctx, "UPDATE remote_instances SET status=$2,last_seen_at=NOW(),last_error=$3,updated_at=NOW() WHERE id=$1", id, status, errorMessage)
		return err
	}
	_, err := s.pool.Exec(ctx, "UPDATE remote_instances SET status=$2,last_error=$3,updated_at=NOW() WHERE id=$1", id, status, errorMessage)
	return err
}

func (s *Store) SetFingerprint(ctx context.Context, id int64, fingerprint string) error {
	_, err := s.pool.Exec(ctx, "UPDATE remote_instances SET sync_fingerprint=$2, updated_at=NOW() WHERE id=$1", id, fingerprint)
	return err
}

func (s *Store) SetServiceMode(ctx context.Context, id int64, mode string) error {
	_, err := s.pool.Exec(ctx, "UPDATE remote_instances SET service_mode=$2, updated_at=NOW() WHERE id=$1", id, mode)
	return err
}
func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var v Overview
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE status='online'),COUNT(*) FILTER (WHERE status='offline'),COUNT(*) FILTER (WHERE status='degraded'),COALESCE((SELECT SUM(alert_count) FROM (SELECT DISTINCT ON (instance_id) alert_count FROM remote_snapshots ORDER BY instance_id,timestamp DESC) latest),0) FROM remote_instances`).Scan(&v.TotalInstances, &v.Online, &v.Offline, &v.Degraded, &v.AlertCount)
	return v, err
}
func (s *Store) PruneSnapshots(ctx context.Context, days int) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM remote_snapshots WHERE timestamp < NOW() - ($1 * INTERVAL '1 day')", days)
	return err
}
func (s *Store) Due(ctx context.Context) ([]Instance, error) { return s.List(ctx) }
func (s *Store) Now() time.Time                              { return time.Now() }
