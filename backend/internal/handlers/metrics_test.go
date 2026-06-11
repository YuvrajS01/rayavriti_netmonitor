package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestMetricLatest(t *testing.T) {
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			rt := 42.5
			return []models.Metric{{ID: 1, DeviceID: 1, ResponseTime: &rt}}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/latest", "")
	h.Latest(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricLatest_DBError(t *testing.T) {
	db := &mockDB{
		getLatestMetricsFn: func(ctx context.Context) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/latest", "")
	h.Latest(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestMetricForDevice_Valid(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			if deviceID != 1 {
				t.Fatalf("expected deviceID 1, got %d", deviceID)
			}
			return []models.Metric{{ID: 1, DeviceID: 1}}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/metrics/device/1", "", "deviceId", "1")
	h.ForDevice(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricForDevice_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewMetricHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/metrics/device/abc", "", "deviceId", "abc")
	h.ForDevice(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMetricForDevice_DBError(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewMetricHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/metrics/device/1", "", "deviceId", "1")
	h.ForDevice(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestMetricQuery_WithDeviceID(t *testing.T) {
	db := &mockDB{
		getDeviceMetricsFn: func(ctx context.Context, deviceID int64, from, to time.Time, limit int) ([]models.Metric, error) {
			return []models.Metric{{ID: 1, DeviceID: deviceID}}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?deviceId=1", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_Summary(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return map[string]any{"avg": 42.0}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_WithAggregation(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.Aggregation != "avg" {
				t.Fatalf("expected aggregation avg, got %s", q.Aggregation)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?aggregation=avg", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_WithBucketMin(t *testing.T) {
	db := &mockDB{
		queryMetricsFn: func(ctx context.Context, q models.MetricQuery) ([]models.Metric, error) {
			if q.BucketMin != 5 {
				t.Fatalf("expected bucketMin 5, got %d", q.BucketMin)
			}
			return []models.Metric{}, nil
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query?bucketMin=5", "")
	h.Query(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMetricQuery_SummaryDBError(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewMetricHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/metrics/query", "")
	h.Query(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestParseTimeRange_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	from, to, limit := parseTimeRange(r)
	if limit != 0 {
		t.Fatalf("expected default limit 0, got %d", limit)
	}
	if !to.After(from) {
		t.Fatal("expected to > from")
	}
	dur := to.Sub(from)
	if dur < 23*time.Hour || dur > 25*time.Hour {
		t.Fatalf("expected ~24h range, got %v", dur)
	}
}

func TestParseTimeRange_Custom(t *testing.T) {
	fromTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	r := httptest.NewRequest("GET", "/api/v1/metrics?from="+fromTime.Format(time.RFC3339)+"&to="+toTime.Format(time.RFC3339)+"&limit=100", nil)
	from, to, limit := parseTimeRange(r)
	if !from.Equal(fromTime) {
		t.Fatalf("expected from %v, got %v", fromTime, from)
	}
	if !to.Equal(toTime) {
		t.Fatalf("expected to %v, got %v", toTime, to)
	}
	if limit != 100 {
		t.Fatalf("expected limit 100, got %d", limit)
	}
}

func TestParseTimeRange_InvalidFrom(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/metrics?from=not-a-date", nil)
	from, to, _ := parseTimeRange(r)
	dur := to.Sub(from)
	if dur < 23*time.Hour || dur > 25*time.Hour {
		t.Fatalf("invalid from should fallback to 24h, got %v", dur)
	}
}

func TestParseTimeRange_Limit(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/metrics?limit=50", nil)
	_, _, limit := parseTimeRange(r)
	if limit != 50 {
		t.Fatalf("expected limit 50, got %d", limit)
	}
}
