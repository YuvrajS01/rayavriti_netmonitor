package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── Capture Sessions ─────────────────────────────────────────────────────────

func (p *Postgres) CreateCaptureSession(ctx context.Context, cs *models.CaptureSession) (*models.CaptureSession, error) {
	protocols, err := json.Marshal(cs.Protocols)
	if err != nil {
		return nil, fmt.Errorf("marshal protocols: %w", err)
	}

	var id int64
	err = p.pool.QueryRow(ctx, `
		INSERT INTO capture_sessions(interface_name,filter,status,started_by,total_packets,total_bytes,protocols)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id`,
		cs.InterfaceName, cs.Filter, cs.Status, nullStr(cs.StartedBy),
		cs.TotalPackets, cs.TotalBytes, protocols,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert capture session: %w", err)
	}
	return p.GetCaptureSession(ctx, id)
}

func (p *Postgres) GetCaptureSession(ctx context.Context, id int64) (*models.CaptureSession, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,interface_name,filter,status,started_by,
		       total_packets,total_bytes,protocols,
		       started_at,stopped_at,error_message
		FROM capture_sessions WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions, err := scanCaptureSessions(rows)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &sessions[0], nil
}

func (p *Postgres) GetCaptureSessions(ctx context.Context) ([]models.CaptureSession, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,interface_name,filter,status,started_by,
		       total_packets,total_bytes,protocols,
		       started_at,stopped_at,error_message
		FROM capture_sessions ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions, err := scanCaptureSessions(rows)
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []models.CaptureSession{}
	}
	return sessions, nil
}

func (p *Postgres) StopCaptureSession(ctx context.Context, id int64, stats models.CaptureSessionStats) error {
	status := "stopped"
	if stats.ErrorMessage != "" {
		status = "error"
	}

	_, err := p.pool.Exec(ctx, `
		UPDATE capture_sessions
		SET status=$1, stopped_at=NOW(), total_packets=$2, total_bytes=$3, error_message=$4
		WHERE id=$5`,
		status, stats.TotalPackets, stats.TotalBytes, nullStr(stats.ErrorMessage), id)
	return err
}

func scanCaptureSessions(rows pgx.Rows) ([]models.CaptureSession, error) {
	var out []models.CaptureSession
	for rows.Next() {
		var cs models.CaptureSession
		var protocolsRaw []byte
		err := rows.Scan(
			&cs.ID, &cs.InterfaceName, &cs.Filter, &cs.Status, &cs.StartedBy,
			&cs.TotalPackets, &cs.TotalBytes, &protocolsRaw,
			&cs.StartedAt, &cs.StoppedAt, &cs.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		if protocolsRaw != nil {
			_ = json.Unmarshal(protocolsRaw, &cs.Protocols)
		}
		if cs.Protocols == nil {
			cs.Protocols = map[string]int64{}
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}
