package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// NotificationChannel defines a delivery target for alert notifications.
type NotificationChannel struct {
	ID      int64
	Type    string // webhook | email | slack
	Name    string
	Config  map[string]string // url, email, token, etc.
	Enabled bool
}

// Notifier dispatches notifications to configured channels.
type Notifier struct {
	channels []NotificationChannel
	client   *http.Client
}

func NewNotifier(channels []NotificationChannel) *Notifier {
	return &Notifier{
		channels: channels,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Send delivers an alert notification to all enabled channels.
func (n *Notifier) Send(ctx context.Context, alertID int64, deviceName, severity, message string) {
	for _, ch := range n.channels {
		if !ch.Enabled {
			continue
		}
		var err error
		switch ch.Type {
		case "webhook":
			err = n.sendWebhook(ctx, ch, alertID, deviceName, severity, message)
		default:
			slog.Warn("Notifier: unsupported channel type", "type", ch.Type)
		}
		if err != nil {
			slog.Warn("Notifier: delivery failed",
				"channel_id", ch.ID, "type", ch.Type, "error", err)
		} else {
			slog.Debug("Notifier: delivered",
				"channel_id", ch.ID, "type", ch.Type, "alert_id", alertID)
		}
	}
}

func (n *Notifier) sendWebhook(ctx context.Context, ch NotificationChannel, alertID int64, deviceName, severity, message string) error {
	url := ch.Config["url"]
	if url == "" {
		return fmt.Errorf("webhook channel %d has no url", ch.ID)
	}
	payload, _ := json.Marshal(map[string]any{
		"alert_id":    alertID,
		"device_name": deviceName,
		"severity":    severity,
		"message":     message,
		"timestamp":   time.Now().UTC(),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
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
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
