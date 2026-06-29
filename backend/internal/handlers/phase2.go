package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type Phase2Handler struct {
	db     database.Database
	phase2 database.Phase2Store
}

func NewPhase2Handler(db database.Database) *Phase2Handler {
	phase2, _ := db.(database.Phase2Store)
	return &Phase2Handler{db: db, phase2: phase2}
}

func (h *Phase2Handler) requireStore(w http.ResponseWriter) bool {
	if h.phase2 == nil {
		httputil.SendError(w, http.StatusNotImplemented, "phase 2 storage is not available")
		return false
	}
	return true
}

func (h *Phase2Handler) Summary(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	summary, err := h.phase2.Phase2Summary(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, summary)
}

func (h *Phase2Handler) List(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filters := map[string]string{}
		for key, vals := range r.URL.Query() {
			if len(vals) > 0 {
				snake := toSnakeLocal(key)
				if snake == "cursor" || snake == "limit" || snake == "format" {
					continue
				}
				filters[snake] = vals[0]
			}
		}
		if !h.requireStore(w) {
			return
		}

		cursor := r.URL.Query().Get("cursor")
		if cursor == "" {
			cursor = r.URL.Query().Get("Cursor")
		}
		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if l := r.URL.Query().Get("Limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		if cursor != "" || r.URL.Query().Get("cursor") != "" || r.URL.Query().Get("Cursor") != "" {
			rows, nextCursor, hasMore, err := h.phase2.ListPhase2Cursor(r.Context(), resource, filters, cursor, limit)
			if err != nil {
				httputil.SendError(w, http.StatusInternalServerError, err.Error())
				return
			}
			httputil.SendOK(w, map[string]any{
				"data":        rows,
				"next_cursor": nextCursor,
				"has_more":    hasMore,
			})
			return
		}

		rows, err := h.phase2.ListPhase2(r.Context(), resource, filters)
		if err != nil {
			httputil.SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if resource == "locations" && r.URL.Query().Get("format") == "tree" {
			httputil.SendOK(w, buildLocationTree(rows))
			return
		}
		httputil.SendOK(w, rows)
	}
}

func (h *Phase2Handler) Get(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if !h.requireStore(w) {
			return
		}
		item, err := h.phase2.GetPhase2(r.Context(), resource, id)
		if err != nil {
			httputil.SendError(w, http.StatusNotFound, "not found")
			return
		}
		httputil.SendOK(w, item)
	}
}

func (h *Phase2Handler) Create(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := httputil.ParseJSON(r, &body); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if !h.requireStore(w) {
			return
		}
		item, err := h.phase2.CreatePhase2(r.Context(), resource, body)
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.SendCreated(w, item)
	}
}

func (h *Phase2Handler) Update(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var body map[string]any
		if err := httputil.ParseJSON(r, &body); err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if !h.requireStore(w) {
			return
		}
		item, err := h.phase2.UpdatePhase2(r.Context(), resource, id, body)
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.SendOK(w, item)
	}
}

func (h *Phase2Handler) Delete(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			httputil.SendError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if !h.requireStore(w) {
			return
		}
		if err := h.phase2.DeletePhase2(r.Context(), resource, id); err != nil {
			httputil.SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.SendOK(w, map[string]bool{"deleted": true})
	}
}

func (h *Phase2Handler) LocationStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := map[string]int{"up": 0, "down": 0, "warning": 0, "maintenance": 0, "unknown": 0}
	for _, d := range devices {
		if d.LocationID == nil || *d.LocationID != id {
			continue
		}
		key := d.Status
		if _, ok := status[key]; !ok {
			key = "unknown"
		}
		status[key]++
	}
	httputil.SendOK(w, status)
}

func (h *Phase2Handler) Topology(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	locations, err := h.phase2.ListPhase2(r.Context(), "locations", nil)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	links := []map[string]any{}
	for _, d := range devices {
		if d.ParentDeviceID != nil {
			links = append(links, map[string]any{
				"sourceDeviceId": *d.ParentDeviceID,
				"targetDeviceId": d.ID,
				"port":           d.DependencyPort,
			})
		}
	}
	httputil.SendOK(w, map[string]any{
		"locations": buildLocationTree(locations),
		"devices":   devices,
		"links":     links,
	})
}

func (h *Phase2Handler) PublicStatusJSON(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	services, err := h.phase2.ListPhase2(r.Context(), "status_page_services", map[string]string{"enabled": "true"})
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	incidents, _ := h.phase2.ListPhase2(r.Context(), "status_page_incidents", nil)
	groups := map[string][]map[string]any{}
	overall := "operational"
	for _, svc := range services {
		status := "operational"
		if enabled, ok := svc["enabled"].(bool); ok && !enabled {
			continue
		}
		group, _ := svc["group_name"].(string)
		if group == "" {
			group = "General"
		}
		entry := map[string]any{
			"id":          svc["id"],
			"name":        svc["name"],
			"description": svc["description"],
			"status":      status,
			"uptime_30d":  100,
		}
		groups[group] = append(groups[group], entry)
	}
	active := []map[string]any{}
	for _, inc := range incidents {
		if inc["resolved_at"] == nil {
			active = append(active, inc)
			if overall == "operational" {
				overall = "degraded"
			}
		}
	}
	groupRows := []map[string]any{}
	for name, items := range groups {
		groupRows = append(groupRows, map[string]any{"name": name, "services": items})
	}
	httputil.SendOK(w, map[string]any{
		"campus":           "Main Campus",
		"overall_status":   overall,
		"last_updated":     time.Now().UTC().Format(time.RFC3339),
		"groups":           groupRows,
		"active_incidents": active,
	})
}

func (h *Phase2Handler) PublicStatusHTML(w http.ResponseWriter, r *http.Request) {
	if !h.requireStore(w) {
		return
	}
	services, _ := h.phase2.ListPhase2(r.Context(), "status_page_services", map[string]string{"enabled": "true"})
	incidents, _ := h.phase2.ListPhase2(r.Context(), "status_page_incidents", nil)

	groups := map[string][]map[string]any{}
	overall := "operational"
	for _, svc := range services {
		status := "operational"
		if enabled, ok := svc["enabled"].(bool); ok && !enabled {
			continue
		}
		group, _ := svc["group_name"].(string)
		if group == "" {
			group = "General"
		}
		entry := map[string]any{
			"id":          svc["id"],
			"name":        svc["name"],
			"description": svc["description"],
			"status":      status,
			"uptime_30d":  100,
		}
		groups[group] = append(groups[group], entry)
	}

	activeIncidents := []map[string]any{}
	for _, inc := range incidents {
		if inc["resolved_at"] == nil {
			activeIncidents = append(activeIncidents, inc)
			if overall == "operational" {
				overall = "degraded"
			}
		}
	}

	overallColor := "#245a3a"
	overallBg := "#dff1e4"
	overallLabel := "Operational"
	switch overall {
	case "degraded":
		overallColor = "#856404"
		overallBg = "#fff3cd"
		overallLabel = "Degraded"
	case "major_outage":
		overallColor = "#721c24"
		overallBg = "#f8d7da"
		overallLabel = "Major Outage"
	}

	var sb strings.Builder
	sb.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="60"><title>Campus Network Status</title><link rel="preconnect" href="https://fonts.googleapis.com"><link href="https://fonts.googleapis.com/css2?family=League+Spartan:wght@400;700;800&family=Space+Grotesk:wght@400;500;700&display=swap" rel="stylesheet"><style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0e0e09;color:#e6e1d5;font-family:'Space Grotesk',system-ui,sans-serif}
.wrap{max-width:860px;margin:0 auto;padding:48px 24px}
.header{display:flex;justify-content:space-between;align-items:flex-end;border-bottom:3px solid #e6e1d5;padding-bottom:20px;margin-bottom:36px;flex-wrap:wrap;gap:16px}
h1{font-family:'League Spartan',sans-serif;font-weight:800;font-size:28px;color:#e6e1d5;letter-spacing:-0.02em}
.meta{color:#9d9689;font-size:13px;margin-top:4px}
.badge{display:inline-flex;align-items:center;gap:6px;padding:6px 14px;border-radius:999px;font-family:'League Spartan',sans-serif;font-weight:700;font-size:12px;text-transform:uppercase;letter-spacing:0.05em}
.badge-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.group-section{margin-bottom:32px}
.group-name{font-family:'League Spartan',sans-serif;font-weight:700;font-size:14px;text-transform:uppercase;letter-spacing:0.06em;color:#9d9689;margin-bottom:12px;padding-left:2px}
.svc{background:#1a1a14;border:1px solid #2e2d25;border-radius:10px;padding:16px 20px;margin-bottom:8px;display:flex;justify-content:space-between;align-items:center;transition:border-color 0.2s}
.svc:hover{border-color:#d9fd3a40}
.svc-left{min-width:0}
.svc-name{font-family:'League Spartan',sans-serif;font-weight:700;font-size:16px;color:#e6e1d5}
.svc-desc{color:#9d9689;font-size:13px;margin-top:2px}
.svc-uptime{color:#9d9689;font-size:12px;margin-top:6px;font-weight:500}
.svc-uptime strong{color:#d9fd3a}
.pill{padding:4px 12px;border-radius:999px;font-family:'League Spartan',sans-serif;font-weight:700;font-size:11px;text-transform:uppercase;letter-spacing:0.04em;white-space:nowrap;flex-shrink:0}
.pill-operational{background:#dff1e4;color:#245a3a}
.pill-degraded{background:#fff3cd;color:#856404}
.pill-outage{background:#f8d7da;color:#721c24}
.inc-section{margin-top:40px}
.inc-section h2{font-family:'League Spartan',sans-serif;font-weight:800;font-size:20px;margin-bottom:16px;color:#e6e1d5}
.inc{background:#1a1a14;border-left:4px solid #ff7351;border-radius:0 10px 10px 0;padding:16px 20px;margin-bottom:8px}
.inc-title{font-family:'League Spartan',sans-serif;font-weight:700;font-size:16px;color:#ff7351}
.inc-meta{color:#9d9689;font-size:12px;margin-top:4px}
.inc-msg{color:#e6e1d5;font-size:14px;margin-top:8px;line-height:1.5}
.footer{margin-top:48px;padding-top:16px;border-top:1px solid #2e2d25;text-align:center;color:#6f6a5f;font-size:12px}
.footer a{color:#d9fd3a;text-decoration:none}
.empty{text-align:center;padding:40px;color:#9d9689;font-size:14px}
@media(max-width:640px){.header{display:block}.svc{flex-direction:column;align-items:flex-start;gap:10px}}
@media print{body{background:#fff;color:#000}.svc{border-color:#ccc}.header{border-color:#000}}
</style></head><body><main class="wrap">`)

	fmt.Fprintf(&sb, `<section class="header"><div><h1>Campus Network Status</h1><p class="meta">Auto-refreshes every 60 seconds &middot; Last checked: %s</p></div><div class="badge" style="background:%s;color:%s"><span class="badge-dot" style="background:%s"></span>%s</div></section>`,
		time.Now().UTC().Format("15:04 UTC"),
		overallBg, overallColor, overallColor, overallLabel,
	)

	if len(groups) == 0 && len(activeIncidents) == 0 {
		sb.WriteString(`<div class="empty"><p>No services configured on the status page yet.</p><p style="margin-top:8px">Ask IT to add services in <strong>Status Page Admin</strong>.</p></div>`)
	}

	for name, items := range groups {
		fmt.Fprintf(&sb, `<section class="group-section"><div class="group-name">%s</div>`, html.EscapeString(name))
		for _, svc := range items {
			status, _ := svc["status"].(string)
			pillClass := "pill-operational"
			pillLabel := "Operational"
			switch status {
			case "degraded_performance":
				pillClass = "pill-degraded"
				pillLabel = "Degraded"
			case "major_outage":
				pillClass = "pill-outage"
				pillLabel = "Outage"
			}
			uptimeStr := ""
			if u, ok := svc["uptime_30d"].(float64); ok && u < 100 {
				uptimeStr = fmt.Sprintf(`<div class="svc-uptime">Uptime: <strong>%.1f%%</strong></div>`, u)
			}
			fmt.Fprintf(&sb, `<article class="svc"><div class="svc-left"><div class="svc-name">%s</div><div class="svc-desc">%s</div>%s</div><span class="pill %s">%s</span></article>`,
				html.EscapeString(fmt.Sprint(svc["name"])),
				html.EscapeString(fmt.Sprint(svc["description"])),
				uptimeStr,
				pillClass, pillLabel,
			)
		}
		sb.WriteString(`</section>`)
	}

	sb.WriteString(`<section class="inc-section"><h2>Active Incidents</h2>`)
	activeCount := 0
	for _, inc := range activeIncidents {
		activeCount++
		title := html.EscapeString(fmt.Sprint(inc["title"]))
		msg := html.EscapeString(fmt.Sprint(inc["message"]))
		status := html.EscapeString(fmt.Sprint(inc["status"]))
		severity := fmt.Sprint(inc["severity"])
		fmt.Fprintf(&sb, `<article class="inc"><div class="inc-title">%s</div><div class="inc-meta">%s &middot; %s</div>%s</article>`,
			title, severity, status,
			func() string {
				if msg != "" {
					return `<div class="inc-msg">` + msg + `</div>`
				}
				return ""
			}(),
		)
	}
	if activeCount == 0 {
		sb.WriteString(`<div class="empty">No active incidents. All systems running smoothly.</div>`)
	}
	sb.WriteString(`</section>`)

	fmt.Fprintf(&sb, `<footer class="footer">Powered by <a href="/">Rayavriti NetMonitor</a> &middot; %s</footer>`, time.Now().UTC().Format("2006"))
	sb.WriteString(`</main></body></html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	_, _ = w.Write([]byte(sb.String()))
}

func buildLocationTree(rows []map[string]any) []map[string]any {
	byID := map[int64]map[string]any{}
	roots := []map[string]any{}
	for _, row := range rows {
		id := asInt64(row["id"])
		node := map[string]any{}
		for k, v := range row {
			node[k] = v
		}
		node["children"] = []map[string]any{}
		byID[id] = node
	}
	for _, node := range byID {
		parentID := asInt64(node["parent_id"])
		if parentID == 0 {
			roots = append(roots, node)
			continue
		}
		parent := byID[parentID]
		if parent == nil {
			roots = append(roots, node)
			continue
		}
		children, _ := parent["children"].([]map[string]any)
		parent["children"] = append(children, node)
	}
	return roots
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func toSnakeLocal(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
