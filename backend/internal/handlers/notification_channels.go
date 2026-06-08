package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

type NotificationChannelHandler struct{ db database.Database }

func NewNotificationChannelHandler(db database.Database) *NotificationChannelHandler {
	return &NotificationChannelHandler{db: db}
}

func (h *NotificationChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.db.GetNotificationChannels(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	httputil.SendOK(w, channels)
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
	httputil.SendOK(w, ch)
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
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *NotificationChannelHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if _, err := h.db.GetNotificationChannel(r.Context(), id); err != nil {
		httputil.SendError(w, 404, "notification channel not found")
		return
	}
	var ch models.NotificationChannel
	if err := httputil.ParseJSON(r, &ch); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	updated, err := h.db.UpdateNotificationChannel(r.Context(), id, &ch)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, updated)
}

func (h *NotificationChannelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteNotificationChannel(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}

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
	httputil.SendOK(w, map[string]any{
		"channelId":   ch.ID,
		"channelName": ch.Name,
		"channelType": ch.Type,
		"success":     true,
		"message":     "test notification sent (placeholder)",
	})
}
