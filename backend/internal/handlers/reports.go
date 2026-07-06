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
		fmt.Sscanf(b, "%d", &bucketMinutes) //nolint:errcheck,gosec
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

func (h *ReportHandler) ISP(w http.ResponseWriter, r *http.Request) {
	from, to, _ := parseTimeRange(r)
	pp, ok := h.db.(database.PoolProvider)
	if !ok || pp.Pool() == nil {
		httputil.SendOK(w, []any{})
		return
	}
	rows, err := pp.Pool().Query(r.Context(),
		`SELECT l.id, l.name, l.provider, l.bandwidth_mbps, l.sla_uptime_percent,
		 COALESCE(AVG(m.latency_ms),0) as avg_latency,
		 COALESCE(AVG(m.jitter_ms),0) as avg_jitter,
		 COALESCE(AVG(m.packet_loss_percent),0) as avg_packet_loss,
		 COALESCE(AVG(m.download_speed_mbps),0) as avg_download,
		 COALESCE(AVG(m.upload_speed_mbps),0) as avg_upload,
		 COUNT(m.id) as total_probes,
		 COUNT(m.id) FILTER (WHERE m.status = 'up') as up_probes
		 FROM isp_links l
		 LEFT JOIN isp_metrics m ON m.link_id = l.id AND m.created_at BETWEEN $1 AND $2
		 WHERE l.enabled = true
		 GROUP BY l.id, l.name, l.provider, l.bandwidth_mbps, l.sla_uptime_percent
		 ORDER BY l.name`, from, to)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type ispReport struct {
		ID            int64   `json:"id"`
		Name          string  `json:"name"`
		Provider      string  `json:"provider"`
		BandwidthMbps int     `json:"bandwidthMbps"`
		SLATarget     float64 `json:"slaTarget"`
		AvgLatency    float64 `json:"avgLatency"`
		AvgJitter     float64 `json:"avgJitter"`
		AvgPacketLoss float64 `json:"avgPacketLoss"`
		AvgDownload   float64 `json:"avgDownload"`
		AvgUpload     float64 `json:"avgUpload"`
		TotalProbes   int64   `json:"totalProbes"`
		UpProbes      int64   `json:"-"`
		UptimePercent float64 `json:"uptimePercent"`
	}

	var links []ispReport
	for rows.Next() {
		var l ispReport
		if err := rows.Scan(&l.ID, &l.Name, &l.Provider, &l.BandwidthMbps, &l.SLATarget,
			&l.AvgLatency, &l.AvgJitter, &l.AvgPacketLoss, &l.AvgDownload, &l.AvgUpload,
			&l.TotalProbes, &l.UpProbes); err != nil {
			continue
		}
		if l.TotalProbes > 0 {
			l.UptimePercent = float64(l.UpProbes) / float64(l.TotalProbes) * 100
		}
		links = append(links, l)
	}
	if links == nil {
		links = []ispReport{}
	}
	httputil.SendOK(w, links)
}
