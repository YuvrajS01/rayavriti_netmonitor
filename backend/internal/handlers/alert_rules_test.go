package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestAlertRuleList(t *testing.T) {
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return []models.AlertRule{{ID: 1, Name: "High Latency", Severity: "warning"}}, nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alert-rules", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleList_DBError(t *testing.T) {
	db := &mockDB{
		getAlertRulesFn: func(ctx context.Context) ([]models.AlertRule, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alert-rules", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertRuleGet(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			if id == 1 {
				return &models.AlertRule{ID: 1, Name: "High Latency"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alert-rules/1", "")
	req = withChiParams(req, "id", "1")
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alert-rules/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleGet_NotFound(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/alert-rules/999", "")
	req = withChiParams(req, "id", "999")
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlertRuleCreate(t *testing.T) {
	db := &mockDB{
		createAlertRuleFn: func(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
			if r.Name == "" {
				t.Fatal("expected name to be set")
			}
			r.ID = 1
			return r, nil
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "High Latency"})
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules", string(body))
	callWithAuth(h.Create, w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAlertRuleCreate_MissingName(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"severity": "warning"})
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules", string(body))
	callWithAuth(h.Create, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleCreate_DBError(t *testing.T) {
	db := &mockDB{
		createAlertRuleFn: func(ctx context.Context, r *models.AlertRule) (*models.AlertRule, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Rule"})
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules", string(body))
	callWithAuth(h.Create, w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertRuleUpdate(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1}, nil
		},
		updateAlertRuleFn: func(ctx context.Context, id int64, r *models.AlertRule) (*models.AlertRule, error) {
			return r, nil
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := authenticatedRequest("PUT", "/api/v1/alert-rules/1", string(body))
	req = withChiParams(req, "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleUpdate_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("PUT", "/api/v1/alert-rules/abc", `{}`)
	req = withChiParams(req, "id", "abc")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleUpdate_NotFound(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertRuleHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "X"})
	w, req := authenticatedRequest("PUT", "/api/v1/alert-rules/999", string(body))
	req = withChiParams(req, "id", "999")
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlertRuleDelete(t *testing.T) {
	db := &mockDB{
		deleteAlertRuleFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/alert-rules/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/alert-rules/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteAlertRuleFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/alert-rules/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAlertRuleToggle(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1, Enabled: true}, nil
		},
		toggleAlertRuleFn: func(ctx context.Context, id int64, enabled bool) error {
			if enabled {
				t.Fatal("expected enabled=false (toggle from true)")
			}
			return nil
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/toggle", "")
	req = withChiParams(req, "id", "1")
	h.Toggle(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAlertRuleToggle_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/abc/toggle", "")
	req = withChiParams(req, "id", "abc")
	h.Toggle(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAlertRuleToggle_NotFound(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/999/toggle", "")
	req = withChiParams(req, "id", "999")
	h.Toggle(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAlertRuleToggle_DBError(t *testing.T) {
	db := &mockDB{
		getAlertRuleFn: func(ctx context.Context, id int64) (*models.AlertRule, error) {
			return &models.AlertRule{ID: 1, Enabled: false}, nil
		},
		toggleAlertRuleFn: func(ctx context.Context, id int64, enabled bool) error {
			return errors.New("db error")
		},
	}
	h := NewAlertRuleHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/alert-rules/1/toggle", "")
	req = withChiParams(req, "id", "1")
	h.Toggle(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
