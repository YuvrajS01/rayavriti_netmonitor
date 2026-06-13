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
	var deviceID *int64
	if s := r.URL.Query().Get("deviceId"); s != "" {
		id, err := parseID(s)
		if err == nil {
			deviceID = &id
		}
	}
	sensors, err := h.db.GetSensors(r.Context(), deviceID)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	if sensors == nil {
		sensors = []models.Sensor{}
	}
	httputil.SendOK(w, sensors)
}

// Get returns a single sensor by ID.
func (h *SensorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	s, err := h.db.GetSensor(r.Context(), id)
	if err != nil {
		httputil.SendError(w, 404, "sensor not found")
		return
	}
	httputil.SendOK(w, s)
}

// Create creates a new sensor.
func (h *SensorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var s models.Sensor
	if err := httputil.ParseJSON(r, &s); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	if s.Name == "" {
		httputil.SendError(w, 400, "name is required")
		return
	}
	if s.DeviceID == 0 {
		httputil.SendError(w, 400, "deviceId is required")
		return
	}
	if s.Type == "" {
		httputil.SendError(w, 400, "type is required")
		return
	}
	if s.Interval == 0 {
		s.Interval = 60
	}
	s.Enabled = true
	created, err := h.db.CreateSensor(r.Context(), &s)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendCreated(w, created)
}

// Update updates an existing sensor.
func (h *SensorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if _, err := h.db.GetSensor(r.Context(), id); err != nil {
		httputil.SendError(w, 404, "sensor not found")
		return
	}
	var s models.Sensor
	if err := httputil.ParseJSON(r, &s); err != nil {
		httputil.SendError(w, 400, "invalid body")
		return
	}
	updated, err := h.db.UpdateSensor(r.Context(), id, &s)
	if err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, updated)
}

// Delete deletes a sensor.
func (h *SensorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		httputil.SendError(w, 400, "invalid id")
		return
	}
	if err := h.db.DeleteSensor(r.Context(), id); err != nil {
		httputil.SendError(w, 500, err.Error())
		return
	}
	httputil.SendOK(w, map[string]bool{"deleted": true})
}
