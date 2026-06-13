package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
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

	// Build MetricQuery for aggregation/bucketing support
	mq := models.MetricQuery{
		From:        from,
		To:          to,
		Status:      q.Get("status"),
		Aggregation: q.Get("aggregation"), // avg, max, min, p95
		BucketMin:   0,
	}
	if bucketStr := q.Get("bucketMin"); bucketStr != "" {
		if b, err := strconv.Atoi(bucketStr); err == nil {
			mq.BucketMin = b
		}
	}

	if deviceIDStr != "" {
		id, _ := strconv.ParseInt(deviceIDStr, 10, 64)
		mq.DeviceID = &id
	}

	if limit > 0 {
		mq.Limit = limit
	}

	// Use QueryMetrics if aggregation or bucketing is requested
	if mq.Aggregation != "" || mq.BucketMin > 0 {
		metrics, err := h.db.QueryMetrics(r.Context(), mq)
		if err != nil {
			httputil.SendError(w, 500, err.Error())
			return
		}
		httputil.SendOK(w, metrics)
		return
	}

	// Fallback to original behavior
	if mq.DeviceID != nil {
		metrics, err := h.db.GetDeviceMetrics(r.Context(), *mq.DeviceID, from, to, limit)
		if err != nil {
			httputil.SendError(w, 500, err.Error())
			return
		}
		httputil.SendOK(w, metrics)
		return
	}
	summary, err := h.db.GetMetricsSummary(r.Context(), from, to, nil)
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
