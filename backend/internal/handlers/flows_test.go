package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestFlowList(t *testing.T) {
	db := &mockDB{
		getFlowsFn: func(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
			return []models.Flow{
				{ID: 1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "TCP"},
			}, 1, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowList_DBError(t *testing.T) {
	db := &mockDB{
		getFlowsFn: func(ctx context.Context, from, to time.Time, limit, offset int) ([]models.Flow, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestFlowTopTalkers(t *testing.T) {
	db := &mockDB{
		getTopTalkersFn: func(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
			return []models.IPCount{{IP: "10.0.0.1", Count: 1000}}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/top-talkers", "")
	h.TopTalkers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowTopTalkers_DBError(t *testing.T) {
	db := &mockDB{
		getTopTalkersFn: func(ctx context.Context, from, to time.Time, n int) ([]models.IPCount, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/top-talkers", "")
	h.TopTalkers(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestFlowProtocols(t *testing.T) {
	db := &mockDB{
		getProtocolStatsFn: func(ctx context.Context, from, to time.Time) (map[string]int64, error) {
			return map[string]int64{"TCP": 500, "UDP": 200}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/protocols", "")
	h.Protocols(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowProtocols_DBError(t *testing.T) {
	db := &mockDB{
		getProtocolStatsFn: func(ctx context.Context, from, to time.Time) (map[string]int64, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/protocols", "")
	h.Protocols(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestFlowTimeseries(t *testing.T) {
	db := &mockDB{
		getFlowTimeseriesFn: func(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
			return []models.FlowTimeseriesPoint{{BucketTime: "2026-01-01T00:00:00Z", TotalBytes: 1024}}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/timeseries?interval=5m", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowTimeseries_DefaultInterval(t *testing.T) {
	db := &mockDB{
		getFlowTimeseriesFn: func(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
			if interval != "5m" {
				t.Fatalf("expected default interval 5m, got %s", interval)
			}
			return []models.FlowTimeseriesPoint{}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/timeseries", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowTimeseries_DBError(t *testing.T) {
	db := &mockDB{
		getFlowTimeseriesFn: func(ctx context.Context, from, to time.Time, interval string) ([]models.FlowTimeseriesPoint, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/timeseries", "")
	h.Timeseries(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestFlowStats(t *testing.T) {
	db := &mockDB{
		getFlowStatsFn: func(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
			return models.FlowSummaryStats{TotalFlows: 100, TotalBytes: 50000}, nil
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/stats", "")
	h.Stats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestFlowStats_DBError(t *testing.T) {
	db := &mockDB{
		getFlowStatsFn: func(ctx context.Context, from, to time.Time) (models.FlowSummaryStats, error) {
			return models.FlowSummaryStats{}, errors.New("db error")
		},
	}
	h := NewFlowHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/flows/stats", "")
	h.Stats(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
