package server

import (
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestMessageDeviceID(t *testing.T) {
	id := int64(9)
	tests := []struct {
		name string
		data any
		want int64
	}{
		{"status event map with int64", map[string]any{"deviceId": int64(7), "status": "down"}, 7},
		{"redis round-trip float64", map[string]any{"deviceId": float64(7)}, 7},
		{"legacy snake_case key", map[string]any{"device_id": 7}, 7},
		{"metric struct pointer", &models.Metric{DeviceID: 5}, 5},
		{"alert struct", &models.Alert{DeviceID: id}, 9},
		{"no device", map[string]any{"foo": 1}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageDeviceID(tt.data); got != tt.want {
				t.Fatalf("messageDeviceID() = %d, want %d", got, tt.want)
			}
		})
	}
}
