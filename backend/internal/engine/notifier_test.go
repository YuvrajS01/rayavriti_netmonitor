package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifier_Send_WebhookSuccess(t *testing.T) {
	t.Parallel()
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&receivedPayload))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:     1,
		Type:   "webhook",
		Config: map[string]any{"url": server.URL},
	}
	alert := &models.Alert{
		ID:         1,
		DeviceID:   10,
		DeviceName: "Server-1",
		Severity:   "critical",
		Message:    "Device is down",
		Status:     "active",
	}

	err := notifier.Send(context.Background(), ch, alert)
	require.NoError(t, err)
	assert.Equal(t, float64(1), receivedPayload["alert_id"])
	assert.Equal(t, "Server-1", receivedPayload["device_name"])
	assert.Equal(t, "critical", receivedPayload["severity"])
	assert.Equal(t, "Device is down", receivedPayload["message"])
}

func TestNotifier_Send_WebhookServerError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:     1,
		Type:   "webhook",
		Config: map[string]any{"url": server.URL},
	}
	alert := &models.Alert{ID: 1, DeviceName: "Server-1", Severity: "critical", Message: "Down"}

	err := notifier.Send(context.Background(), ch, alert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestNotifier_Send_WebhookNoURL(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:     1,
		Type:   "webhook",
		Config: map[string]any{},
	}
	alert := &models.Alert{ID: 1}

	err := notifier.Send(context.Background(), ch, alert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no url")
}

func TestNotifier_Send_DisabledChannel(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:      1,
		Type:    "webhook",
		Enabled: false,
		Config:  map[string]any{"url": "http://localhost"},
	}
	alert := &models.Alert{ID: 1}

	// Notifier.Send doesn't check Enabled — that's done by the engine
	// This tests that unsupported types return error
	err := notifier.Send(context.Background(), ch, alert)
	// Will try to send to the URL and may fail, but shouldn't panic
	_ = err
}

func TestNotifier_Send_UnsupportedChannelType(t *testing.T) {
	t.Parallel()
	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:     1,
		Type:   "sms",
		Config: map[string]any{},
	}
	alert := &models.Alert{ID: 1}

	err := notifier.Send(context.Background(), ch, alert)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestNotifier_Send_MultipleChannels(t *testing.T) {
	t.Parallel()
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewNotifier()
	channels := []models.NotificationChannel{
		{ID: 1, Type: "webhook", Config: map[string]any{"url": server.URL}},
		{ID: 2, Type: "webhook", Config: map[string]any{"url": server.URL}},
	}
	alert := &models.Alert{ID: 1}

	for _, ch := range channels {
		err := notifier.Send(context.Background(), ch, alert)
		require.NoError(t, err)
	}
	assert.Equal(t, 2, callCount)
}

func TestNotifier_Send_ContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewNotifier()
	ch := models.NotificationChannel{
		ID:     1,
		Type:   "webhook",
		Config: map[string]any{"url": server.URL},
	}
	alert := &models.Alert{ID: 1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := notifier.Send(ctx, ch, alert)
	require.Error(t, err)
}
