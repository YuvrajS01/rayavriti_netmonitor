package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

type DeviceHandler struct{ db database.Database }

func NewDeviceHandler(db database.Database) *DeviceHandler { return &DeviceHandler{db: db} }

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.SendOK(w, devices)
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
		Ports      []int  `json:"ports"`
		TimeoutMs  int    `json:"timeoutMs"`
		Concurrency int   `json:"concurrency"`
	}
	_ = httputil.ParseJSON(r, &body)

	// For now, return a placeholder - actual port scanning will be implemented in Phase 5
	result := map[string]any{
		"deviceId":   device.ID,
		"deviceName": device.Name,
		"openPorts":  []int{},
		"changes":    []string{},
		"alerts":     []string{},
		"scannedAt":  "2026-06-08T00:00:00Z",
	}
	httputil.SendOK(w, result)
}
