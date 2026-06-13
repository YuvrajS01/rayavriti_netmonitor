package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── Port Scans ───────────────────────────────────────────────────────────────

func (p *Postgres) UpsertPortScanResults(ctx context.Context, deviceID int64, results []models.PortScanResult) error {
	if len(results) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, r := range results {
		batch.Queue(`
			INSERT INTO port_scan_results(device_id,port,protocol,state,service,response_time)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(device_id,port,protocol) DO UPDATE SET
				state=EXCLUDED.state,
				service=EXCLUDED.service,
				response_time=EXCLUDED.response_time,
				last_seen=NOW(),
				scanned_at=NOW(),
				last_changed_at=CASE
					WHEN port_scan_results.state != EXCLUDED.state THEN NOW()
					ELSE port_scan_results.last_changed_at
				END`,
			deviceID, r.Port, r.Protocol, r.State, nullStr(r.Service), r.ResponseTime,
		)
	}

	br := p.pool.SendBatch(ctx, batch)
	return br.Close()
}

func (p *Postgres) GetPortScanResults(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,device_id,port,protocol,state,service,response_time,
		       first_seen,last_seen,last_changed_at,scanned_at
		FROM port_scan_results
		WHERE device_id=$1
		ORDER BY (state = 'open') DESC, port ASC`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := scanPortScanResults(rows)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []models.PortScanResult{}
	}
	return results, nil
}

func scanPortScanResults(rows pgx.Rows) ([]models.PortScanResult, error) {
	var out []models.PortScanResult
	for rows.Next() {
		var r models.PortScanResult
		err := rows.Scan(
			&r.ID, &r.DeviceID, &r.Port, &r.Protocol, &r.State, &r.Service,
			&r.ResponseTime, &r.FirstSeen, &r.LastSeen, &r.LastChangedAt, &r.ScannedAt,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
