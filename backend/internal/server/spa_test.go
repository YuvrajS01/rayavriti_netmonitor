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
