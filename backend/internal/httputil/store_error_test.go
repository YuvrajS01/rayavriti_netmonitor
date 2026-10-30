package httputil

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSendStoreError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		hidden   string
		shown    string
	}{
		{"unique violation", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505", ConstraintName: "uq_users_username", Message: "duplicate key value violates unique constraint"}), http.StatusConflict, "uq_users_username", "already exists"},
		{"fk violation", &pgconn.PgError{Code: "23503", TableName: "devices"}, http.StatusBadRequest, "devices", "referenced record"},
		{"syntax error is internal", &pgconn.PgError{Code: "42601", Message: "syntax error at or near SELECT"}, http.StatusInternalServerError, "syntax", "internal server error"},
		{"connection failure is internal", &pgconn.ConnectError{}, http.StatusInternalServerError, "", "internal server error"},
		{"validation error is shown", errors.New("unknown resource field: foo"), http.StatusBadRequest, "", "unknown resource field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SendStoreError(w, http.StatusBadRequest, tt.err)
			body := w.Body.String()
			if w.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d (%s)", w.Code, tt.wantCode, body)
			}
			if tt.hidden != "" && strings.Contains(body, tt.hidden) {
				t.Fatalf("leaked %q: %s", tt.hidden, body)
			}
			if !strings.Contains(body, tt.shown) {
				t.Fatalf("body %s does not mention %q", body, tt.shown)
			}
		})
	}
}
