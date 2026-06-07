package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// SensorHandler serves /api/v1/sensors — virtual sensors attached to devices.
type SensorHandler struct{ db database.Database }

func NewSensorHandler(db database.Database) *SensorHandler { return &SensorHandler{db: db} }

// List returns all sensors for a device (or all if no deviceId query param).
func (h *SensorHandler) List(w http.ResponseWriter, r *http.Request) {
	devices, err := h.db.GetDevices(r.Context())
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	// Derive sensors from device list — one virtual sensor per monitoring protocol.
	var sensors []map[string]any
	for _, d := range devices {
		sensors = append(sensors, deviceToSensor(d))
	}
	if sensors == nil {
		sensors = []map[string]any{}
	}
	httputil.SendOK(w, sensors)
}

// Get returns a single sensor by device ID.
func (h *SensorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	d, err := h.db.GetDevice(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "sensor not found")
		return
	}
	httputil.SendOK(w, deviceToSensor(*d))
}

func deviceToSensor(d models.Device) map[string]any {
	return map[string]any{
		"id":         d.ID,
		"deviceId":   d.ID,
		"name":       d.Name + " — " + d.Protocol,
		"protocol":   d.Protocol,
		"enabled":    d.Enabled,
		"status":     d.Status,
		"interval":   d.Interval,
		"ipAddress":  d.IPAddress,
	}
}
