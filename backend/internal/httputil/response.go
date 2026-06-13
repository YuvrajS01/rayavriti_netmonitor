package httputil

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
