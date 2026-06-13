package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func TestNotificationChannelList(t *testing.T) {
	db := &mockDB{
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return []models.NotificationChannel{{ID: 1, Name: "Slack", Type: "slack"}}, nil
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/notification-channels", "")
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNotificationChannelList_DBError(t *testing.T) {
	db := &mockDB{
		getNotificationChannelsFn: func(ctx context.Context) ([]models.NotificationChannel, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/notification-channels", "")
	h.List(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestNotificationChannelGet(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			if id == 1 {
				return &models.NotificationChannel{ID: 1, Name: "Slack"}, nil
			}
			return nil, errors.New("not found")
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/notification-channels/1", "")
	req = withChiParams(req, "id", "1")
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNotificationChannelGet_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/notification-channels/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Get(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelGet_NotFound(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("GET", "/api/v1/notification-channels/999", "")
	req = withChiParams(req, "id", "999")
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNotificationChannelCreate(t *testing.T) {
	db := &mockDB{
		createNotificationChannelFn: func(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
			ch.ID = 1
			return ch, nil
		},
	}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Slack", "type": "slack", "config": map[string]any{"webhook": "http://hook"}})
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels", string(body))
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotificationChannelCreate_MissingName(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"type": "slack"})
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelCreate_MissingType(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Slack"})
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels", string(body))
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelCreate_DBError(t *testing.T) {
	db := &mockDB{
		createNotificationChannelFn: func(ctx context.Context, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Slack", "type": "slack"})
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels", string(body))
	h.Create(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestNotificationChannelUpdate(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			return &models.NotificationChannel{ID: 1}, nil
		},
		updateNotificationChannelFn: func(ctx context.Context, id int64, ch *models.NotificationChannel) (*models.NotificationChannel, error) {
			return ch, nil
		},
	}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "Updated"})
	w, req := authenticatedRequest("PUT", "/api/v1/notification-channels/1", string(body))
	req = withChiParams(req, "id", "1")
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNotificationChannelUpdate_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("PUT", "/api/v1/notification-channels/abc", `{}`)
	req = withChiParams(req, "id", "abc")
	h.Update(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelUpdate_NotFound(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewNotificationChannelHandler(db)
	body, _ := json.Marshal(map[string]any{"name": "X"})
	w, req := authenticatedRequest("PUT", "/api/v1/notification-channels/999", string(body))
	req = withChiParams(req, "id", "999")
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestNotificationChannelDelete(t *testing.T) {
	db := &mockDB{
		deleteNotificationChannelFn: func(ctx context.Context, id int64) error { return nil },
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/notification-channels/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestNotificationChannelDelete_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/notification-channels/abc", "")
	req = withChiParams(req, "id", "abc")
	h.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelDelete_DBError(t *testing.T) {
	db := &mockDB{
		deleteNotificationChannelFn: func(ctx context.Context, id int64) error { return errors.New("db error") },
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("DELETE", "/api/v1/notification-channels/1", "")
	req = withChiParams(req, "id", "1")
	h.Delete(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestNotificationChannelTest_InvalidID(t *testing.T) {
	db := &mockDB{}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels/abc/test", "")
	req = withChiParams(req, "id", "abc")
	h.Test(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotificationChannelTest_NotFound(t *testing.T) {
	db := &mockDB{
		getNotificationChannelFn: func(ctx context.Context, id int64) (*models.NotificationChannel, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewNotificationChannelHandler(db)
	w, req := authenticatedRequest("POST", "/api/v1/notification-channels/999/test", "")
	req = withChiParams(req, "id", "999")
	h.Test(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
