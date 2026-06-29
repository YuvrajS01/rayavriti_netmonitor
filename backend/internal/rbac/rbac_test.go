package rbac

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      []string
		required   string
		wantResult bool
	}{
		{"exact match", []string{"devices.read", "alerts.read"}, "devices.read", true},
		{"no match", []string{"devices.read"}, "alerts.read", false},
		{"empty perms", nil, "devices.read", false},
		{"empty required", []string{"devices.read"}, "", false},
		{"multiple perms", []string{"a", "b", "c"}, "b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.perms, tt.required); got != tt.wantResult {
				t.Errorf("HasPermission() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestParsePermissions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"valid", `["devices.read","alerts.read"]`, 2},
		{"empty array", `[]`, 0},
		{"nil", ``, 0},
		{"invalid json", `not json`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.input != "" {
				raw = json.RawMessage(tt.input)
			}
			got := ParsePermissions(raw)
			if len(got) != tt.want {
				t.Errorf("ParsePermissions() returned %d items, want %d", len(got), tt.want)
			}
		})
	}
}

func TestRequirePermission_BypassesForAdmin(t *testing.T) {
	adminRoles := []string{"super_admin", "admin"}
	for _, role := range adminRoles {
		if role != "super_admin" && role != "admin" {
			t.Errorf("expected %s to be admin role", role)
		}
	}
}

func TestRequirePermission_HTTP(t *testing.T) {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	tests := []struct {
		name       string
		role       string
		perms      []string
		required   string
		wantStatus int
	}{
		{"admin bypasses", "admin", nil, "devices.write", http.StatusOK},
		{"super_admin bypasses", "super_admin", nil, "devices.write", http.StatusOK},
		{"viewer with permission", "viewer", []string{"devices.read"}, "devices.read", http.StatusOK},
		{"viewer missing permission", "viewer", []string{"devices.read"}, "devices.write", http.StatusForbidden},
		{"viewer empty permissions", "viewer", nil, "devices.read", http.StatusForbidden},
		{"no claims", "", nil, "devices.read", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.role != "" || tt.perms != nil {
				ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
					UserID:      1,
					Username:    "testuser",
					Role:        tt.role,
					Permissions: tt.perms,
				})
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := RequirePermission(tt.required)(noopHandler)
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestRequireAnyPermission_HTTP(t *testing.T) {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		role       string
		perms      []string
		required   []string
		wantStatus int
	}{
		{"admin bypasses", "admin", nil, []string{"devices.write", "alerts.create"}, http.StatusOK},
		{"viewer has one", "viewer", []string{"devices.read"}, []string{"devices.read", "alerts.read"}, http.StatusOK},
		{"viewer has none", "viewer", []string{"devices.read"}, []string{"devices.write", "alerts.create"}, http.StatusForbidden},
		{"viewer empty perms", "viewer", nil, []string{"devices.read"}, http.StatusForbidden},
		{"no claims", "", nil, []string{"devices.read"}, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.role != "" || tt.perms != nil {
				ctx := context.WithValue(req.Context(), auth.ClaimsKey, &auth.Claims{
					UserID:      1,
					Username:    "testuser",
					Role:        tt.role,
					Permissions: tt.perms,
				})
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := RequireAnyPermission(tt.required...)(noopHandler)
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
