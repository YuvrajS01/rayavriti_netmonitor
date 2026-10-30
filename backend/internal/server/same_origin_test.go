package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameOrigin(t *testing.T) {
	mw := RequireSameOrigin([]string{"http://localhost:5173"})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	tests := []struct {
		name, method, origin, auth string
		want                       int
	}{
		{"same host", "POST", "https://nm.example.edu", "", http.StatusOK},
		{"allowed dev origin", "POST", "http://localhost:5173", "", http.StatusOK},
		{"sibling subdomain", "POST", "https://evil.example.edu", "", http.StatusForbidden},
		{"no origin (curl)", "DELETE", "", "", http.StatusOK},
		{"bearer token not checked", "POST", "https://evil.example.edu", "Bearer x", http.StatusOK},
		{"GET not checked", "GET", "https://evil.example.edu", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "https://nm.example.edu/api/v1/devices", nil)
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			if tt.auth != "" {
				r.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			mw(ok).ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}
