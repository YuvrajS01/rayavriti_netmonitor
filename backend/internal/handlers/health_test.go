package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestHealth_OK(t *testing.T) {
	db := &mockDB{
		pingFn: func(ctx context.Context) error { return nil },
	}
	h := NewHealthHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/health", "")
	h.Health(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", data["status"])
	}
	if data["database"] != "ok" {
		t.Fatalf("expected database ok, got %v", data["database"])
	}
}

func TestHealth_DBError(t *testing.T) {
	db := &mockDB{
		pingFn: func(ctx context.Context) error { return errors.New("connection refused") },
	}
	h := NewHealthHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/health", "")
	h.Health(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	dbStatus := data["database"].(string)
	if dbStatus == "ok" {
		t.Fatal("expected database error status")
	}
}

func TestHealth_Stats(t *testing.T) {
	db := &mockDB{
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{"totalDevices": 5, "totalAlerts": 3}, nil
		},
	}
	h := NewHealthHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/health/stats", "")
	h.Stats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealth_Stats_DBError(t *testing.T) {
	db := &mockDB{
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewHealthHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/health/stats", "")
	h.Stats(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
