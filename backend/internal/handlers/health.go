package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

var startTime = time.Now()

type HealthHandler struct {
	db      database.Database
	version string
}

func NewHealthHandler(db database.Database, version string) *HealthHandler {
	return &HealthHandler{db: db, version: version}
}

// Health is the readiness check: 503 when the database is unreachable, so
// container healthchecks and load balancers see a real failure. The driver
// error is logged, never returned (this endpoint is unauthenticated).
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	status, dbStatus, code := "ok", "ok", http.StatusOK
	if err := h.db.Ping(ctx); err != nil {
		slog.Warn("health check: database unreachable", "error", err)
		status, dbStatus, code = "degraded", "unavailable", http.StatusServiceUnavailable
	}
	httputil.SendStatus(w, code, map[string]any{
		"status":   status,
		"version":  h.version,
		"uptime":   time.Since(startTime).Seconds(),
		"database": dbStatus,
	})
}

// Live is the liveness check: the process is up and serving HTTP.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	httputil.SendOK(w, map[string]any{"status": "ok", "version": h.version})
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
		httputil.SendInternalError(w, err)
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
		httputil.SendInternalError(w, err)
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
