package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSystemInfo(t *testing.T) {
	h := NewSystemHandler()
	w, req := authenticatedRequest("GET", "/api/v1/system/info", "")
	h.Info(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)

	// Check that key fields are present
	if _, ok := data["cpu"]; !ok {
		t.Fatal("expected cpu field in response")
	}
	if _, ok := data["memory"]; !ok {
		t.Fatal("expected memory field in response")
	}
	if _, ok := data["disk"]; !ok {
		t.Fatal("expected disk field in response")
	}
	if _, ok := data["uptime"]; !ok {
		t.Fatal("expected uptime field in response")
	}
	if _, ok := data["goVersion"]; !ok {
		t.Fatal("expected goVersion field in response")
	}
	if _, ok := data["hostname"]; !ok {
		t.Fatal("expected hostname field in response")
	}
	if _, ok := data["loadAvg"]; !ok {
		t.Fatal("expected loadAvg field in response")
	}
}

func TestSystemInfo_CpuStructure(t *testing.T) {
	h := NewSystemHandler()
	w, req := authenticatedRequest("GET", "/api/v1/system/info", "")
	h.Info(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	cpu := data["cpu"].(map[string]any)
	if _, ok := cpu["usage"]; !ok {
		t.Fatal("expected cpu.usage")
	}
	if _, ok := cpu["cores"]; !ok {
		t.Fatal("expected cpu.cores")
	}
	if _, ok := cpu["model"]; !ok {
		t.Fatal("expected cpu.model")
	}
}

func TestSystemInfo_MemoryStructure(t *testing.T) {
	h := NewSystemHandler()
	w, req := authenticatedRequest("GET", "/api/v1/system/info", "")
	h.Info(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	mem := data["memory"].(map[string]any)
	if _, ok := mem["used"]; !ok {
		t.Fatal("expected memory.used")
	}
	if _, ok := mem["total"]; !ok {
		t.Fatal("expected memory.total")
	}
	if _, ok := mem["percent"]; !ok {
		t.Fatal("expected memory.percent")
	}
}

func TestSystemInfo_DiskStructure(t *testing.T) {
	h := NewSystemHandler()
	w, req := authenticatedRequest("GET", "/api/v1/system/info", "")
	h.Info(w, req)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	disk := data["disk"].(map[string]any)
	if _, ok := disk["used"]; !ok {
		t.Fatal("expected disk.used")
	}
	if _, ok := disk["total"]; !ok {
		t.Fatal("expected disk.total")
	}
	if _, ok := disk["percent"]; !ok {
		t.Fatal("expected disk.percent")
	}
}
