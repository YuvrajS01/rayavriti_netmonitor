package handlers

import (
	"fmt"
	"html"
	"net/http"
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
				filters[toSnakeLocal(key)] = vals[0]
			}
		}
		if !h.requireStore(w) {
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
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="60"><title>Campus Network Status</title><style>body{margin:0;font-family:system-ui,-apple-system,sans-serif;background:#f5f2ea;color:#1f2421}.wrap{max-width:920px;margin:auto;padding:40px 20px}.head{display:flex;justify-content:space-between;gap:20px;align-items:end;border-bottom:3px solid #1f2421;padding-bottom:18px}.status{font-weight:800;text-transform:uppercase;color:#2f6f4e}.grid{display:grid;gap:12px;margin-top:24px}.svc,.inc{background:#fffaf0;border:1px solid #d8d0bd;border-radius:8px;padding:16px}.name{font-weight:800}.muted{color:#6f6a5f}.pill{float:right;background:#dff1e4;color:#245a3a;border-radius:999px;padding:4px 10px;font-size:12px;font-weight:800;text-transform:uppercase}@media(max-width:640px){.head{display:block}}</style></head><body><main class="wrap"><section class="head"><div><h1>Campus Network Status</h1><p class="muted">Auto-refreshes every 60 seconds.</p></div><div class="status">Operational</div></section><section class="grid">`)
	if len(services) == 0 {
		b.WriteString(`<div class="svc"><div class="name">No public services configured</div><p class="muted">Ask IT to add status page services.</p></div>`)
	}
	for _, svc := range services {
		b.WriteString(fmt.Sprintf(`<article class="svc"><span class="pill">operational</span><div class="name">%s</div><p class="muted">%s</p></article>`, html.EscapeString(fmt.Sprint(svc["name"])), html.EscapeString(fmt.Sprint(svc["description"]))))
	}
	b.WriteString(`</section><h2>Active incidents</h2><section class="grid">`)
	active := 0
	for _, inc := range incidents {
		if inc["resolved_at"] != nil {
			continue
		}
		active++
		b.WriteString(fmt.Sprintf(`<article class="inc"><div class="name">%s</div><p>%s</p><p class="muted">%s</p></article>`, html.EscapeString(fmt.Sprint(inc["title"])), html.EscapeString(fmt.Sprint(inc["message"])), html.EscapeString(fmt.Sprint(inc["status"]))))
	}
	if active == 0 {
		b.WriteString(`<div class="inc muted">No active public incidents.</div>`)
	}
	b.WriteString(`</section></main></body></html>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
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
