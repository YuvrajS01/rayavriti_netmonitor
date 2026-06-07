package database

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── Sensors ──────────────────────────────────────────────────────────────────

// GetSensors returns all sensors. If deviceID is non-nil, results are filtered
// to only sensors belonging to that device.
func (p *Postgres) GetSensors(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
	var rows pgx.Rows
	var err error

	if deviceID != nil {
		rows, err = p.pool.Query(ctx, `
			SELECT id,device_id,name,type,enabled,interval,config,created_at,updated_at
			FROM sensors WHERE device_id=$1 ORDER BY id`, *deviceID)
	} else {
		rows, err = p.pool.Query(ctx, `
			SELECT id,device_id,name,type,enabled,interval,config,created_at,updated_at
			FROM sensors ORDER BY id`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSensors(rows)
}

// GetSensor returns a single sensor by ID.
func (p *Postgres) GetSensor(ctx context.Context, id int64) (*models.Sensor, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,device_id,name,type,enabled,interval,config,created_at,updated_at
		FROM sensors WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sensors, err := scanSensors(rows)
	if err != nil {
		return nil, err
	}
	if len(sensors) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &sensors[0], nil
}

// CreateSensor inserts a new sensor and returns the fully-populated row.
func (p *Postgres) CreateSensor(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
	cfg, _ := json.Marshal(s.Config)
	if s.Config == nil {
		cfg = []byte("{}")
	}
	var id int64
	err := p.pool.QueryRow(ctx, `
		INSERT INTO sensors(device_id,name,type,enabled,interval,config)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id`,
		s.DeviceID, s.Name, s.Type, s.Enabled, s.Interval, cfg,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return p.GetSensor(ctx, id)
}

// UpdateSensor updates an existing sensor and returns the refreshed row.
func (p *Postgres) UpdateSensor(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
	cfg, _ := json.Marshal(s.Config)
	if s.Config == nil {
		cfg = []byte("{}")
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE sensors SET device_id=$1,name=$2,type=$3,enabled=$4,interval=$5,
		    config=$6,updated_at=NOW()
		WHERE id=$7`,
		s.DeviceID, s.Name, s.Type, s.Enabled, s.Interval, cfg, id)
	if err != nil {
		return nil, err
	}
	return p.GetSensor(ctx, id)
}

// DeleteSensor removes a sensor by ID.
func (p *Postgres) DeleteSensor(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM sensors WHERE id=$1`, id)
	return err
}

// GetSensorsByDeviceID returns all sensors for a given device.
func (p *Postgres) GetSensorsByDeviceID(ctx context.Context, deviceID int64) ([]models.Sensor, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id,device_id,name,type,enabled,interval,config,created_at,updated_at
		FROM sensors WHERE device_id=$1 ORDER BY id`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSensors(rows)
}

// scanSensors scans pgx.Rows into a slice of Sensor models, unmarshalling
// the JSONB config column.
func scanSensors(rows pgx.Rows) ([]models.Sensor, error) {
	var out []models.Sensor
	for rows.Next() {
		var s models.Sensor
		var cfgRaw []byte
		err := rows.Scan(
			&s.ID, &s.DeviceID, &s.Name, &s.Type, &s.Enabled,
			&s.Interval, &cfgRaw, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if cfgRaw != nil {
			_ = json.Unmarshal(cfgRaw, &s.Config)
		}
		if s.Config == nil {
			s.Config = map[string]any{}
		}
		out = append(out, s)
	}
	if out == nil {
		out = []models.Sensor{}
	}
	return out, rows.Err()
}
