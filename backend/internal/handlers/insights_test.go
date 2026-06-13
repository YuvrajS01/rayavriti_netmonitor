package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestInsightCurrent(t *testing.T) {
	db := &mockDB{
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "UpDevice", Status: "up"},
				{ID: 2, Name: "DownDevice", Status: "down"},
				{ID: 3, Name: "WarnDevice", Status: "up"},
			}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			highRT := 2000.0
			return []models.Metric{
				{DeviceID: 1, ResponseTime: float64Ptr(50)},
				{DeviceID: 3, ResponseTime: &highRT},
			}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInsightCurrent_AllUp(t *testing.T) {
	db := &mockDB{
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "D1", Status: "up"},
				{ID: 2, Name: "D2", Status: "up"},
			}, nil
		},
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return []models.Metric{}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInsightCurrent_DBError(t *testing.T) {
	db := &mockDB{
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestInsightHistory(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			return []models.Metric{{ID: 1, DeviceID: 1}}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInsightHistory_DBError(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history", "")
	h.History(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
