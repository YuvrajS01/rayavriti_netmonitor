package handlers

import (
	"net/http"
	"time"

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
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, stats)
}
