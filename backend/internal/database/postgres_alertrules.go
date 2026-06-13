package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── Alert Rules ──────────────────────────────────────────────────────────────

func (p *Postgres) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, description, enabled, severity, scope_type, scope_value,
		       device_id, condition_logic, cooldown_seconds, auto_resolve,
		       created_by, created_at, updated_at
		FROM alert_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules, err := scanAlertRules(rows)
	if err != nil {
		return nil, err
	}

	for i := range rules {
		if err := p.loadAlertRuleRelations(ctx, &rules[i]); err != nil {
			return nil, err
		}
	}

	if rules == nil {
		rules = []models.AlertRule{}
	}
	return rules, nil
}

func (p *Postgres) GetAlertRule(ctx context.Context, id int64) (*models.AlertRule, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, description, enabled, severity, scope_type, scope_value,
		       device_id, condition_logic, cooldown_seconds, auto_resolve,
		       created_by, created_at, updated_at
		FROM alert_rules WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules, err := scanAlertRules(rows)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, pgx.ErrNoRows
	}

	if err := p.loadAlertRuleRelations(ctx, &rules[0]); err != nil {
		return nil, err
	}
	return &rules[0], nil
}

func (p *Postgres) CreateAlertRule(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1) Insert the rule
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO alert_rules(name, description, enabled, severity, scope_type, scope_value,
		                        device_id, condition_logic, cooldown_seconds, auto_resolve, created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		r.Name, nullStr(r.Description), r.Enabled, r.Severity, r.ScopeType,
		nullStr(r.ScopeValue), r.DeviceID, r.ConditionLogic, r.CooldownSec,
		r.AutoResolve, r.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert alert_rule: %w", err)
	}

	// 2) Insert conditions
	if err := insertConditions(ctx, tx, id, r.Conditions); err != nil {
		return nil, err
	}

	// 3) Link channels
	if err := insertChannelLinks(ctx, tx, id, r.ChannelIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return p.GetAlertRule(ctx, id)
}

func (p *Postgres) UpdateAlertRule(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1) Update the rule
	_, err = tx.Exec(ctx, `
		UPDATE alert_rules
		SET name=$1, description=$2, enabled=$3, severity=$4, scope_type=$5,
		    scope_value=$6, device_id=$7, condition_logic=$8, cooldown_seconds=$9,
		    auto_resolve=$10, created_by=$11, updated_at=NOW()
		WHERE id=$12`,
		r.Name, nullStr(r.Description), r.Enabled, r.Severity, r.ScopeType,
		nullStr(r.ScopeValue), r.DeviceID, r.ConditionLogic, r.CooldownSec,
		r.AutoResolve, r.CreatedBy, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update alert_rule: %w", err)
	}

	// 2) Replace conditions: delete old, insert new
	if _, err := tx.Exec(ctx, `DELETE FROM alert_rule_conditions WHERE rule_id=$1`, id); err != nil {
		return nil, fmt.Errorf("delete old conditions: %w", err)
	}
	if err := insertConditions(ctx, tx, id, r.Conditions); err != nil {
		return nil, err
	}

	// 3) Replace channel links: delete old, insert new
	if _, err := tx.Exec(ctx, `DELETE FROM alert_rule_channels WHERE rule_id=$1`, id); err != nil {
		return nil, fmt.Errorf("delete old channel links: %w", err)
	}
	if err := insertChannelLinks(ctx, tx, id, r.ChannelIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return p.GetAlertRule(ctx, id)
}

func (p *Postgres) DeleteAlertRule(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

func (p *Postgres) ToggleAlertRule(ctx context.Context, id int64, enabled bool) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE alert_rules SET enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	return err
}

// ── Notification Channels ────────────────────────────────────────────────────

func (p *Postgres) GetNotificationChannels(ctx context.Context) ([]models.NotificationChannel, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, type, config, enabled, created_at
		FROM notification_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels, err := scanNotificationChannels(rows)
	if err != nil {
		return nil, err
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	return channels, nil
}

func (p *Postgres) GetNotificationChannel(ctx context.Context, id int64) (*models.NotificationChannel, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, name, type, config, enabled, created_at
		FROM notification_channels WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels, err := scanNotificationChannels(rows)
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &channels[0], nil
}

func (p *Postgres) CreateNotificationChannel(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	cfgJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	var id int64
	err = p.pool.QueryRow(ctx, `
		INSERT INTO notification_channels(name, type, config, enabled)
		VALUES($1,$2,$3,$4) RETURNING id`,
		ch.Name, ch.Type, cfgJSON, ch.Enabled,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return p.GetNotificationChannel(ctx, id)
}

func (p *Postgres) UpdateNotificationChannel(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
	cfgJSON, err := json.Marshal(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
		UPDATE notification_channels
		SET name=$1, type=$2, config=$3, enabled=$4
		WHERE id=$5`,
		ch.Name, ch.Type, cfgJSON, ch.Enabled, id,
	)
	if err != nil {
		return nil, err
	}
	return p.GetNotificationChannel(ctx, id)
}

func (p *Postgres) DeleteNotificationChannel(ctx context.Context, id int64) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1`, id)
	return err
}

// ── Alert History ────────────────────────────────────────────────────────────

func (p *Postgres) RecordAlertHistory(ctx context.Context, h *models.AlertHistory) error {
	detailsJSON, _ := json.Marshal(h.Details)
	_, err := p.pool.Exec(ctx, `
		INSERT INTO alert_history(alert_id, rule_id, action, actor, details)
		VALUES($1,$2,$3,$4,$5)`,
		h.AlertID, h.RuleID, h.Action, nullStr(h.Actor), detailsJSON,
	)
	return err
}

func (p *Postgres) GetAlertHistory(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, alert_id, rule_id, action, actor, details, created_at
		FROM alert_history
		WHERE alert_id=$1
		ORDER BY created_at DESC`, alertID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.AlertHistory
	for rows.Next() {
		var h models.AlertHistory
		var detailsRaw []byte
		var actor *string
		err := rows.Scan(&h.ID, &h.AlertID, &h.RuleID, &h.Action, &actor, &detailsRaw, &h.CreatedAt)
		if err != nil {
			return nil, err
		}
		if actor != nil {
			h.Actor = *actor
		}
		if detailsRaw != nil {
			_ = json.Unmarshal(detailsRaw, &h.Details)
		}
		out = append(out, h)
	}
	if out == nil {
		out = []models.AlertHistory{}
	}
	return out, rows.Err()
}

// ── Alert Rule State ─────────────────────────────────────────────────────────

func (p *Postgres) GetAlertRuleState(ctx context.Context, ruleID, deviceID int64) (*models.AlertRuleState, error) {
	var s models.AlertRuleState
	var snapshotRaw []byte
	err := p.pool.QueryRow(ctx, `
		SELECT rule_id, device_id, state, first_met_at, last_evaluated_at,
		       last_fired_at, last_resolved_at, active_alert_id, condition_snapshot
		FROM alert_rule_state
		WHERE rule_id=$1 AND device_id=$2`, ruleID, deviceID).Scan(
		&s.RuleID, &s.DeviceID, &s.State, &s.FirstMetAt, &s.LastEvaluatedAt,
		&s.LastFiredAt, &s.LastResolvedAt, &s.ActiveAlertID, &snapshotRaw,
	)
	if err != nil {
		return nil, err
	}
	if snapshotRaw != nil {
		_ = json.Unmarshal(snapshotRaw, &s.ConditionSnapshot)
	}
	return &s, nil
}

func (p *Postgres) UpsertAlertRuleState(ctx context.Context, s *models.AlertRuleState) error {
	snapshotJSON, _ := json.Marshal(s.ConditionSnapshot)
	_, err := p.pool.Exec(ctx, `
		INSERT INTO alert_rule_state(rule_id, device_id, state, first_met_at,
		    last_evaluated_at, last_fired_at, last_resolved_at, active_alert_id, condition_snapshot)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (rule_id, device_id) DO UPDATE SET
		    state              = EXCLUDED.state,
		    first_met_at       = EXCLUDED.first_met_at,
		    last_evaluated_at  = EXCLUDED.last_evaluated_at,
		    last_fired_at      = EXCLUDED.last_fired_at,
		    last_resolved_at   = EXCLUDED.last_resolved_at,
		    active_alert_id    = EXCLUDED.active_alert_id,
		    condition_snapshot = EXCLUDED.condition_snapshot`,
		s.RuleID, s.DeviceID, s.State, s.FirstMetAt, s.LastEvaluatedAt,
		s.LastFiredAt, s.LastResolvedAt, s.ActiveAlertID, snapshotJSON,
	)
	return err
}

// ── internal helpers ─────────────────────────────────────────────────────────

// loadAlertRuleRelations populates Conditions and ChannelIDs on a rule.
func (p *Postgres) loadAlertRuleRelations(ctx context.Context, r *models.AlertRule) error {
	// conditions
	condRows, err := p.pool.Query(ctx, `
		SELECT id, rule_id, type, metric_field, operator, value, duration_seconds, config
		FROM alert_rule_conditions
		WHERE rule_id=$1 ORDER BY id`, r.ID)
	if err != nil {
		return fmt.Errorf("query conditions: %w", err)
	}
	defer condRows.Close()

	for condRows.Next() {
		var c models.AlertRuleCondition
		var cfgRaw []byte
		err := condRows.Scan(&c.ID, &c.RuleID, &c.Type, &c.MetricField,
			&c.Operator, &c.Value, &c.DurationSeconds, &cfgRaw)
		if err != nil {
			return err
		}
		if cfgRaw != nil {
			_ = json.Unmarshal(cfgRaw, &c.Config)
		}
		r.Conditions = append(r.Conditions, c)
	}
	if err := condRows.Err(); err != nil {
		return err
	}
	if r.Conditions == nil {
		r.Conditions = []models.AlertRuleCondition{}
	}

	// channel IDs
	chRows, err := p.pool.Query(ctx, `
		SELECT channel_id FROM alert_rule_channels
		WHERE rule_id=$1 ORDER BY channel_id`, r.ID)
	if err != nil {
		return fmt.Errorf("query channel links: %w", err)
	}
	defer chRows.Close()

	for chRows.Next() {
		var chID int64
		if err := chRows.Scan(&chID); err != nil {
			return err
		}
		r.ChannelIDs = append(r.ChannelIDs, chID)
	}
	if err := chRows.Err(); err != nil {
		return err
	}
	if r.ChannelIDs == nil {
		r.ChannelIDs = []int64{}
	}

	return nil
}

// insertConditions batch-inserts alert rule conditions within a transaction.
func insertConditions(ctx context.Context, tx pgx.Tx, ruleID int64, conditions []models.AlertRuleCondition) error {
	if len(conditions) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range conditions {
		cfgJSON, _ := json.Marshal(c.Config)
		batch.Queue(`
			INSERT INTO alert_rule_conditions(rule_id, type, metric_field, operator, value, duration_seconds, config)
			VALUES($1,$2,$3,$4,$5,$6,$7)`,
			ruleID, c.Type, c.MetricField, c.Operator, c.Value, c.DurationSeconds, cfgJSON,
		)
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("insert conditions: %w", err)
	}
	return nil
}

// insertChannelLinks batch-inserts alert_rule_channels rows within a transaction.
func insertChannelLinks(ctx context.Context, tx pgx.Tx, ruleID int64, channelIDs []int64) error {
	if len(channelIDs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, chID := range channelIDs {
		batch.Queue(`
			INSERT INTO alert_rule_channels(rule_id, channel_id) VALUES($1,$2)`,
			ruleID, chID,
		)
	}
	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("insert channel links: %w", err)
	}
	return nil
}

func scanAlertRules(rows pgx.Rows) ([]models.AlertRule, error) {
	var out []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var description, scopeValue *string
		err := rows.Scan(
			&r.ID, &r.Name, &description, &r.Enabled, &r.Severity, &r.ScopeType,
			&scopeValue, &r.DeviceID, &r.ConditionLogic, &r.CooldownSec,
			&r.AutoResolve, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if description != nil {
			r.Description = *description
		}
		if scopeValue != nil {
			r.ScopeValue = *scopeValue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanNotificationChannels(rows pgx.Rows) ([]models.NotificationChannel, error) {
	var out []models.NotificationChannel
	for rows.Next() {
		var ch models.NotificationChannel
		var cfgRaw []byte
		err := rows.Scan(&ch.ID, &ch.Name, &ch.Type, &cfgRaw, &ch.Enabled, &ch.CreatedAt)
		if err != nil {
			return nil, err
		}
		if cfgRaw != nil {
			_ = json.Unmarshal(cfgRaw, &ch.Config)
		}
		if ch.Config == nil {
			ch.Config = map[string]any{}
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}
