package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/scanner"
)

type DeviceHandler struct{ db database.Database }

func normalizeHost(raw string) string {
	h := strings.TrimSpace(raw)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(h), prefix) {
			h = h[len(prefix):]
			break
		}
	}
	return strings.TrimRight(h, "/")
}

func NewDeviceHandler(db database.Database) *DeviceHandler { return &DeviceHandler{db: db} }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := database.DeviceFilter{
		Status:   q.Get("status"),
		Protocol: q.Get("protocol"),
		Search:   q.Get("search"),
		SortBy:   q.Get("sort"),
		SortDir:  q.Get("dir"),
	}

	if enabledStr := q.Get("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		f.Enabled = &enabled
	}

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	f.Limit = pageSize
	f.Offset = (page - 1) * pageSize

	devices, total, err := h.db.GetDevicesFiltered(r.Context(), f)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.SendOKWithMeta(w, devices, &httputil.ResponseMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (h *DeviceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	d, err := h.db.GetDevice(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "device not found")
		return
	}
	httputil.SendOK(w, d)
}

func (h *DeviceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var d models.Device
	if err := httputil.ParseJSON(r, &d); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if d.Name == "" || d.IPAddress == "" {
		httputil.SendError(w, http.StatusBadRequest, "name and ipAddress are required")
		return
	}
	// Auto-detect protocol from pasted URL before normalizing
	origIP := strings.TrimSpace(d.IPAddress)
	if strings.HasPrefix(strings.ToLower(origIP), "https://") {
		if d.Protocol == "" || d.Protocol == "ping" {
			d.Protocol = "https"
		}
	} else if strings.HasPrefix(strings.ToLower(origIP), "http://") {
		if d.Protocol == "" || d.Protocol == "ping" {
			d.Protocol = "http"
		}
	}
	d.IPAddress = normalizeHost(origIP)
	if d.Protocol == "" {
		d.Protocol = "ping"
	}
	if d.Port == 0 {
		switch d.Protocol {
		case "http":
			d.Port = 80
		case "https":
			d.Port = 443
		case "snmp":
			d.Port = 161
		case "ssh":
			d.Port = 22
		case "port":
			httputil.SendError(w, http.StatusBadRequest, "port protocol requires a valid port number")
			return
		default:
			d.Port = 0
		}
	}
	if d.Interval == 0 {
		d.Interval = 60
	}
	d.Enabled = true
	d.Status = "unknown"
	created, err := h.db.CreateDevice(r.Context(), &d)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var d models.Device
	if err := httputil.ParseJSON(r, &d); err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if d.IPAddress != "" {
		d.IPAddress = normalizeHost(d.IPAddress)
	}
	updated, err := h.db.UpdateDevice(r.Context(), id, &d)
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, updated)
}

func (h *DeviceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.DeleteDevice(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.SendError(w, http.StatusNotFound, "device not found")
			return
		}
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
}

var portServiceNames = map[int]string{
	21: "FTP", 22: "SSH", 23: "Telnet", 25: "SMTP", 53: "DNS",
	80: "HTTP", 110: "POP3", 143: "IMAP", 161: "SNMP", 443: "HTTPS",
	465: "SMTPS", 587: "Submission", 993: "IMAPS", 995: "POP3S",
	1433: "MSSQL", 3306: "MySQL", 3389: "RDP", 5432: "PostgreSQL",
	6379: "Redis", 8080: "HTTP-Alt", 8443: "HTTPS-Alt", 27017: "MongoDB",
}

func (h *DeviceHandler) ScanPorts(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, http.StatusBadRequest, "invalid id")
		return
	}
	device, err := h.db.GetDevice(r.Context(), id)
	if err != nil {
		httputil.SendError(w, http.StatusNotFound, "device not found")
		return
	}

	var body struct {
		Ports       []int `json:"ports"`
		TimeoutMs   int   `json:"timeoutMs"`
		Concurrency int   `json:"concurrency"`
	}
	_ = httputil.ParseJSON(r, &body)

	portsToScan := body.Ports
	if len(portsToScan) == 0 {
		portsToScan = scanner.CommonPorts
	}
	timeout := 2 * time.Second
	if body.TimeoutMs > 0 {
		timeout = time.Duration(body.TimeoutMs) * time.Millisecond
	}
	if timeout > 10*time.Second {
		timeout = 10 * time.Second
	}
	concurrency := 100
	if body.Concurrency > 0 {
		concurrency = body.Concurrency
	}
	if concurrency > 500 {
		concurrency = 500
	}

	results := scanner.ScanPorts(r.Context(), device.IPAddress, portsToScan, scanner.ScanOptions{
		Timeout:     timeout,
		Concurrency: concurrency,
	})

	// Convert to models and persist
	var modelResults []models.PortScanResult
	var openPorts []int
	for _, r := range results {
		state := "closed"
		if r.Open {
			state = "open"
			openPorts = append(openPorts, r.Port)
		}
		modelResults = append(modelResults, models.PortScanResult{
			DeviceID:     device.ID,
			Port:         r.Port,
			Protocol:     "tcp",
			State:        state,
			Service:      portServiceNames[r.Port],
			ResponseTime: nil,
			ScannedAt:    time.Now(),
		})
	}

	if err := h.db.UpsertPortScanResults(r.Context(), device.ID, modelResults); err != nil {
		slog.Error("Failed to persist port scan results", "device_id", device.ID, "error", err)
	}

	httputil.SendOK(w, map[string]any{
		"deviceId":     device.ID,
		"deviceName":   device.Name,
		"host":         device.IPAddress,
		"scannedPorts": len(portsToScan),
		"openPorts":    len(openPorts),
		"results":      modelResults,
		"changes":      []string{},
		"scannedAt":    time.Now().Format(time.RFC3339),
	})
}
