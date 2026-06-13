package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestSensorList(t *testing.T) {
	db := &mockDB{
		getSensorsFn: func(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
			return []models.Sensor{{ID: 1, DeviceID: 1, Name: "CPU"}}, nil
		},
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSensorList_WithDeviceID(t *testing.T) {
	db := &mockDB{
		getSensorsFn: func(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
			if deviceID == nil || *deviceID != 5 {
				t.Fatalf("expected deviceID 5, got %v", deviceID)
			}
			return []models.Sensor{}, nil
		},
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors?deviceId=5", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSensorList_DBError(t *testing.T) {
	db := &mockDB{
		getSensorsFn: func(ctx context.Context, deviceID *int64) ([]models.Sensor, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSensorGet(t *testing.T) {
	db := &mockDB{
		getSensorFn: func(ctx context.Context, id int64) (*models.Sensor, error) {
			if id == 1 {
				return &models.Sensor{ID: 1, Name: "CPU"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors/1", "")
	req = withChiParams(req, "id", "1")
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSensorGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorGet_NotFound(t *testing.T) {
	db := &mockDB{
		getSensorFn: func(ctx context.Context, id int64) (*models.Sensor, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/sensors/999", "")
	req = withChiParams(req, "id", "999")
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSensorCreate(t *testing.T) {
	db := &mockDB{
		createSensorFn: func(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
			if s.Name == "" {
				t.Fatal("expected name to be set")
			}
			if s.DeviceID == 0 {
				t.Fatal("expected deviceId to be set")
			}
			s.ID = 1
			return s, nil
		},
	}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "CPU Sensor", "deviceId": 1, "type": "system"})
	w, req := authenticatedRequest("POST", "/api/v1/sensors", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSensorCreate_MissingName(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"deviceId": 1, "type": "system"})
	w, req := authenticatedRequest("POST", "/api/v1/sensors", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorCreate_MissingDeviceID(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "CPU", "type": "system"})
	w, req := authenticatedRequest("POST", "/api/v1/sensors", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorCreate_MissingType(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "CPU", "deviceId": 1})
	w, req := authenticatedRequest("POST", "/api/v1/sensors", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorCreate_DBError(t *testing.T) {
	db := &mockDB{
		createSensorFn: func(ctx context.Context, s *models.Sensor) (*models.Sensor, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "CPU", "deviceId": 1, "type": "system"})
	w, req := authenticatedRequest("POST", "/api/v1/sensors", string(body))
	h.Create(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestSensorUpdate(t *testing.T) {
	db := &mockDB{
		getSensorFn: func(ctx context.Context, id int64) (*models.Sensor, error) {
			return &models.Sensor{ID: 1}, nil
		},
		updateSensorFn: func(ctx context.Context, id int64, s *models.Sensor) (*models.Sensor, error) {
			return s, nil
		},
	}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := authenticatedRequest("PUT", "/api/v1/sensors/1", string(body))
	req = withChiParams(req, "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSensorUpdate_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("PUT", "/api/v1/sensors/abc", `{}`)
	req = withChiParams(req, "id", "abc")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorUpdate_NotFound(t *testing.T) {
	db := &mockDB{
		getSensorFn: func(ctx context.Context, id int64) (*models.Sensor, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewSensorHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "X"})
	w, req := authenticatedRequest("PUT", "/api/v1/sensors/999", string(body))
	req = withChiParams(req, "id", "999")
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSensorDelete(t *testing.T) {
	db := &mockDB{
		deleteSensorFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/sensors/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSensorDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/sensors/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSensorDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteSensorFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewSensorHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/sensors/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
