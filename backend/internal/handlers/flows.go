package handlers

import (
	"net/http"
	"strconv"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type FlowHandler struct{ db database.Database }

func NewFlowHandler(db database.Database) *FlowHandler { return &FlowHandler{db: db} }

func (h *FlowHandler) List(w http.ResponseWriter, r *http.Request) {
	from, to, limit := parseTimeRange(r)
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	flows, _, err := h.db.GetFlows(r.Context(), from, to, limit, offset)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, flows)
}

func (h *FlowHandler) TopTalkers(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	talkers, err := h.db.GetTopTalkers(r.Context(), from, to, n)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, talkers)
}

func (h *FlowHandler) Protocols(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	stats, err := h.db.GetProtocolStats(r.Context(), from, to)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, stats)
}

func (h *FlowHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}
	points, err := h.db.GetFlowTimeseries(r.Context(), from, to, interval)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, points)
}

func (h *FlowHandler) Stats(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	stats, err := h.db.GetFlowStats(r.Context(), from, to)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, stats)
}
