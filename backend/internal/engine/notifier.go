package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// Notifier dispatches notifications to configured channels.
type Notifier struct {
	client *http.Client
}

// NewNotifier creates a notification dispatcher.
func NewNotifier() *Notifier {
	return &Notifier{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers an alert notification to the given channel.
func (n *Notifier) Send(ctx context.Context, ch models.NotificationChannel, alert *models.Alert) error {
	switch ch.Type {
	case "webhook":
		return n.sendWebhook(ctx, ch, alert)
	case "email":
		return n.sendEmail(ctx, ch, alert)
	case "slack":
		return n.sendSlack(ctx, ch, alert)
	default:
		slog.Warn("Notifier: unsupported channel type", "type", ch.Type, "channel_id", ch.ID)
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

func (n *Notifier) sendWebhook(ctx context.Context, ch models.NotificationChannel, alert *models.Alert) error {
	url, _ := ch.Config["url"].(string)
	if url == "" {
		return fmt.Errorf("webhook channel %d has no url", ch.ID)
	}

	payload, _ := json.Marshal(map[string]any{
		"alert_id":    alert.ID,
		"device_id":   alert.DeviceID,
		"device_name": alert.DeviceName,
		"severity":    alert.Severity,
		"message":     alert.Message,
		"status":      alert.Status,
		"rule_id":     alert.RuleID,
		"timestamp":   time.Now().UTC(),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if headers, ok := ch.Config["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendEmail(ctx context.Context, ch models.NotificationChannel, alert *models.Alert) error {
	host, _ := ch.Config["smtp_host"].(string)
	port, _ := ch.Config["smtp_port"].(string)
	from, _ := ch.Config["from"].(string)
	to, _ := ch.Config["to"].(string)
	username, _ := ch.Config["username"].(string)
	password, _ := ch.Config["password"].(string)

	if host == "" || from == "" || to == "" {
		return fmt.Errorf("email channel %d missing required config (smtp_host, from, to)", ch.ID)
	}

	subject := fmt.Sprintf("[NetMonitor %s] %s — %s", alert.Severity, alert.Message, alert.DeviceName)
	body := fmt.Sprintf(
		"Device:     %s\n"+
			"Severity:   %s\n"+
			"Message:    %s\n"+
			"Status:     %s\n"+
			"Alert ID:   %d\n"+
			"Time:       %s\n",
		alert.DeviceName, alert.Severity, alert.Message, alert.Status,
		alert.ID, time.Now().UTC().Format(time.RFC3339),
	)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%s", host, port)
	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func (n *Notifier) sendSlack(ctx context.Context, ch models.NotificationChannel, alert *models.Alert) error {
	webhookURL, _ := ch.Config["webhook_url"].(string)
	if webhookURL == "" {
		return fmt.Errorf("slack channel %d has no webhook_url", ch.ID)
	}

	severityEmoji := "⚠️"
	if alert.Severity == "critical" {
		severityEmoji = "🔴"
	} else if alert.Severity == "info" {
		severityEmoji = "ℹ️"
	}

	text := fmt.Sprintf("%s *[%s]* %s\nDevice: %s\nTime: %s",
		severityEmoji, alert.Severity, alert.Message,
		alert.DeviceName, time.Now().UTC().Format(time.RFC3339))

	payload, _ := json.Marshal(map[string]any{
		"text": text,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}
