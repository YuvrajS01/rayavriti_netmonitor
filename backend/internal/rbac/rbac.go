package rbac

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

// HasPermission checks if the given permission set contains the required permission.
func HasPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == required {
			return true
		}
	}
	return false
}

// ParsePermissions extracts a string slice from raw JSONB permissions.
func ParsePermissions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var perms []string
	if err := json.Unmarshal(raw, &perms); err != nil {
		return nil
	}
	return perms
}

// RequirePermission returns middleware that checks if the authenticated user
// has the required permission. Requires RequireAuth to have run first.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"success":false,"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}
			// Super_admin and admin bypass permission checks
			if claims.Role == "super_admin" || claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			if !HasPermission(claims.Permissions, permission) {
				slog.Warn("RBAC: permission denied",
					"user", claims.Username,
					"role", claims.Role,
					"required", permission,
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "forbidden",
					"detail":  "missing permission: " + permission,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission returns middleware that checks if the user has at least
// one of the required permissions.
func RequireAnyPermission(permissions ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"success":false,"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}
			if claims.Role == "super_admin" || claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			for _, perm := range permissions {
				if HasPermission(claims.Permissions, perm) {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "forbidden",
				"detail":  "missing required permissions",
			})
		})
	}
}
