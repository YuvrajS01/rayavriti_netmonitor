package httputil

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
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

func SendError(w http.ResponseWriter, status int, message string) {
	code := httpStatusToCode(status)
	respond(w, status, Response{Success: false, Error: &APIError{Code: code, Message: message}})
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
