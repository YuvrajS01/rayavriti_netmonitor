package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandler(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, "public")
	if err := os.MkdirAll(filepath.Join(static, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(static, "index.html"), []byte("INDEX"), 0o600)
	_ = os.WriteFile(filepath.Join(static, "assets", "app.js"), []byte("APP"), 0o600)
	_ = os.WriteFile(filepath.Join(root, "secret.txt"), []byte("SECRET"), 0o600)

	h := spaHandler{staticDir: static}
	tests := []struct {
		path, want string
	}{
		{"/assets/app.js", "APP"},
		{"/devices/42", "INDEX"},
		// Traversal attempts are rejected (400) or fall back to index.
		{"/../secret.txt", ""},
		{"/assets/../../secret.txt", ""},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.URL.Path = tt.path
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		body := w.Body.String()
		if strings.Contains(body, "SECRET") {
			t.Fatalf("%s escaped the static dir", tt.path)
		}
		if !strings.Contains(body, tt.want) {
			t.Fatalf("%s: got %q (status %d), want %q", tt.path, body, w.Code, tt.want)
		}
	}
}

func TestSPAHandler_UnknownAPIPathIsJSON404(t *testing.T) {
	static := t.TempDir()
	_ = os.WriteFile(filepath.Join(static, "index.html"), []byte("INDEX"), 0o600)
	h := spaHandler{staticDir: static}
	for _, p := range []string{"/api", "/api/v1/nope", "/api/v1/devices/1/unknown"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusNotFound || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
			t.Errorf("%s: got %d %q, want JSON 404", p, w.Code, w.Header().Get("Content-Type"))
		}
	}
	// "/apiary" is an app route, not an API path.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apiary", nil))
	if w.Body.String() != "INDEX" {
		t.Errorf("/apiary: got %q, want the app shell", w.Body.String())
	}
}
