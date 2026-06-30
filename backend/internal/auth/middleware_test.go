package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_ValidJWT(t *testing.T) {
	t.Parallel()
	token, _, err := GenerateTokenPair(1, "admin", "admin", testSecret, 15*time.Minute, 7*24*time.Hour)
	require.NoError(t, err)

	nextCalled := false
	handler := RequireAuth(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		claims := GetClaims(r.Context())
		require.NotNil(t, claims)
		assert.Equal(t, int64(1), claims.UserID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_ExpiredJWT(t *testing.T) {
	t.Parallel()
	token, _, err := GenerateTokenPair(1, "admin", "admin", testSecret, -1*time.Hour, 7*24*time.Hour)
	require.NoError(t, err)

	nextCalled := false
	handler := RequireAuth(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireAuth(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_InvalidPrefix(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireAuth(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_ValidAPIKey(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireAuth(testSecret, func(ctx context.Context, hash string) (*Claims, error) {
		return &Claims{UserID: 1, Username: "apikey-user", Role: "admin"}, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		claims := GetClaims(r.Context())
		require.NotNil(t, claims)
		assert.Equal(t, "apikey-user", claims.Username)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Api-Key", "test-api-key-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_InvalidAPIKey(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireAuth(testSecret, func(ctx context.Context, hash string) (*Claims, error) {
		return nil, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Api-Key", "invalid-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireRole_Admin_AccessingAdmin(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	ctx := context.WithValue(context.Background(), ClaimsKey, &Claims{UserID: 1, Username: "admin", Role: "admin"})
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Viewer_AccessingAdmin(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	ctx := context.WithValue(context.Background(), ClaimsKey, &Claims{UserID: 1, Username: "viewer", Role: "viewer"})
	req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_NoClaims(t *testing.T) {
	t.Parallel()
	nextCalled := false
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireRole_HierarchyCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		userRole   string
		required   string
		expectCode int
	}{
		{"super_admin accessing admin", "super_admin", "admin", http.StatusOK},
		{"admin accessing admin", "admin", "admin", http.StatusOK},
		{"network_admin accessing admin", "network_admin", "admin", http.StatusForbidden},
		{"dept_admin accessing admin", "dept_admin", "admin", http.StatusForbidden},
		{"viewer accessing admin", "viewer", "admin", http.StatusForbidden},
		{"viewer accessing viewer", "viewer", "viewer", http.StatusOK},
		{"admin accessing viewer", "admin", "viewer", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			nextCalled := false
			handler := RequireRole(tt.required)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			ctx := context.WithValue(context.Background(), ClaimsKey, &Claims{UserID: 1, Role: tt.userRole})
			req := httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if tt.expectCode == http.StatusOK {
				assert.True(t, nextCalled)
			} else {
				assert.False(t, nextCalled)
			}
			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestGetClaims_EmptyContext(t *testing.T) {
	t.Parallel()
	claims := GetClaims(context.Background())
	assert.Nil(t, claims)
}

func TestRequireAuth_JSONResponse(t *testing.T) {
	t.Parallel()
	handler := RequireAuth(testSecret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
