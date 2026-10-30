package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name, remote, xff, want string
	}{
		{"direct client ignores spoofed XFF", "203.0.113.5:4000", "1.2.3.4", "203.0.113.5"},
		{"local proxy uses rightmost XFF", "127.0.0.1:5000", "1.2.3.4, 198.51.100.7", "198.51.100.7"},
		{"local proxy without XFF", "127.0.0.1:5000", "", "127.0.0.1"},
		{"local proxy with junk XFF", "127.0.0.1:5000", "not-an-ip", "127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(r); got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
