package handlers

import (
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type InsightHandler struct{ db database.Database }

func NewInsightHandler(db database.Database) *InsightHandler { return &InsightHandler{db: db} }

func (h *InsightHandler) Current(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	latest, _ := h.db.GetLatestMetrics(r.Context())
	metricMap := map[int64]float64{}
	for _, m := range latest {
		if m.ResponseTime != nil {
			metricMap[m.DeviceID] = *m.ResponseTime
		}
	}
	type Score struct {
		DeviceID   int64   `json:"deviceId"`
		DeviceName string  `json:"deviceName"`
		Score      float64 `json:"score"`
		Status     string  `json:"status"`
	}
	scores := make([]Score, 0, len(devices))
	for _, d := range devices {
		score := 100.0
		if d.Status == "down" {
			score = 0
		} else if rt, ok := metricMap[d.ID]; ok && rt > 1000 {
			score = 50
		}
		scores = append(scores, Score{DeviceID: d.ID, DeviceName: d.Name, Score: score, Status: d.Status})
	}
	httputil.SendOK(w, scores)
}

func (h *InsightHandler) History(w http.ResponseWriter, r *http.Request) {
	from, to, limit := parseTimeRange(r)
	metrics, err := h.db.GetDeviceMetrics(r.Context(), 0, from, to, limit)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, metrics)
}
