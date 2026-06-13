package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestSimulatorMetrics_WithTimestamp(t *testing.T) {
	db := &mockDB{
		recordMetricFn: func(ctx context.Context, m *models.Metric) error {
			if m.Timestamp.IsZero() {
				t.Fatal("expected non-zero timestamp")
			}
			return nil
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal(map[string]any{
		"deviceId":  1,
		"status":    "up",
		"timestamp": "2026-06-11T14:00:00Z",
	})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/metrics", string(body))
	h.Metrics(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSimulatorMetrics_NoTimestamp(t *testing.T) {
	db := &mockDB{
		recordMetricFn: func(ctx context.Context, m *models.Metric) error {
			if m.Timestamp.IsZero() {
				t.Fatal("expected auto-filled timestamp")
			}
			return nil
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal(map[string]any{"deviceId": 1, "status": "up"})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/metrics", string(body))
	h.Metrics(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSimulatorMetrics_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := NewSimulatorHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/simulator/metrics", "not-json")
	h.Metrics(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSimulatorMetrics_DBError(t *testing.T) {
	db := &mockDB{
		recordMetricFn: func(ctx context.Context, m *models.Metric) error {
			return errors.New("db error")
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal(map[string]any{"deviceId": 1})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/metrics", string(body))
	h.Metrics(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSimulatorFlows(t *testing.T) {
	db := &mockDB{
		recordFlowsFn: func(ctx context.Context, flows []models.Flow) error {
			if len(flows) != 2 {
				t.Fatalf("expected 2 flows, got %d", len(flows))
			}
			for _, f := range flows {
				if f.Timestamp.IsZero() {
					t.Fatal("expected auto-filled timestamp")
				}
			}
			return nil
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal([]map[string]any{
		{"srcIp": "10.0.0.1", "dstIp": "10.0.0.2"},
		{"srcIp": "10.0.0.3", "dstIp": "10.0.0.4"},
	})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/flows", string(body))
	h.Flows(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["recorded"] != float64(2) {
		t.Fatalf("expected recorded=2, got %v", data["recorded"])
	}
}

func TestSimulatorFlows_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := NewSimulatorHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/simulator/flows", "not-json")
	h.Flows(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSimulatorFlows_DBError(t *testing.T) {
	db := &mockDB{
		recordFlowsFn: func(ctx context.Context, flows []models.Flow) error {
			return errors.New("db error")
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal([]map[string]any{{"srcIp": "10.0.0.1"}})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/flows", string(body))
	h.Flows(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSimulatorAlert(t *testing.T) {
	db := &mockDB{
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			if a.Status != "active" {
				t.Fatalf("expected status active, got %s", a.Status)
			}
			a.ID = 1
			return a, nil
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal(map[string]any{"severity": "critical", "message": "test alert"})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/alert", string(body))
	h.Alert(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSimulatorAlert_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := NewSimulatorHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/simulator/alert", "not-json")
	h.Alert(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSimulatorAlert_DBError(t *testing.T) {
	db := &mockDB{
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewSimulatorHandler(db)
	body, _ := json.Marshal(map[string]any{"severity": "warning"})
	w, req := authenticatedRequest("POST", "/api/v1/simulator/alert", string(body))
	h.Alert(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
