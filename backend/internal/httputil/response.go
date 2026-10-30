package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type APIError struct {
	Code    string `json:"code,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// ValidationError represents a field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type ResponseMeta struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"pageSize,omitempty"`
	Total    int `json:"total,omitempty"`
}

type Response struct {
	Success bool          `json:"success"`
	Data    any           `json:"data,omitempty"`
	Error   *APIError     `json:"error,omitempty"`
	Meta    *ResponseMeta `json:"meta,omitempty"`
}

func SendOK(w http.ResponseWriter, data any) {
	respond(w, http.StatusOK, Response{Success: true, Data: data})
}

func SendOKWithMeta(w http.ResponseWriter, data any, meta *ResponseMeta) {
	respond(w, http.StatusOK, Response{Success: true, Data: data, Meta: meta})
}

func SendCreated(w http.ResponseWriter, data any) {
	respond(w, http.StatusCreated, Response{Success: true, Data: data})
}

// SendStatus writes a data envelope with an explicit status code, e.g. a
// readiness report that must be 503 while still carrying details.
func SendStatus(w http.ResponseWriter, status int, data any) {
	respond(w, status, Response{Success: status < 400, Data: data})
}

func SendError(w http.ResponseWriter, status int, message string) {
	code := httpStatusToCode(status)
	respond(w, status, Response{Success: false, Error: &APIError{Code: code, Message: message}})
}

// SendInternalError logs the underlying error and returns a generic 500
// response to the client. Use this instead of SendError(w, 500, err.Error())
// to avoid leaking raw driver/SQL errors that can expose schema names,
// constraint details, and DSN fragments.
func SendInternalError(w http.ResponseWriter, err error) {
	slog.Error("internal server error", "error", err)
	respond(w, http.StatusInternalServerError, Response{Success: false, Error: &APIError{Code: "INTERNAL_ERROR", Message: "internal server error"}})
}

// SendStoreError reports a failed store operation without leaking database
// internals (S15). PostgreSQL errors become a generic message chosen by
// SQLSTATE; connection and timeout failures become a 500; anything else is
// one of our own validation errors and is shown with the given status.
func SendStoreError(w http.ResponseWriter, status int, err error) {
	switch {
	case sendPgError(w, err):
	case errors.Is(err, pgx.ErrNoRows):
		SendError(w, http.StatusNotFound, "not found")
	case isInfraError(err):
		SendInternalError(w, err)
	default:
		SendError(w, status, err.Error())
	}
}

// SendDBError is SendStoreError for calls that never produce user-facing
// validation errors: anything that isn't a mapped PostgreSQL error is a 500.
func SendDBError(w http.ResponseWriter, err error) {
	if !sendPgError(w, err) {
		SendInternalError(w, err)
	}
}

// sendPgError writes a safe response for a PostgreSQL error and reports
// whether err was one.
func sendPgError(w http.ResponseWriter, err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	slog.Warn("store error", "sqlstate", pgErr.Code, "error", err)
	switch {
	case pgErr.Code == "23505":
		SendError(w, http.StatusConflict, "a record with these values already exists")
	case pgErr.Code == "23503":
		SendError(w, http.StatusBadRequest, "a referenced record does not exist or is still in use")
	case pgErr.Code == "23502":
		SendError(w, http.StatusBadRequest, "a required field is missing")
	case strings.HasPrefix(pgErr.Code, "22"), strings.HasPrefix(pgErr.Code, "23"):
		SendError(w, http.StatusBadRequest, "invalid value")
	default:
		SendInternalError(w, err)
	}
	return true
}

func isInfraError(err error) bool {
	var connErr *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &connErr) || errors.As(err, &netErr) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func SendErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, Response{Success: false, Error: &APIError{Code: code, Message: message}})
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ParseJSON(r *http.Request, v any) error {
	if r.ContentLength == 0 {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(v)
}

// QueryParamInt reads a query parameter as an integer with min/default/max bounds.
// Returns the clamped value. If the param is missing or invalid, returns def.
func QueryParamInt(r *http.Request, key string, def, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

// RequiredString returns an error message if s is empty.
func RequiredString(s, field string) string {
	if s == "" {
		return field + " is required"
	}
	return ""
}

// InRangeInt returns an error message if v is outside [min, max].
func InRangeInt(v int, field string, min, max int) string {
	if v < min || v > max {
		return field + " must be between " + strconv.Itoa(min) + " and " + strconv.Itoa(max)
	}
	return ""
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail checks if the string is a valid email format.
func IsValidEmail(s string) bool {
	return emailRegex.MatchString(s)
}

func httpStatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		return "ERROR"
	}
}
