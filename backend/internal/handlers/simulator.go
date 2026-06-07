package handlers

import (
	"net/http"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type SimulatorHandler struct{ db database.Database }

func NewSimulatorHandler(db database.Database) *SimulatorHandler {
	return &SimulatorHandler{db: db}
}

func (h *SimulatorHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	var m models.Metric
	if err := httputil.ParseJSON(r, &m); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	if err := h.db.RecordMetric(r.Context(), &m); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, m)
}

func (h *SimulatorHandler) Flows(w http.ResponseWriter, r *http.Request) {
	var flows []models.Flow
	if err := httputil.ParseJSON(r, &flows); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	now := time.Now()
	for i := range flows {
		if flows[i].Timestamp.IsZero() {
			flows[i].Timestamp = now
		}
	}
	if err := h.db.RecordFlows(r.Context(), flows); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]int{"recorded": len(flows)})
}

func (h *SimulatorHandler) Alert(w http.ResponseWriter, r *http.Request) {
	var a models.Alert
	if err := httputil.ParseJSON(r, &a); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	a.Status = "active"
	created, err := h.db.CreateAlert(r.Context(), &a)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}
