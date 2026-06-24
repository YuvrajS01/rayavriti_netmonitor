package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// ── ReportHandler: Summary ────────────────────────────────────────────────────

func TestReportSummary_MergesDashboardStats(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return map[string]any{"avgResponseTime": 42.0}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{"totalDevices": 10, "totalAlerts": 3}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary", "")
	h.Summary(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["avgResponseTime"] != 42.0 {
		t.Fatalf("expected avgResponseTime 42, got %v", data["avgResponseTime"])
	}
	if data["totalDevices"] != float64(10) {
		t.Fatalf("expected totalDevices 10, got %v", data["totalDevices"])
	}
	if data["totalAlerts"] != float64(3) {
		t.Fatalf("expected totalAlerts 3, got %v", data["totalAlerts"])
	}
}

func TestReportSummary_DashboardStatsErrorDoesNotFail(t *testing.T) {
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			return map[string]any{"avg": 50.0}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return nil, errors.New("stats error")
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary", "")
	h.Summary(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful degradation), got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["avg"] != 50.0 {
		t.Fatalf("expected avg 50.0, got %v", data["avg"])
	}
}

func TestReportSummary_NilDeviceID(t *testing.T) {
	var receivedDeviceID *int64
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			receivedDeviceID = deviceID
			return map[string]any{}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary", "")
	h.Summary(w, req)
	if receivedDeviceID != nil {
		t.Fatalf("expected nil deviceID, got %v", receivedDeviceID)
	}
}

func TestReportSummary_WithTimeRange(t *testing.T) {
	var receivedFrom, receivedTo time.Time
	db := &mockDB{
		getMetricsSummaryFn: func(ctx context.Context, from, to time.Time, deviceID *int64) (map[string]any, error) {
			receivedFrom = from
			receivedTo = to
			return map[string]any{}, nil
		},
		getDashboardStatsFn: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/summary?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", "")
	h.Summary(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedFrom.IsZero() || receivedTo.IsZero() {
		t.Fatal("expected non-zero time values")
	}
}

// ── ReportHandler: Devices ────────────────────────────────────────────────────

func TestReportDevices_WithDeviceBreakdown(t *testing.T) {
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			return []models.DeviceBreakdown{
				{DeviceID: 1, DeviceName: "Router1", Protocol: "ping", AvgResponse: 45.2, SampleCount: 100},
				{DeviceID: 2, DeviceName: "Switch1", Protocol: "snmp", AvgResponse: 12.1, SampleCount: 50},
			}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices", "")
	h.Devices(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(data))
	}
	first := data[0].(map[string]any)
	if first["deviceName"] != "Router1" {
		t.Fatalf("expected Router1, got %v", first["deviceName"])
	}
}

func TestReportDevices_EmptyResult(t *testing.T) {
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			return []models.DeviceBreakdown{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices", "")
	h.Devices(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(data))
	}
}

func TestReportDevices_WithDeviceIDFilter(t *testing.T) {
	var receivedDeviceID *int64
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			receivedDeviceID = deviceID
			return []models.DeviceBreakdown{{DeviceID: 5, DeviceName: "FilteredDevice"}}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices?deviceId=5", "")
	h.Devices(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedDeviceID == nil || *receivedDeviceID != 5 {
		t.Fatalf("expected deviceID 5, got %v", receivedDeviceID)
	}
}

func TestReportDevices_NilDeviceID(t *testing.T) {
	var receivedDeviceID *int64
	db := &mockDB{
		getReportDeviceBreakdownFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.DeviceBreakdown, error) {
			receivedDeviceID = deviceID
			return []models.DeviceBreakdown{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/devices", "")
	h.Devices(w, req)
	if receivedDeviceID != nil {
		t.Fatalf("expected nil deviceID, got %v", receivedDeviceID)
	}
}

// ── ReportHandler: Alerts ─────────────────────────────────────────────────────

func TestReportAlerts_WithMultipleAlerts(t *testing.T) {
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			return []models.Alert{
				{ID: 1, Severity: "critical", Message: "CPU high", Status: "active"},
				{ID: 2, Severity: "warning", Message: "Memory high", Status: "acknowledged"},
				{ID: 3, Severity: "info", Message: "Config change", Status: "resolved"},
			}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts", "")
	h.Alerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("expected 3 alerts, got %d", len(data))
	}
}

func TestReportAlerts_EmptyResult(t *testing.T) {
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			return []models.Alert{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts", "")
	h.Alerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("expected 0 alerts, got %d", len(data))
	}
}

func TestReportAlerts_WithDeviceIDFilter(t *testing.T) {
	var receivedDeviceID *int64
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			receivedDeviceID = deviceID
			return []models.Alert{{ID: 1, Severity: "warning"}}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts?deviceId=3", "")
	h.Alerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedDeviceID == nil || *receivedDeviceID != 3 {
		t.Fatalf("expected deviceID 3, got %v", receivedDeviceID)
	}
}

func TestReportAlerts_NilDeviceID(t *testing.T) {
	var receivedDeviceID *int64
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			receivedDeviceID = deviceID
			return []models.Alert{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts", "")
	h.Alerts(w, req)
	if receivedDeviceID != nil {
		t.Fatalf("expected nil deviceID, got %v", receivedDeviceID)
	}
}

func TestReportAlerts_WithTimeRange(t *testing.T) {
	var receivedFrom, receivedTo time.Time
	db := &mockDB{
		getAlertsForReportFn: func(ctx context.Context, from, to time.Time, deviceID *int64) ([]models.Alert, error) {
			receivedFrom = from
			receivedTo = to
			return []models.Alert{}, nil
		},
	}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports/alerts?from=2026-01-01T00:00:00Z&to=2026-06-01T00:00:00Z", "")
	h.Alerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedFrom.IsZero() || receivedTo.IsZero() {
		t.Fatal("expected non-zero time values")
	}
}

// ── ReportHandler: List ───────────────────────────────────────────────────────

func TestReportList_ReturnsPredefinedReports(t *testing.T) {
	db := &mockDB{}
	h := NewReportHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/reports", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(data))
	}
	first := data[0].(map[string]any)
	if first["id"] != "availability" {
		t.Fatalf("expected availability, got %v", first["id"])
	}
}

// ── InsightHandler: Current ───────────────────────────────────────────────────

func TestInsightCurrent_WithIssues(t *testing.T) {
	issuesJSON := json.RawMessage(`[{"severity":"high","type":"cpu","message":"CPU above 90%"}]`)
	factorsJSON := json.RawMessage(`{"cpu":95.0,"memory":60.0}`)
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 15, Label: "critical", Trend: "degrading", TrendDelta: -12.5, Issues: issuesJSON, Factors: factorsJSON},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "CriticalServer"}}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["networkScore"] == nil {
		t.Fatal("expected networkScore in response")
	}
	topRisks := data["topRisks"].([]any)
	if len(topRisks) != 1 {
		t.Fatalf("expected 1 top risk, got %d", len(topRisks))
	}
	risk := topRisks[0].(map[string]any)
	if risk["deviceName"] != "CriticalServer" {
		t.Fatalf("expected CriticalServer, got %v", risk["deviceName"])
	}
}

func TestInsightCurrent_AllHealthy(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 95, Label: "healthy", Trend: "stable"},
				{DeviceID: 2, Score: 88, Label: "healthy", Trend: "stable"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "Server1"},
				{ID: 2, Name: "Server2"},
			}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	dist := data["healthDistribution"].(map[string]any)
	if dist["healthy"] != float64(2) {
		t.Fatalf("expected 2 healthy, got %v", dist["healthy"])
	}
	if dist["critical"] != float64(0) {
		t.Fatalf("expected 0 critical, got %v", dist["critical"])
	}
	topRisksRaw := data["topRisks"]
	if topRisksRaw != nil {
		topRisks := topRisksRaw.([]any)
		if len(topRisks) != 0 {
			t.Fatalf("expected 0 top risks, got %d", len(topRisks))
		}
	}
}

func TestInsightCurrent_MixedLabels(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 95, Label: "healthy", Trend: "stable"},
				{DeviceID: 2, Score: 55, Label: "watch", Trend: "stable"},
				{DeviceID: 3, Score: 30, Label: "risk", Trend: "degrading"},
				{DeviceID: 4, Score: 10, Label: "critical", Trend: "degrading"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "S1"}, {ID: 2, Name: "S2"},
				{ID: 3, Name: "S3"}, {ID: 4, Name: "S4"},
			}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	dist := data["healthDistribution"].(map[string]any)
	if dist["healthy"] != float64(1) {
		t.Fatalf("expected 1 healthy, got %v", dist["healthy"])
	}
	if dist["watch"] != float64(1) {
		t.Fatalf("expected 1 watch, got %v", dist["watch"])
	}
	if dist["risk"] != float64(1) {
		t.Fatalf("expected 1 risk, got %v", dist["risk"])
	}
	if dist["critical"] != float64(1) {
		t.Fatalf("expected 1 critical, got %v", dist["critical"])
	}
	topRisks := data["topRisks"].([]any)
	if len(topRisks) != 2 {
		t.Fatalf("expected 2 top risks (risk + critical), got %d", len(topRisks))
	}
}

func TestInsightCurrent_TopRisksCappedAt5(t *testing.T) {
	scores := make([]models.DeviceHealthScoreRow, 8)
	for i := range scores {
		scores[i] = models.DeviceHealthScoreRow{
			DeviceID: int64(i + 1),
			Score:    float64(10 + i),
			Label:    "critical",
			Trend:    "degrading",
		}
	}
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return scores, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			devices := make([]models.Device, 8)
			for i := range devices {
				devices[i] = models.Device{ID: int64(i + 1), Name: "Dev" + string(rune('A'+i))}
			}
			return devices, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	topRisks := data["topRisks"].([]any)
	if len(topRisks) != 5 {
		t.Fatalf("expected 5 top risks (capped), got %d", len(topRisks))
	}
}

func TestInsightCurrent_NetworkScoreCalculation(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 80, Label: "healthy"},
				{DeviceID: 2, Score: 60, Label: "watch"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{
				{ID: 1, Name: "S1"},
				{ID: 2, Name: "S2"},
			}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	networkScore := int(data["networkScore"].(float64))
	if networkScore != 70 {
		t.Fatalf("expected networkScore 70, got %d", networkScore)
	}
}

func TestInsightCurrent_DevicesError(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 50, Label: "watch"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestInsightCurrent_UnknownLabel(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 50, Label: "unknown_label"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "D1"}}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	dist := data["healthDistribution"].(map[string]any)
	if dist["healthy"] != float64(1) {
		t.Fatalf("unknown label should count as healthy, got %v", dist["healthy"])
	}
}

func TestInsightCurrent_DeviceNameNotFound(t *testing.T) {
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 99, Score: 50, Label: "watch"},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "DifferentDevice"}}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	health := data["health"].([]any)
	if len(health) != 1 {
		t.Fatalf("expected 1 health entry, got %d", len(health))
	}
	entry := health[0].(map[string]any)
	if entry["deviceName"] != "" {
		t.Fatalf("expected empty device name for unknown ID, got %v", entry["deviceName"])
	}
}

func TestInsightCurrent_WithIssuesAndFactors(t *testing.T) {
	issuesJSON := json.RawMessage(`[{"severity":"critical","type":"latency","message":"High latency detected"}]`)
	factorsJSON := json.RawMessage(`{"availability": 85.5}`)
	db := &mockDB{
		getHealthScoresFn: func(ctx context.Context) ([]models.DeviceHealthScoreRow, error) {
			return []models.DeviceHealthScoreRow{
				{DeviceID: 1, Score: 25, Label: "risk", Trend: "degrading", TrendDelta: -8.0, Factors: factorsJSON, Issues: issuesJSON},
			}, nil
		},
		getDevicesFn: func(ctx context.Context) ([]models.Device, error) {
			return []models.Device{{ID: 1, Name: "RiskDevice"}}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/current", "")
	h.Current(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	insights := data["insights"].([]any)
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(insights))
	}
	insight := insights[0].(map[string]any)
	if insight["title"] != "RiskDevice — 25%" {
		t.Fatalf("unexpected title: %v", insight["title"])
	}
	if insight["message"] != "High latency detected" {
		t.Fatalf("unexpected message: %v", insight["message"])
	}
}

// ── InsightHandler: History ───────────────────────────────────────────────────

func TestInsightHistory_WithPoints(t *testing.T) {
	db := &mockDB{
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			return []models.HealthHistoryPoint{
				{Timestamp: "2026-01-01T00:00:00Z", Score: float64Ptr(85.0)},
				{Timestamp: "2026-01-01T01:00:00Z", Score: float64Ptr(82.5)},
			}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	points := data["points"].([]any)
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
}

func TestInsightHistory_CustomHours(t *testing.T) {
	var receivedHours int
	db := &mockDB{
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			receivedHours = hours
			return []models.HealthHistoryPoint{}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history?hours=48", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedHours != 48 {
		t.Fatalf("expected 48 hours, got %d", receivedHours)
	}
}

func TestInsightHistory_DefaultHours(t *testing.T) {
	var receivedHours int
	db := &mockDB{
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			receivedHours = hours
			return []models.HealthHistoryPoint{}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedHours != 12 {
		t.Fatalf("expected default 12 hours, got %d", receivedHours)
	}
}

func TestInsightHistory_InvalidHoursIgnored(t *testing.T) {
	var receivedHours int
	db := &mockDB{
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			receivedHours = hours
			return []models.HealthHistoryPoint{}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history?hours=abc", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedHours != 12 {
		t.Fatalf("expected default 12 for invalid hours, got %d", receivedHours)
	}
}

func TestInsightHistory_NegativeHoursIgnored(t *testing.T) {
	var receivedHours int
	db := &mockDB{
		getNetworkHealthHistoryFn: func(ctx context.Context, hours int) ([]models.HealthHistoryPoint, error) {
			receivedHours = hours
			return []models.HealthHistoryPoint{}, nil
		},
	}
	h := NewInsightHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/insights/history?hours=-5", "")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedHours != 12 {
		t.Fatalf("expected default 12 for negative hours, got %d", receivedHours)
	}
}

// ── DashboardHandler: List ────────────────────────────────────────────────────

func TestDashboardList_MultipleDashboards(t *testing.T) {
	db := &mockDB{
		getDashboardsFn: func(ctx context.Context, userID int64) ([]models.Dashboard, error) {
			return []models.Dashboard{
				{ID: 1, Name: "Overview", UserID: userID},
				{ID: 2, Name: "Performance", UserID: userID},
				{ID: 3, Name: "Security", UserID: userID},
			}, nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards", "")
	callWithAuth(h.List, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 3 {
		t.Fatalf("expected 3 dashboards, got %d", len(data))
	}
	first := data[0].(map[string]any)
	if first["name"] != "Overview" {
		t.Fatalf("expected Overview, got %v", first["name"])
	}
}

func TestDashboardList_EmptyList(t *testing.T) {
	db := &mockDB{
		getDashboardsFn: func(ctx context.Context, userID int64) ([]models.Dashboard, error) {
			return []models.Dashboard{}, nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards", "")
	callWithAuth(h.List, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]any)
	if len(data) != 0 {
		t.Fatalf("expected 0 dashboards, got %d", len(data))
	}
}

func TestDashboardList_CorrectUserID(t *testing.T) {
	var receivedUserID int64
	db := &mockDB{
		getDashboardsFn: func(ctx context.Context, userID int64) ([]models.Dashboard, error) {
			receivedUserID = userID
			return []models.Dashboard{}, nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards", "")
	callWithAuth(h.List, w, req)
	if receivedUserID != testUserID {
		t.Fatalf("expected userID %d, got %d", testUserID, receivedUserID)
	}
}

// ── DashboardHandler: Get ─────────────────────────────────────────────────────

func TestDashboardGet_ReturnsDashboard(t *testing.T) {
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			return &models.Dashboard{ID: 42, Name: "My Dashboard", UserID: testUserID}, nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards/42", "")
	callWithAuthAndParams(h.Get, w, req, "id", "42")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["name"] != "My Dashboard" {
		t.Fatalf("expected My Dashboard, got %v", data["name"])
	}
}

// ── DashboardHandler: Save ────────────────────────────────────────────────────

func TestDashboardSave_SetsUserID(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			if d.UserID != testUserID {
				t.Fatalf("expected userID %d, got %d", testUserID, d.UserID)
			}
			d.ID = 1
			return d, nil
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "New", "layout": []any{}})
	w, req := authenticatedRequest("POST", "/api/v1/dashboards", string(body))
	callWithAuth(h.Save, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboardSave_UpdateSetsIDFromURL(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			if d.ID != 7 {
				t.Fatalf("expected ID 7 from URL, got %d", d.ID)
			}
			return d, nil
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := authenticatedRequest("PUT", "/api/v1/dashboards/7", string(body))
	callWithAuthAndParams(h.Save, w, req, "id", "7")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardSave_NilLayout(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			d.ID = 1
			return d, nil
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "NoLayout"})
	w, req := authenticatedRequest("POST", "/api/v1/dashboards", string(body))
	callWithAuth(h.Save, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── DashboardHandler: Delete ──────────────────────────────────────────────────

func TestDashboardDelete_ResponseContainsMessage(t *testing.T) {
	db := &mockDB{
		deleteDashboardFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/dashboards/1", "")
	callWithAuthAndParams(h.Delete, w, req, "id", "1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["message"] != "deleted" {
		t.Fatalf("expected 'deleted' message, got %v", data["message"])
	}
}
