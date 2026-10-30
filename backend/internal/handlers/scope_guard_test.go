package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

func ptr(v int64) *int64 { return &v }

// scopedRequest builds a request with chi URL params and, unless scopes is
// nil, a scope context for a location-scoped user.
func scopedRequest(method, path, body string, scopes []rbac.UserScope, params ...string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(params); i += 2 {
		rctx.URLParams.Add(params[i], params[i+1])
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	if scopes != nil {
		r = rbac.WithScopeContext(r, &rbac.ScopeContext{UserID: 9, Role: "viewer", IsScoped: true, Scopes: scopes})
	}
	return r
}

var buildingOne = []rbac.UserScope{{Type: "location", Value: "1"}}

func TestDeviceScopeGuard(t *testing.T) {
	devices := map[int64]*models.Device{
		10: {ID: 10, LocationID: ptr(1), IPAddress: "10.0.0.10"},
		20: {ID: 20, LocationID: ptr(2), IPAddress: "10.0.0.20"},
	}
	db := &mockDB{getDeviceFn: func(_ context.Context, id int64) (*models.Device, error) {
		return devices[id], nil
	}}
	guard := DeviceScopeGuard(db, "id")
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	tests := []struct {
		name   string
		id     string
		scopes []rbac.UserScope
		want   int
	}{
		{"in-scope device", "10", buildingOne, http.StatusOK},
		{"out-of-scope device looks missing", "20", buildingOne, http.StatusNotFound},
		{"unknown device", "99", buildingOne, http.StatusNotFound},
		{"unscoped user sees everything", "20", nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			guard(ok).ServeHTTP(w, scopedRequest("GET", "/api/v1/devices/"+tt.id, "", tt.scopes, "id", tt.id))
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestRequireUnscoped(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	w := httptest.NewRecorder()
	RequireUnscoped(ok).ServeHTTP(w, scopedRequest("GET", "/api/v1/flows", "", buildingOne))
	if w.Code != http.StatusForbidden {
		t.Fatalf("scoped user: status = %d, want 403", w.Code)
	}
	w = httptest.NewRecorder()
	RequireUnscoped(ok).ServeHTTP(w, scopedRequest("GET", "/api/v1/flows", "", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("unscoped user: status = %d, want 200", w.Code)
	}
}

func TestDeviceUpdate_PartialBodyKeepsPlacement(t *testing.T) {
	var saved *models.Device
	db := &mockDB{
		getDeviceFn: func(_ context.Context, id int64) (*models.Device, error) {
			return &models.Device{ID: id, Name: "sw", IPAddress: "10.0.0.1", Protocol: "ping", LocationID: ptr(1)}, nil
		},
		updateDeviceFn: func(_ context.Context, _ int64, d *models.Device) (*models.Device, error) {
			saved = d
			return d, nil
		},
	}
	h := NewDeviceHandler(db)
	w := httptest.NewRecorder()
	h.Update(w, scopedRequest("PUT", "/api/v1/devices/5/parent", `{"parentDeviceId":7}`, nil, "id", "5"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if saved.LocationID == nil || *saved.LocationID != 1 {
		t.Fatalf("location was cleared by a parent-only update: %v", saved.LocationID)
	}
	if saved.ParentDeviceID == nil || *saved.ParentDeviceID != 7 {
		t.Fatalf("parent not applied: %v", saved.ParentDeviceID)
	}
}

func TestDeviceUpdate_ScopedUserCannotMoveDeviceOutOfScope(t *testing.T) {
	db := &mockDB{getDeviceFn: func(_ context.Context, id int64) (*models.Device, error) {
		return &models.Device{ID: id, Name: "sw", IPAddress: "10.0.0.1", Protocol: "ping", LocationID: ptr(1)}, nil
	}}
	h := NewDeviceHandler(db)
	w := httptest.NewRecorder()
	h.Update(w, scopedRequest("PUT", "/api/v1/devices/5", `{"locationId":2}`, buildingOne, "id", "5"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}
