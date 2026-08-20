package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

var startTime = time.Now()

type HealthHandler struct{ db database.Database }

func NewHealthHandler(db database.Database) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := h.db.Ping(r.Context()); err != nil {
		dbStatus = "error: " + err.Error()
	}
	httputil.SendOK(w, map[string]any{
		"status":   "ok",
		"version":  "1.1.0",
		"uptime":   time.Since(startTime).Seconds(),
		"database": dbStatus,
	})
}

func (h *HealthHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetDashboardStats(r.Context())
	if err != nil {
		httputil.SendInternalError(w, err)
		return
	}
	httputil.SendOK(w, stats)
}

// Scores returns the latest persisted AI Health scores for all devices.
func (h *HealthHandler) Scores(w http.ResponseWriter, r *http.Request) {
	scores, err := h.db.GetHealthScores(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, scores)
}

// DeviceScore returns the latest persisted AI Health score for a single device.
func (h *HealthHandler) DeviceScore(w http.ResponseWriter, r *http.Request) {
	deviceID, err := strconv.ParseInt(chi.URLParam(r, "deviceId"), 10, 64)
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid device id")
		return
	}
	scores, err := h.db.GetHealthScores(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, s := range scores {
		if s.DeviceID == deviceID {
			httputil.SendOK(w, s)
			return
		}
	}
	httputil.SendError(w, http.StatusNotFound, "no health score for device")
}
