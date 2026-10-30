package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// RetentionPolicy maps a hypertable or continuous aggregate to how many days
// of data it keeps. Days <= 0 removes any retention policy (keep forever).
type RetentionPolicy struct {
	Relation string
	Days     int
}

// ApplyRetentionPolicies reconciles TimescaleDB retention policies with
// config. It is idempotent: a policy is only replaced when its interval
// changed, and relations that don't exist are skipped. TimescaleDB drops
// whole chunks, which is far cheaper than row-level DELETEs.
func (p *Postgres) ApplyRetentionPolicies(ctx context.Context, policies []RetentionPolicy) error {
	for _, pol := range policies {
		if err := p.applyRetentionPolicy(ctx, pol); err != nil {
			return fmt.Errorf("retention policy for %s: %w", pol.Relation, err)
		}
	}
	return nil
}

func (p *Postgres) applyRetentionPolicy(ctx context.Context, pol RetentionPolicy) error {
	var exists bool
	if err := p.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM timescaledb_information.hypertables WHERE hypertable_name = $1)
		    OR EXISTS (SELECT 1 FROM timescaledb_information.continuous_aggregates WHERE view_name = $1)`,
		pol.Relation).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		slog.Debug("retention: relation is not a hypertable or continuous aggregate, skipping", "relation", pol.Relation)
		return nil
	}

	// Current policy, if any. For a continuous aggregate the job is attached
	// to its materialization hypertable, so match on the job config instead.
	var current *string
	if err := p.pool.QueryRow(ctx, `
		SELECT config->>'drop_after'
		FROM timescaledb_information.jobs
		WHERE proc_name = 'policy_retention'
		  AND (hypertable_name = $1 OR config->>'hypertable_id' = (
		        SELECT h.id::text FROM _timescaledb_catalog.hypertable h
		        JOIN timescaledb_information.continuous_aggregates ca
		          ON ca.materialization_hypertable_name = h.table_name
		        WHERE ca.view_name = $1))
		LIMIT 1`, pol.Relation).Scan(&current); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if pol.Days <= 0 {
		if current != nil {
			_, err := p.pool.Exec(ctx, `SELECT remove_retention_policy($1::regclass, if_exists => TRUE)`, pol.Relation)
			return err
		}
		return nil
	}

	want := fmt.Sprintf("%d days", pol.Days)
	if current != nil && *current == want {
		return nil
	}
	if current != nil {
		if _, err := p.pool.Exec(ctx, `SELECT remove_retention_policy($1::regclass, if_exists => TRUE)`, pol.Relation); err != nil {
			return err
		}
	}
	if _, err := p.pool.Exec(ctx, `SELECT add_retention_policy($1::regclass, drop_after => make_interval(days => $2))`, pol.Relation, pol.Days); err != nil {
		return err
	}
	slog.Info("retention policy set", "relation", pol.Relation, "days", pol.Days)
	return nil
}
