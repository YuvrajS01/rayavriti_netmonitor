package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestInsightCurrent(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 90, Label: "healthy", Trend: "stable"},
				{DeviceID: 2, Score: 20, Label: "critical", Trend: "degrading"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "UpDevice"},
				{ID: 2, Name: "DownDevice"},
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

func TestInsightCurrent_Empty(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{}, nil
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
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
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
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			return []models.HealthHistoryPoint{}, nil
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
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
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
