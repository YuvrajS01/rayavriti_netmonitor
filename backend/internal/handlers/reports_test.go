package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestReportSummary(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return map[string]any{"avg": 50.0}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{"totalDevices": 10}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary", "")
	h.Summary(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportSummary_WithDeviceID(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			if deviceID == nil || *deviceID != 5 {
				t.Fatalf("expected deviceID 5, got %v", deviceID)
			}
			return map[string]any{}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary?deviceId=5", "")
	h.Summary(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportSummary_DBError(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary", "")
	h.Summary(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportTimeseries(t *testing.T) {
	db := &mockDB{
		getReportTimeseriesFn: func(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
			if bucketMinutes != 60 {
				t.Fatalf("expected default bucket 60, got %d", bucketMinutes)
			}
			return []models.ReportTimeseriesPoint{{BucketTime: "2026-01-01T00:00:00Z", AvgResponse: 42}}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/timeseries", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportTimeseries_CustomBucket(t *testing.T) {
	db := &mockDB{
		getReportTimeseriesFn: func(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
			if bucketMinutes != 30 {
				t.Fatalf("expected bucket 30, got %d", bucketMinutes)
			}
			return []models.ReportTimeseriesPoint{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/timeseries?bucket=30", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportTimeseries_DBError(t *testing.T) {
	db := &mockDB{
		getReportTimeseriesFn: func(ctx context.Context, from, to time.Time, bucketMinutes int, deviceID *int64) ([]models.ReportTimeseriesPoint, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/timeseries", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportDevices(t *testing.T) {
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			return []models.DeviceBreakdown{{DeviceID: 1, DeviceName: "Router"}}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices", "")
	h.Devices(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportDevices_DBError(t *testing.T) {
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices", "")
	h.Devices(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportAlerts(t *testing.T) {
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			return []models.Alert{{ID: 1, Severity: "warning"}}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts", "")
	h.Alerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReportAlerts_DBError(t *testing.T) {
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts", "")
	h.Alerts(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportExport(t *testing.T) {
	db := &mockDB{
		exportMetricsFn: func(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
			rt := 42.5
			return []models.Metric{
				{ID: 1, DeviceID: 1, DeviceName: "Router", Protocol: "ping", Status: "up", ResponseTime: &rt},
			}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/export", "")
	h.Export(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected Content-Type text/csv, got %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "id,device_id,device_name") {
		t.Fatal("expected CSV headers in response")
	}
	if !strings.Contains(body, "42.5") {
		t.Fatal("expected metric value in CSV")
	}
}

func TestReportExport_DBError(t *testing.T) {
	db := &mockDB{
		exportMetricsFn: func(ctx context.Context, from, to time.Time, deviceID *int64, limit int) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/export", "")
	h.Export(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReportList(t *testing.T) {
	db := &mockDB{}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestParseDeviceID(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/reports?deviceId=42", nil)
	id := parseDeviceID(r)
	if id == nil || *id != 42 {
		t.Fatalf("expected 42, got %v", id)
	}

	r = httptest.NewRequest("GET", "/api/v1/reports", nil)
	id = parseDeviceID(r)
	if id != nil {
		t.Fatalf("expected nil, got %v", id)
	}

	r = httptest.NewRequest("GET", "/api/v1/reports?deviceId=abc", nil)
	id = parseDeviceID(r)
	if id != nil {
		t.Fatalf("expected nil for invalid, got %v", id)
	}
}

func TestFloatPtr(t *testing.T) {
	v := 42.5
	result := floatPtr(&v)
	if result != "42.5" {
		t.Fatalf("expected '42.5', got '%s'", result)
	}
	result = floatPtr(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil, got '%s'", result)
	}
}
