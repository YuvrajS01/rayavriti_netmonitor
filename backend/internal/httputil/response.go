package httputil

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func SendOK(w http.ResponseWriter, data any) {
	respond(w, http.StatusOK, Response{Success: true, Data: data})
}

func SendCreated(w http.ResponseWriter, data any) {
	respond(w, http.StatusCreated, Response{Success: true, Data: data})
}

func SendError(w http.ResponseWriter, status int, message string) {
	respond(w, status, Response{Success: false, Error: message})
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
