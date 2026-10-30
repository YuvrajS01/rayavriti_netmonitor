package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/netutil"
)

func TestNotifier_WebhookToLoopbackIsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	ch := models.NotificationChannel{ID: 1, Type: "webhook", Enabled: true, Config: map[string]any{"url": srv.URL}}
	err := NewNotifier().Send(context.Background(), ch, &models.Alert{Message: "x"})
	if !errors.Is(err, netutil.ErrBlockedHost) {
		t.Fatalf("expected the production notifier to refuse loopback, got %v", err)
	}
}

func TestHeaderSafe(t *testing.T) {
	if got := headerSafe("down\r\nBcc: attacker@example.com"); got != "down  Bcc: attacker@example.com" {
		t.Fatalf("headerSafe left a line break: %q", got)
	}
}
