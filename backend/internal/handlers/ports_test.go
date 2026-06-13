package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestPortsForDevice(t *testing.T) {
	db := &mockDB{
		getPortScanResultsFn: func(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
			if deviceID != 1 {
				t.Fatalf("expected deviceID 1, got %d", deviceID)
			}
			return []models.PortScanResult{
				{DeviceID: 1, Port: 22, State: "open", Service: "SSH"},
				{DeviceID: 1, Port: 80, State: "open", Service: "HTTP"},
			}, nil
		},
	}
	h := NewPortsHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/ports/device/1", "")
	req = withChiParams(req, "id", "1")
	h.ForDevice(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPortsForDevice_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewPortsHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/ports/device/abc", "")
	req = withChiParams(req, "id", "abc")
	h.ForDevice(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPortsForDevice_DBError(t *testing.T) {
	db := &mockDB{
		getPortScanResultsFn: func(ctx context.Context, deviceID int64) ([]models.PortScanResult, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewPortsHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/ports/device/1", "")
	req = withChiParams(req, "id", "1")
	h.ForDevice(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
