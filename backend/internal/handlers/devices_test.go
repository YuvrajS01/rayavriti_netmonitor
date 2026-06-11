package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestDeviceList_All(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "Router1", IPAddress: "10.0.0.1", Protocol: "ping", Status: "up"},
		{ID: 2, Name: "Switch1", IPAddress: "10.0.0.2", Protocol: "snmp", Status: "down"},
		{ID: 3, Name: "Server1", IPAddress: "10.0.0.3", Protocol: "http", Status: "up"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(data))
	}
}

func TestDeviceList_FilterByStatus(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "R1", Status: "up"},
		{ID: 2, Name: "R2", Status: "down"},
		{ID: 3, Name: "R3", Status: "up"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?status=up", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 2 {
		t.Fatalf("expected 2 up devices, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_FilterByProtocol(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Protocol: "ping"},
		{ID: 2, Protocol: "snmp"},
		{ID: 3, Protocol: "ping"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?protocol=ping", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 2 {
		t.Fatalf("expected 2 ping devices, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_FilterByEnabled(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Enabled: true},
		{ID: 2, Enabled: false},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?enabled=true", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 1 {
		t.Fatalf("expected 1 enabled device, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_Search(t *testing.T) {
	devices := []models.Device{
		{ID: 1, Name: "Router", IPAddress: "10.0.0.1"},
		{ID: 2, Name: "Switch", IPAddress: "192.168.1.1"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?search=router", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["total"].(float64)) != 1 {
		t.Fatalf("expected 1 search result, got %d", int(meta["total"].(float64)))
	}
}

func TestDeviceList_SortByName(t *testing.T) {
	devices := []models.Device{
		{ID: 3, Name: "Charlie"},
		{ID: 1, Name: "Alpha"},
		{ID: 2, Name: "Bravo"},
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?sort=name&dir=asc", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	first := data[0].(map[string]any)
	if first["name"] != "Alpha" {
		t.Fatalf("expected Alpha first, got %s", first["name"])
	}
}

func TestDeviceList_Pagination(t *testing.T) {
	devices := make([]models.Device, 10)
	for i := range devices {
		devices[i] = models.Device{ID: int64(i + 1), Name: "Device"}
	}
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) { return devices, nil }}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices?page=2&pageSize=3", "")
	h.List(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	meta := resp["meta"].(map[string]any)
	if int(meta["page"].(float64)) != 2 {
		t.Fatalf("expected page 2, got %d", int(meta["page"].(float64)))
	}
	data := resp["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("expected 3 items on page 2, got %d", len(data))
	}
}

func TestDeviceList_DBError(t *testing.T) {
	db := &mockDB{getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
		return nil, errors.New("db error")
	}}
	h := NewDeviceHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/devices", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Get ---

func TestDeviceGet_Valid(t *testing.T) {
	db := &mockDB{getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
		if id == 1 {
			return &models.Device{ID: 1, Name: "Router1", IPAddress: "10.0.0.1"}, nil
		}
		return nil, errors.New("not found")
	}}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/devices/1", "", "id", "1")
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeviceGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/devices/abc", "", "id", "abc")
	h.Get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeviceGet_NotFound(t *testing.T) {
	db := &mockDB{getDeviceFn: func(ctx context.Context, id int64) (*models.Device, error) {
		return nil, errors.New("not found")
	}}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/devices/999", "", "id", "999")
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Create ---

func TestDeviceCreate_HappyPath(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			d.ID = 10
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "NewRouter", "ipAddress": "10.0.0.5"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeviceCreate_MissingFields(t *testing.T) {
	db := &mockDB{}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "OnlyName"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeviceCreate_AutoDetectHTTPS(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			if d.Protocol != "https" {
				t.Fatalf("expected protocol https, got %s", d.Protocol)
			}
			if d.Port != 443 {
				t.Fatalf("expected port 443, got %d", d.Port)
			}
			d.ID = 1
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Web", "ipAddress": "https://example.com"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDeviceCreate_AutoDetectHTTP(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			if d.Protocol != "http" {
				t.Fatalf("expected protocol http, got %s", d.Protocol)
			}
			if d.Port != 80 {
				t.Fatalf("expected port 80, got %d", d.Port)
			}
			d.ID = 1
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Web", "ipAddress": "http://example.com"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDeviceCreate_CustomProtocol(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			if d.Protocol != "ssh" {
				t.Fatalf("expected protocol ssh, got %s", d.Protocol)
			}
			d.ID = 1
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Server", "ipAddress": "10.0.0.1", "protocol": "ssh"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDeviceCreate_DefaultProtocol(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			if d.Protocol != "ping" {
				t.Fatalf("expected default protocol ping, got %s", d.Protocol)
			}
			d.ID = 1
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Dev", "ipAddress": "10.0.0.1"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestDeviceCreate_DBError(t *testing.T) {
	db := &mockDB{
		createDeviceFn: func(ctx context.Context, d *models.Device) (*models.Device, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Dev", "ipAddress": "10.0.0.1"})
	w, req := authenticatedRequest("POST", "/api/v1/devices", string(body))
	h.Create(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Update ---

func TestDeviceUpdate_Valid(t *testing.T) {
	db := &mockDB{
		updateDeviceFn: func(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
			d.ID = id
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := makeRequestWithParams("PUT", "/api/v1/devices/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeviceUpdate_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("PUT", "/api/v1/devices/abc", `{"name":"x"}`, "id", "abc")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeviceUpdate_DBError(t *testing.T) {
	db := &mockDB{
		updateDeviceFn: func(ctx context.Context, id int64, d *models.Device) (*models.Device, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewDeviceHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "X"})
	w, req := makeRequestWithParams("PUT", "/api/v1/devices/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Delete ---

func TestDeviceDelete_Valid(t *testing.T) {
	db := &mockDB{
		deleteDeviceFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/devices/1", "", "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeviceDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/devices/abc", "", "id", "abc")
	h.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeviceDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteDeviceFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewDeviceHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/devices/1", "", "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- normalizeHost ---

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"10.0.0.1", "10.0.0.1"},
		{"  10.0.0.1  ", "10.0.0.1"},
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"HTTPS://example.com", "example.com"},
		{"http://example.com/", "example.com"},
		{"https://example.com:8443/", "example.com:8443"},
	}
	for _, tt := range tests {
		got := normalizeHost(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeHost(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
