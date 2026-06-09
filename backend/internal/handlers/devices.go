package handlers

import (
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/scanner"
)

type DeviceHandler struct{ db database.Database }

func NewDeviceHandler(db database.Database) *DeviceHandler { return &DeviceHandler{db: db} }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()

	// Filter by status
	if status := q.Get("status"); status != "" {
		filtered := make([]models.Device, 0)
		for _, d := range devices {
			if d.Status == status {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	// Filter by protocol
	if protocol := q.Get("protocol"); protocol != "" {
		filtered := make([]models.Device, 0)
		for _, d := range devices {
			if d.Protocol == protocol {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	// Filter by enabled
	if enabledStr := q.Get("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		filtered := make([]models.Device, 0)
		for _, d := range devices {
			if d.Enabled == enabled {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	// Search by name or IP
	if search := q.Get("search"); search != "" {
		search = strings.ToLower(search)
		filtered := make([]models.Device, 0)
		for _, d := range devices {
			if strings.Contains(strings.ToLower(d.Name), search) ||
				strings.Contains(strings.ToLower(d.IPAddress), search) {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	total := len(devices)

	// Sort
	sortBy := q.Get("sort")
	sortDir := q.Get("dir")
	if sortBy == "" {
		sortBy = "id"
	}
	if sortDir == "" {
		sortDir = "asc"
	}
	sort.Slice(devices, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = devices[i].Name < devices[j].Name
		case "status":
			less = devices[i].Status < devices[j].Status
		case "protocol":
			less = devices[i].Protocol < devices[j].Protocol
		case "ipAddress":
			less = devices[i].IPAddress < devices[j].IPAddress
		default:
			less = devices[i].ID < devices[j].ID
		}
		if sortDir == "desc" {
			return !less
		}
		return less
	})

	// Pagination
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
	start := (page - 1) * pageSize
	if start > len(devices) {
		start = len(devices)
	}
	end := start + pageSize
	if end > len(devices) {
		end = len(devices)
	}
	paged := devices[start:end]
	if paged == nil {
		paged = []models.Device{}
	}

	httputil.SendOKWithMeta(w, paged, &httputil.ResponseMeta{
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
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, map[string]string{"message": "deleted"})
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
	concurrency := 100
	if body.Concurrency > 0 {
		concurrency = body.Concurrency
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
			ResponseTime: nil,
			ScannedAt:    time.Now(),
		})
	}

	if err := h.db.UpsertPortScanResults(r.Context(), device.ID, modelResults); err != nil {
		slog.Error("Failed to persist port scan results", "device_id", device.ID, "error", err)
	}

	httputil.SendOK(w, map[string]any{
		"deviceId":   device.ID,
		"deviceName": device.Name,
		"host":       device.IPAddress,
		"scannedPorts": len(portsToScan),
		"openPorts":  len(openPorts),
		"results":    modelResults,
		"changes":    []string{},
		"scannedAt":  time.Now().Format(time.RFC3339),
	})
}
