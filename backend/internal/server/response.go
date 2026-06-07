package server

import (
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/httputil"
)

// Re-export httputil helpers so server package callers don't need to import httputil.
func SendOK(w http.ResponseWriter, data any)              { httputil.SendOK(w, data) }
func SendCreated(w http.ResponseWriter, data any)         { httputil.SendCreated(w, data) }
func SendError(w http.ResponseWriter, status int, msg string) { httputil.SendError(w, status, msg) }
func ParseJSON(r *http.Request, v any) error              { return httputil.ParseJSON(r, v) }
