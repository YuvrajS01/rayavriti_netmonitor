package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type ReportHandler struct{ db database.Database }

func NewReportHandler(db database.Database) *ReportHandler { return &ReportHandler{db: db} }

func (h *ReportHandler) Summary(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	summary, err := h.db.GetMetricsSummary(r.Context(), from, to)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	stats, _ := h.db.GetDashboardStats(r.Context())
	for k, v := range stats {
		summary[k] = v
	}
	httputil.SendOK(w, summary)
}

func (h *ReportHandler) Timeseries(w http.ResponseWriter, r *http.Request) {
	from, to, limit := parseTimeRange(r)
	metrics, err := h.db.GetDeviceMetrics(r.Context(), 0, from, to, limit)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, metrics)
}

func (h *ReportHandler) Devices(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetDashboardStats(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, stats)
}

func (h *ReportHandler) Export(w http.ResponseWriter, r *http.Request) {
	from, to, limit := parseTimeRange(r)
	metrics, err := h.db.GetDeviceMetrics(r.Context(), 0, from, to, limit)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=metrics.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "device_id", "timestamp", "status", "response_time", "packet_loss"})
	for _, m := range metrics {
		rt := ""
		if m.ResponseTime != nil {
			rt = fmt.Sprintf("%g", *m.ResponseTime)
		}
		pl := ""
		if m.PacketLoss != nil {
			pl = fmt.Sprintf("%g", *m.PacketLoss)
		}
		_ = cw.Write([]string{
			fmt.Sprintf("%d", m.ID), fmt.Sprintf("%d", m.DeviceID),
			m.Timestamp.String(), m.Status, rt, pl,
		})
	}
	cw.Flush()
}
