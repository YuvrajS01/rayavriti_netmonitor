package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

func guardRequest(callerID int64, callerRole string, targetID, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+targetID, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", targetID)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, auth.ClaimsKey, &auth.Claims{UserID: callerID, Role: callerRole})
	return r.WithContext(ctx)
}

func TestGuardUserUpdate(t *testing.T) {
	roles := map[int64]string{1: "super_admin", 2: "network_admin", 3: "viewer", 4: "admin"}
	db := &mockDB{getUserByIDFn: func(ctx context.Context, id int64) (*models.User, error) {
		return &models.User{ID: id, Role: roles[id], Enabled: true}, nil
	}}
	h := NewAuthHandler(db, testConfig())

	tests := []struct {
		name       string
		callerID   int64
		callerRole string
		target     string
		body       string
		wantStatus int
	}{
		{"self-escalation to super_admin blocked", 2, "network_admin", "2", `{"role":"super_admin"}`, http.StatusForbidden},
		{"grant higher role than own blocked", 2, "network_admin", "3", `{"role":"admin"}`, http.StatusForbidden},
		{"grant equal role allowed", 2, "network_admin", "3", `{"role":"network_admin"}`, http.StatusOK},
		{"modify higher-ranked user blocked", 2, "network_admin", "4", `{"display_name":"x"}`, http.StatusForbidden},
		{"disable super_admin blocked", 4, "admin", "1", `{"enabled":false}`, http.StatusForbidden},
		{"self-disable blocked", 2, "network_admin", "2", `{"enabled":false}`, http.StatusForbidden},
		{"custom role by non-admin blocked", 2, "network_admin", "3", `{"roleId":7}`, http.StatusForbidden},
		{"custom role by admin allowed", 4, "admin", "3", `{"role_id":7}`, http.StatusOK},
		{"profile edit allowed", 2, "network_admin", "3", `{"display_name":"Lab"}`, http.StatusOK},
		{"unknown role rejected", 4, "admin", "3", `{"role":"root"}`, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) }
			w := httptest.NewRecorder()
			h.GuardUserUpdate(next)(w, guardRequest(tt.callerID, tt.callerRole, tt.target, tt.body))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if called != (tt.wantStatus == http.StatusOK) {
				t.Fatalf("next called = %v, want %v", called, tt.wantStatus == http.StatusOK)
			}
		})
	}
}
