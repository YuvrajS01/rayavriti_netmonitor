package monitoring

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SysConfigStore interface {
	GetSysConfig(context.Context, string) (string, error)
	SetSysConfig(context.Context, string, string) error
}

type PostgresSysConfigStore struct{ pool *pgxpool.Pool }

func NewSysConfigStore(pool *pgxpool.Pool) *PostgresSysConfigStore {
	return &PostgresSysConfigStore{pool: pool}
}
func (s *PostgresSysConfigStore) GetSysConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx, "SELECT value FROM sys_config WHERE key=$1", key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return value, err
}
func (s *PostgresSysConfigStore) SetSysConfig(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO sys_config(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()", key, value)
	return err
}
