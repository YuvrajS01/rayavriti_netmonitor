package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestDashboardList(t *testing.T) {
	db := &mockDB{
		getDashboardsFn: func(ctx context.Context, userID int64) ([]models.Dashboard, error) {
			if userID != testUserID {
				t.Fatalf("expected userID %d, got %d", testUserID, userID)
			}
			return []models.Dashboard{{ID: 1, Name: "Main", UserID: userID}}, nil
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards", "")
	callWithAuth(h.List, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardList_DBError(t *testing.T) {
	db := &mockDB{
		getDashboardsFn: func(ctx context.Context, userID int64) ([]models.Dashboard, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards", "")
	callWithAuth(h.List, w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDashboardGet_Valid(t *testing.T) {
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			if id == 1 {
				return &models.Dashboard{ID: 1, Name: "Main"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards/1", "")
	callWithAuthAndParams(h.Get, w, req, "id", "1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardGet_NotFound(t *testing.T) {
	db := &mockDB{
		getDashboardFn: func(ctx context.Context, id int64) (*models.Dashboard, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards/999", "")
	callWithAuthAndParams(h.Get, w, req, "id", "999")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDashboardGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/dashboards/abc", "")
	callWithAuthAndParams(h.Get, w, req, "id", "abc")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDashboardSave_Create(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			if d.UserID != testUserID {
				t.Fatalf("expected userID %d, got %d", testUserID, d.UserID)
			}
			d.ID = 10
			return d, nil
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "New Dashboard", "layout": []any{}})
	w, req := authenticatedRequest("POST", "/api/v1/dashboards", string(body))
	callWithAuth(h.Save, w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboardSave_Update(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			if d.ID != 5 {
				t.Fatalf("expected ID 5, got %d", d.ID)
			}
			return d, nil
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := authenticatedRequest("PUT", "/api/v1/dashboards/5", string(body))
	callWithAuthAndParams(h.Save, w, req, "id", "5")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardSave_InvalidBody(t *testing.T) {
	db := &mockDB{}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/dashboards", "not-json")
	callWithAuth(h.Save, w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDashboardSave_DBError(t *testing.T) {
	db := &mockDB{
		saveDashboardFn: func(ctx context.Context, d *models.Dashboard) (*models.Dashboard, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewDashboardHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Dashboard"})
	w, req := authenticatedRequest("POST", "/api/v1/dashboards", string(body))
	callWithAuth(h.Save, w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDashboardDelete(t *testing.T) {
	db := &mockDB{
		deleteDashboardFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/dashboards/1", "")
	callWithAuthAndParams(h.Delete, w, req, "id", "1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/dashboards/abc", "")
	callWithAuthAndParams(h.Delete, w, req, "id", "abc")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDashboardDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteDashboardFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewDashboardHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/dashboards/1", "")
	callWithAuthAndParams(h.Delete, w, req, "id", "1")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
