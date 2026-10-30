package netutil

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSafeTransportBlocksLoopbackAtConnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := SafeHTTPClient(SafeTransport(DefaultHostPolicy()), 0)
	resp, err := client.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected loopback connection to be blocked")
	}
	if !errors.Is(err, ErrBlockedHost) {
		t.Fatalf("expected ErrBlockedHost, got %v", err)
	}
}

func TestSafeHTTPClientRefusesRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/", http.StatusFound)
	}))
	defer srv.Close()

	// Plain transport so the test server itself is reachable.
	client := SafeHTTPClient(http.DefaultTransport, 0)
	resp, err := client.Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected redirect to be refused")
	}
	if !errors.Is(err, ErrRedirectBlocked) {
		t.Fatalf("expected ErrRedirectBlocked, got %v", err)
	}
}
