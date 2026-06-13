package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestAlertList(t *testing.T) {
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return []models.Alert{{ID: 1, Severity: "critical", Status: "active"}}, 1, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertList_WithStatusFilter(t *testing.T) {
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			if status != "active" {
				t.Fatalf("expected status=active, got %s", status)
			}
			return []models.Alert{}, 0, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts?status=active", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertList_DBError(t *testing.T) {
	db := &mockDB{
		getAlertsFn: func(ctx context.Context, status string, limit, offset int) ([]models.Alert, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertGet_Valid(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			if id == 1 {
				return &models.Alert{ID: 1, Severity: "warning"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/1", "", "id", "1")
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertGet_NotFound(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/999", "", "id", "999")
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlertGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/abc", "", "id", "abc")
	h.Get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertCreate(t *testing.T) {
	db := &mockDB{
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			if a.Status != "active" {
				t.Fatalf("expected default status active, got %s", a.Status)
			}
			a.ID = 1
			return a, nil
		},
	}
	h := NewAlertHandler(db)
	body, _ := json.Marshal(map[string]any{"severity": "warning", "message": "test"})
	w, req := authenticatedRequest("POST", "/api/v1/alerts", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertCreate_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alerts", "not-json")
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertCreate_DBError(t *testing.T) {
	db := &mockDB{
		createAlertFn: func(ctx context.Context, a *models.Alert) (*models.Alert, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	body, _ := json.Marshal(map[string]any{"severity": "warning", "message": "test"})
	w, req := authenticatedRequest("POST", "/api/v1/alerts", string(body))
	h.Create(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertAcknowledge(t *testing.T) {
	db := &mockDB{
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			if status != "acknowledged" {
				t.Fatalf("expected status acknowledged, got %s", status)
			}
			return nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/1/acknowledge", "", "id", "1")
	h.Acknowledge(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertAcknowledge_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/abc/acknowledge", "", "id", "abc")
	h.Acknowledge(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertAcknowledge_DBError(t *testing.T) {
	db := &mockDB{
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/1/acknowledge", "", "id", "1")
	h.Acknowledge(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertResolve(t *testing.T) {
	db := &mockDB{
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			if status != "resolved" {
				t.Fatalf("expected status resolved, got %s", status)
			}
			return nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/1/resolve", "", "id", "1")
	h.Resolve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertResolve_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("POST", "/api/v1/alerts/abc/resolve", "", "id", "abc")
	h.Resolve(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertDelete(t *testing.T) {
	db := &mockDB{
		deleteAlertFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/alerts/1", "", "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/alerts/abc", "", "id", "abc")
	h.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteAlertFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("DELETE", "/api/v1/alerts/1", "", "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertCounts(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{Active: 3, Acknowledged: 1, Resolved: 5}, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/counts", "")
	h.Counts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertCounts_DBError(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{}, errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/counts", "")
	h.Counts(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertHistory(t *testing.T) {
	db := &mockDB{
		getAlertHistoryFn: func(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
			return []models.AlertHistory{{ID: 1, AlertID: alertID, Action: "fired"}}, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/1/history", "", "id", "1")
	h.History(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertHistory_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/abc/history", "", "id", "abc")
	h.History(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertHistory_DBError(t *testing.T) {
	db := &mockDB{
		getAlertHistoryFn: func(ctx context.Context, alertID int64) ([]models.AlertHistory, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("GET", "/api/v1/alerts/1/history", "", "id", "1")
	h.History(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertStats(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{Active: 2}, nil
		},
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{ID: 1, Name: "High Latency", Severity: "warning", Enabled: true}}, nil
		},
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{{ID: 1, Name: "Slack", Type: "slack", Enabled: true}}, nil
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/stats", "")
	h.AlertStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	if data["totalRules"] != float64(1) {
		t.Fatalf("expected 1 rule, got %v", data["totalRules"])
	}
	if data["totalChannels"] != float64(1) {
		t.Fatalf("expected 1 channel, got %v", data["totalChannels"])
	}
}

func TestAlertStats_CountsError(t *testing.T) {
	db := &mockDB{
		getAlertCountsFn: func(ctx context.Context) (models.AlertCounts, error) {
			return models.AlertCounts{}, errors.New("db error")
		},
	}
	h := NewAlertHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alerts/stats", "")
	h.AlertStats(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertUpdate(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return &models.Alert{ID: 1, Severity: "info", Message: "old"}, nil
		},
		updateAlertStatusFn: func(ctx context.Context, id int64, status, by string) error {
			return nil
		},
	}
	h := NewAlertHandler(db)
	severity := "critical"
	body, _ := json.Marshal(map[string]any{"severity": severity})
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/1", string(body), "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertUpdate_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/abc", `{}`, "id", "abc")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertUpdate_NotFound(t *testing.T) {
	db := &mockDB{
		getAlertFn: func(ctx context.Context, id int64) (*models.Alert, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertHandler(db)
	w, req := makeRequestWithParams("PUT", "/api/v1/alerts/999", `{"severity":"critical"}`, "id", "999")
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
