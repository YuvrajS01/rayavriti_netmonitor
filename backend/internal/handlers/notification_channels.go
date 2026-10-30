package handlers

import (
	"errors"
	"github.com/rayavriti/netmonitor-backend/internal/netutil"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/engine"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type NotificationChannelHandler struct {
	db database.Database
}

func NewNotificationChannelHandler(db database.Database) *NotificationChannelHandler {
	return &NotificationChannelHandler{db: db}
}

func (h *NotificationChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.db.GetNotificationChannels(r.Context())
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	httputil.SendOK(w, maskChannels(channels))
}

func (h *NotificationChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	ch, err := h.db.GetNotificationChannel(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "notification channel not found")
		return
	}
	httputil.SendOK(w, maskChannel(ch))
}

func (h *NotificationChannelHandler) Create(w http.ResponseWriter, r *http.Request) {
	var ch models.NotificationChannel
	if err := httputil.ParseJSON(r, &ch); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if ch.Name == "" {
		httputil.SendError(w, 400, "name is required")
		return
	}
	if ch.Type == "" {
		httputil.SendError(w, 400, "type is required")
		return
	}
	if ch.Config == nil {
		ch.Config = map[string]any{}
	}
	ch.Enabled = true
	created, err := h.db.CreateNotificationChannel(r.Context(), &ch)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendCreated(w, maskChannel(created))
}

func (h *NotificationChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	existing, err := h.db.GetNotificationChannel(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "notification channel not found")
		return
	}
	var ch models.NotificationChannel
	if err := httputil.ParseJSON(r, &ch); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	restoreMaskedSecrets(ch.Config, existing.Config)
	updated, err := h.db.UpdateNotificationChannel(r.Context(), id, &ch)
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, maskChannel(updated))
}

func (h *NotificationChannelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteNotificationChannel(r.Context(), id); err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}

// Test sends an actual test notification through the channel.
func (h *NotificationChannelHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	ch, err := h.db.GetNotificationChannel(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "notification channel not found")
		return
	}

	testAlert := &models.Alert{
		ID:         0,
		DeviceID:   0,
		DeviceName: "Test Device",
		Severity:   "info",
		Message:    "This is a test notification from Rayavriti NetMonitor",
		Status:     "active",
		CreatedAt:  time.Now(),
	}

	notifier := engine.NewNotifier()
	start := time.Now()
	if err := notifier.Send(r.Context(), *ch, testAlert); err != nil {
		// Don't echo the raw error: SMTP banners and dial errors would turn
		// this endpoint into a probe of internal services.
		slog.Warn("test notification failed", "channel_id", ch.ID, "type", ch.Type, "error", err)
		msg := "test notification failed; see server logs for details"
		if errors.Is(err, netutil.ErrBlockedHost) {
			msg = "test notification failed: destination address is not allowed"
		}
		httputil.SendError(w, http.StatusBadGateway, msg)
		return
	}
	duration := time.Since(start)

	httputil.SendOK(w, map[string]any{
		"channelId":   ch.ID,
		"channelName": ch.Name,
		"channelType": ch.Type,
		"success":     true,
		"durationMs":  duration.Milliseconds(),
		"message":     "test notification sent successfully",
	})
}
