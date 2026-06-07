package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type MetricHandler struct{ db database.Database }

func NewMetricHandler(db database.Database) *MetricHandler { return &MetricHandler{db: db} }

func (h *MetricHandler) Latest(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.db.GetLatestMetrics(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, metrics)
}

func (h *MetricHandler) ForDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "deviceId"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	from, to, limit := parseTimeRange(r)
	metrics, err := h.db.GetDeviceMetrics(r.Context(), id, from, to, limit)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, metrics)
}

func (h *MetricHandler) Query(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	deviceIDStr := q.Get("deviceId")
	from, to, limit := parseTimeRange(r)
	if deviceIDStr != "" {
		id, _ := strconv.ParseInt(deviceIDStr, 10, 64)
		metrics, err := h.db.GetDeviceMetrics(r.Context(), id, from, to, limit)
		if err != nil {
			httputil.SendError(w, 500, err.Error())
			return
		}
		httputil.SendOK(w, metrics)
		return
	}
	summary, err := h.db.GetMetricsSummary(r.Context(), from, to)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, summary)
}

func parseTimeRange(r *http.Request) (from, to time.Time, limit int) {
	q := r.URL.Query()
	to = time.Now()
	from = to.Add(-24 * time.Hour)
	if s := q.Get("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = t
		}
	}
	if s := q.Get("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = t
		}
	}
	limit, _ = strconv.Atoi(q.Get("limit"))
	return
}
