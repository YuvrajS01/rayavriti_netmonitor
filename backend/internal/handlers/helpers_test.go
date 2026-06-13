package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/config"
)

const (
	testJWTSecret = "test-secret-key-for-handlers"
	testUsername  = "admin"
	testPassword  = "password123"
	testUserID    = int64(1)
	testUserRole  = "admin"
)

func testConfig() *config.Config {
	return &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:          testJWTSecret,
			AccessTokenExpiry:  15 * time.Minute,
			RefreshTokenExpiry: 7 * 24 * time.Hour,
		},
	}
}

func authenticatedRequest(method, url string, body string) (*httptest.ResponseRecorder, *http.Request) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	token, _, _ := auth.GenerateTokenPair(testUserID, testUsername, testUserRole, testJWTSecret, 15*time.Minute, 7*24*time.Hour)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	return w, req
}

// callWithAuth wraps a handler with RequireAuth middleware, then calls it.
// This properly sets claims in the request context.
func callWithAuth(handler http.HandlerFunc, w http.ResponseWriter, r *http.Request) {
	token, _, _ := auth.GenerateTokenPair(testUserID, testUsername, testUserRole, testJWTSecret, 15*time.Minute, 7*24*time.Hour)
	req := r.Clone(r.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	middleware := auth.RequireAuth(testJWTSecret, func(ctx context.Context, hash string) (*auth.Claims, error) {
		return nil, nil
	})
	middleware(handler).ServeHTTP(w, req)
}

// callWithAuthAndParams wraps a handler with RequireAuth and chi params.
func callWithAuthAndParams(handler http.HandlerFunc, w http.ResponseWriter, r *http.Request, params ...string) {
	if len(params) > 0 {
		rctx := chi.NewRouteContext()
		for i := 0; i+1 < len(params); i += 2 {
			rctx.URLParams.Add(params[i], params[i+1])
		}
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	token, _, _ := auth.GenerateTokenPair(testUserID, testUsername, testUserRole, testJWTSecret, 15*time.Minute, 7*24*time.Hour)
	r.Header.Set("Authorization", "Bearer "+token)
	middleware := auth.RequireAuth(testJWTSecret, func(ctx context.Context, hash string) (*auth.Claims, error) {
		return nil, nil
	})
	middleware(handler).ServeHTTP(w, r)
}

func withChiParams(r *http.Request, params ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(params); i += 2 {
		rctx.URLParams.Add(params[i], params[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func makeRequestWithParams(method, url string, body string, params ...string) (*httptest.ResponseRecorder, *http.Request) {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	if len(params) > 0 {
		req = withChiParams(req, params...)
	}
	return httptest.NewRecorder(), req
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}
