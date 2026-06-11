package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type ReportHandler struct{ db database.Database }

func NewReportHandler(db database.Database) *ReportHandler { return &ReportHandler{db: db} }

func parseDeviceID(r *http.Request) *int64 {
	s := r.URL.Query().Get("deviceId")
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func (h *ReportHandler) Summary(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	deviceID := parseDeviceID(r)
	summary, err := h.db.GetMetricsSummary(r.Context(), from, to, deviceID)
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
	from, to, _ := parseTimeRange(r)
	deviceID := parseDeviceID(r)
	bucketMinutes := 60
	if b := r.URL.Query().Get("bucket"); b != "" {
		fmt.Sscanf(b, "%d", &bucketMinutes)
	}
	points, err := h.db.GetReportTimeseries(r.Context(), from, to, bucketMinutes, deviceID)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, points)
}

func (h *ReportHandler) Devices(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	deviceID := parseDeviceID(r)
	breakdown, err := h.db.GetReportDeviceBreakdown(r.Context(), from, to, deviceID)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, breakdown)
}

func (h *ReportHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	deviceID := parseDeviceID(r)
	alerts, err := h.db.GetAlertsForReport(r.Context(), from, to, deviceID)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, alerts)
}

func (h *ReportHandler) Export(w http.ResponseWriter, r *http.Request) {
	from, to, limit := parseTimeRange(r)
	deviceID := parseDeviceID(r)
	if limit <= 0 {
		limit = 5000
	}
	metrics, err := h.db.ExportMetrics(r.Context(), from, to, deviceID, limit)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=metrics.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"id", "device_id", "device_name", "protocol", "timestamp", "status",
		"response_time_ms", "packet_loss_pct", "cpu_usage_pct", "memory_usage_pct",
		"bandwidth_mbps", "custom_value",
	})
	for _, m := range metrics {
		_ = cw.Write([]string{
			fmt.Sprintf("%d", m.ID),
			fmt.Sprintf("%d", m.DeviceID),
			m.DeviceName,
			m.Protocol,
			m.Timestamp.Format(time.RFC3339),
			m.Status,
			floatPtr(m.ResponseTime),
			floatPtr(m.PacketLoss),
			floatPtr(m.CPUUsage),
			floatPtr(m.MemoryUsage),
			floatPtr(m.Bandwidth),
			floatPtr(m.CustomValue),
		})
	}
	cw.Flush()
}

func floatPtr(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%g", *f)
}

func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	reports := []map[string]string{
		{"id": "availability", "name": "Availability Report"},
		{"id": "performance", "name": "Performance Report"},
		{"id": "sla", "name": "SLA Report"},
	}
	httputil.SendOK(w, reports)
}
