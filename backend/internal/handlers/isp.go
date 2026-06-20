package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type ISPHandler struct {
	pool *pgxpool.Pool
}

func NewISPHandler(db database.Database) *ISPHandler {
	pg, ok := db.(*database.Postgres)
	if !ok {
		slog.Warn("ISPHandler: database is not *Postgres, ISP features will be unavailable")
		return &ISPHandler{}
	}
	return &ISPHandler{pool: pg.Pool()}
}

func (h *ISPHandler) Comparison(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "ISP service unavailable")
		return
	}

	links, err := h.getEnabledLinks(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	comparisons := []map[string]any{}
	for _, link := range links {
		linkID := link["id"].(int64)
		stats := h.getLinkStats(r.Context(), linkID)

		comparisons = append(comparisons, map[string]any{
			"id":            linkID,
			"name":          link["name"],
			"provider":      link["provider"],
			"bandwidthMbps": link["bandwidth_mbps"],
			"gatewayIp":     link["gateway_ip"],
			"slaUptime":     link["sla_uptime_percent"],
			"avgLatencyMs":  stats["avg_latency"],
			"avgJitterMs":   stats["avg_jitter"],
			"avgPacketLoss": stats["avg_packet_loss"],
			"avgDownload":   stats["avg_download"],
			"avgUpload":     stats["avg_upload"],
			"uptimePercent": stats["uptime_percent"],
			"totalProbes":   stats["total_probes"],
			"status":        stats["latest_status"],
		})
	}

	httputil.SendOK(w, map[string]any{
		"links":      comparisons,
		"comparedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *ISPHandler) LinkSLA(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "ISP service unavailable")
		return
	}
	linkID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid link id")
		return
	}

	var name, provider string
	var slaUptime *float64
	var bandwidth *int
	err = h.pool.QueryRow(r.Context(),
		`SELECT name, provider, sla_uptime_percent, bandwidth_mbps FROM isp_links WHERE id=$1`, linkID,
	).Scan(&name, &provider, &slaUptime, &bandwidth)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "ISP link not found")
		return
	}

	stats := h.getLinkStats(r.Context(), linkID)

	actualUptime := stats["uptime_percent"].(float64)
	slaCompliant := true
	var slaGap *float64
	if slaUptime != nil {
		gap := actualUptime - *slaUptime
		slaGap = &gap
		if actualUptime < *slaUptime {
			slaCompliant = false
		}
	}

	httputil.SendOK(w, map[string]any{
		"linkId":        linkID,
		"name":          name,
		"provider":      provider,
		"slaTarget":     slaUptime,
		"actualUptime":  actualUptime,
		"slaCompliant":  slaCompliant,
		"slaGap":        slaGap,
		"avgLatencyMs":  stats["avg_latency"],
		"avgJitterMs":   stats["avg_jitter"],
		"avgPacketLoss": stats["avg_packet_loss"],
		"totalProbes":   stats["total_probes"],
		"latestStatus":  stats["latest_status"],
	})
}

func (h *ISPHandler) MetricsSummary(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		httputil.SendError(w, http.StatusNotImplemented, "ISP service unavailable")
		return
	}
	linkID, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid link id")
		return
	}

	stats := h.getLinkStats(r.Context(), linkID)
	httputil.SendOK(w, stats)
}

func (h *ISPHandler) getEnabledLinks(ctx context.Context) ([]map[string]any, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, provider, circuit_id, bandwidth_mbps, gateway_ip, sla_uptime_percent, cost_monthly, enabled
		 FROM isp_links WHERE enabled=true ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	out := []map[string]any{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(map[string]any, len(fields))
		for i, fd := range fields {
			item[fd.Name] = vals[i]
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (h *ISPHandler) getLinkStats(ctx context.Context, linkID int64) map[string]any {
	stats := map[string]any{}

	var avgLatency, avgJitter, avgPacketLoss, avgDownload, avgUpload *float64
	var totalProbes int
	var latestStatus string

	_ = h.pool.QueryRow(ctx,
		`SELECT AVG(latency_ms), AVG(jitter_ms), AVG(packet_loss_percent),
		 AVG(download_speed_mbps), AVG(upload_speed_mbps), COUNT(*)
		 FROM isp_metrics WHERE link_id=$1`, linkID,
	).Scan(&avgLatency, &avgJitter, &avgPacketLoss, &avgDownload, &avgUpload, &totalProbes)

	_ = h.pool.QueryRow(ctx,
		`SELECT status FROM isp_metrics WHERE link_id=$1 ORDER BY created_at DESC LIMIT 1`, linkID,
	).Scan(&latestStatus)

	if avgLatency != nil {
		stats["avg_latency"] = *avgLatency
	} else {
		stats["avg_latency"] = 0
	}
	if avgJitter != nil {
		stats["avg_jitter"] = *avgJitter
	} else {
		stats["avg_jitter"] = 0
	}
	if avgPacketLoss != nil {
		stats["avg_packet_loss"] = *avgPacketLoss
	} else {
		stats["avg_packet_loss"] = 0
	}
	if avgDownload != nil {
		stats["avg_download"] = *avgDownload
	} else {
		stats["avg_download"] = 0
	}
	if avgUpload != nil {
		stats["avg_upload"] = *avgUpload
	} else {
		stats["avg_upload"] = 0
	}

	stats["total_probes"] = totalProbes
	stats["latest_status"] = latestStatus

	var upProbes int
	_ = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM isp_metrics WHERE link_id=$1 AND status='up'`, linkID,
	).Scan(&upProbes)
	if totalProbes > 0 {
		stats["uptime_percent"] = float64(upProbes) / float64(totalProbes) * 100.0
	} else {
		stats["uptime_percent"] = 0.0
	}

	return stats
}
