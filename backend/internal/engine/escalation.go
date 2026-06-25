package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// EscalationEngine manages multi-step escalation for alerts.
type EscalationEngine struct {
	pool     *pgxpool.Pool
	resolver *ContactResolver
	notifier *Notifier
	config   *EscalationConfig

	mu      sync.Mutex
	running map[int64]*escalationRun
}

// EscalationConfig holds escalation settings from environment.
type EscalationConfig struct {
	Enabled         bool
	BotToken        string
	DefaultChatID   string
	MaxSteps        int
	DefaultDelayMin int
}

// escalationRun tracks an active escalation for a single alert.
type escalationRun struct {
	alertID   int64
	step      int
	cancelled bool
}

// NewEscalationEngine creates a new EscalationEngine.
func NewEscalationEngine(pool *pgxpool.Pool, resolver *ContactResolver, notifier *Notifier, config *EscalationConfig) *EscalationEngine {
	if config == nil {
		config = &EscalationConfig{}
	}
	return &EscalationEngine{
		pool:     pool,
		resolver: resolver,
		notifier: notifier,
		config:   config,
		running:  make(map[int64]*escalationRun),
	}
}

// StartEscalation begins multi-step escalation for a newly fired alert.
func (e *EscalationEngine) StartEscalation(ctx context.Context, alert *models.Alert, policyID int64) error {
	if !e.config.Enabled {
		return nil
	}

	steps, err := e.getSteps(ctx, policyID)
	if err != nil || len(steps) == 0 {
		return err
	}

	resolved, err := e.resolver.ResolveForAlert(ctx, alert, alert.Severity)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.running[alert.ID] = &escalationRun{alertID: alert.ID, step: 0}
	e.mu.Unlock()

	if len(resolved) > 0 {
		if sendErr := e.notifyStep(ctx, resolved[0], alert, 0); sendErr != nil {
			slog.Error("Escalation: initial notify failed", "alert_id", alert.ID, "err", sendErr)
		}
	}

	if len(steps) > 1 {
		go e.runSteps(alert, steps) //nolint:gosec // Background escalation; request context not available
	}

	return nil
}

// CancelEscalation stops escalation for a given alert.
func (e *EscalationEngine) CancelEscalation(alertID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if run, ok := e.running[alertID]; ok {
		run.cancelled = true
		delete(e.running, alertID)
	}
}

func (e *EscalationEngine) runSteps(alert *models.Alert, steps []models.EscalationStep) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	for i := 1; i < len(steps); i++ {
		step := steps[i]
		delay := time.Duration(step.DelayMinutes) * time.Minute
		if delay < time.Minute {
			delay = time.Minute
		}

		time.Sleep(delay)

		e.mu.Lock()
		run, ok := e.running[alert.ID]
		cancelled := ok && run.cancelled
		e.mu.Unlock()

		if cancelled {
			return
		}

		if _, err := e.pool.Exec(ctx,
			`UPDATE alerts SET status = 'notified' WHERE id = $1 AND status = 'firing'`,
			alert.ID,
		); err != nil {
			return
		}

		contacts, err := e.resolver.ResolveForAlert(ctx, alert, alert.Severity)
		if err != nil {
			slog.Error("Escalation: resolve contacts failed", "step", i, "err", err)
			continue
		}

		if len(contacts) > 0 {
			idx := i % len(contacts)
			if sendErr := e.notifyStep(ctx, contacts[idx], alert, i); sendErr != nil {
				slog.Error("Escalation: notify failed", "step", i, "alert_id", alert.ID, "err", sendErr)
			}
		}

		e.mu.Lock()
		if run, ok := e.running[alert.ID]; ok {
			run.step = i
		}
		e.mu.Unlock()
	}
}

func (e *EscalationEngine) getSteps(ctx context.Context, policyID int64) ([]models.EscalationStep, error) {
	rows, err := e.pool.Query(ctx,
		`SELECT id, policy_id, step_order, contact_id, delay_minutes,
			notify_via, repeat_count, COALESCE(repeat_interval_minutes,0)
		FROM escalation_steps
		WHERE policy_id = $1
		ORDER BY step_order`,
		policyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []models.EscalationStep
	for rows.Next() {
		var s models.EscalationStep
		if err := rows.Scan(&s.ID, &s.PolicyID, &s.StepOrder, &s.ContactID,
			&s.DelayMinutes, &s.NotifyVia, &s.RepeatCount, &s.RepeatIntervalMinutes); err != nil {
			return nil, err
		}
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

func (e *EscalationEngine) notifyStep(ctx context.Context, rc ResolvedContact, alert *models.Alert, step int) error {
	slog.Info("Sending escalation notification",
		"alert_id", alert.ID,
		"step", step,
		"contact", rc.Contact.Name,
		"channel", rc.Channel,
	)

	logID, logErr := e.logNotification(ctx, alert, rc, step)
	if logErr != nil {
		slog.Error("Failed to log notification", "err", logErr)
	}

	var err error
	switch rc.Channel {
	case "telegram":
		keyboard := [][]map[string]string{
			{
				{"text": "Acknowledge", "callback_data": fmt.Sprintf("/ack %d", alert.ID)},
			},
		}
		err = e.notifier.SendToChatID(ctx, e.config.BotToken, rc.Target, alert, keyboard)
	case "email":
		ch := models.NotificationChannel{Type: "email", Config: map[string]any{"to": rc.Target}}
		err = e.notifier.Send(ctx, ch, alert)
	case "slack":
		ch := models.NotificationChannel{Type: "slack", Config: map[string]any{"webhook_url": rc.Target}}
		err = e.notifier.Send(ctx, ch, alert)
	default:
		err = fmt.Errorf("unsupported channel: %s", rc.Channel)
	}

	if logID > 0 {
		status := "sent"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
		}
		if _, updateErr := e.pool.Exec(ctx,
			`UPDATE notification_log SET status = $1, error_message = $2, sent_at = NOW()
			WHERE id = $3`,
			status, errMsg, logID,
		); updateErr != nil {
			slog.Error("Failed to update notification log", "err", updateErr)
		}
	}

	return err
}

func (e *EscalationEngine) logNotification(ctx context.Context, alert *models.Alert, rc ResolvedContact, step int) (int64, error) {
	var id int64
	err := e.pool.QueryRow(ctx,
		`INSERT INTO notification_log (alert_id, contact_id, channel_type, recipient, message_preview, status, escalation_step, sent_at)
		VALUES ($1, $2, $3, $4, $5, 'sending', $6, NOW())
		RETURNING id`,
		alert.ID, rc.Contact.ID, rc.Channel, rc.Target, truncate(alert.Message, 500), step,
	).Scan(&id)
	return id, err
}

// GetActiveStep returns the current escalation step for an alert, or -1 if none.
func (e *EscalationEngine) GetActiveStep(alertID int64) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if run, ok := e.running[alertID]; ok {
		return run.step
	}
	return -1
}

// RunCount returns the number of active escalations.
func (e *EscalationEngine) RunCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.running)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
