package logging

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(status int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Flush implements http.Flusher if the underlying writer supports it.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the original ResponseWriter for http.ResponseController.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// RequestLogger returns middleware that logs each HTTP request/response with full detail.
func RequestLogger(logger *Logger, slowRequestMs int) func(http.Handler) http.Handler {
	l := logger.With("http")
	slowThreshold := time.Duration(slowRequestMs) * time.Millisecond

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := GenerateRequestID()
			ctx := WithRequestID(r.Context(), reqID)
			w.Header().Set("X-Request-ID", reqID)

			// Capture request body preview (first 1KB) for non-GET requests
			var bodyPreview string
			if r.Body != nil && r.ContentLength > 0 && r.Method != "GET" {
				buf := make([]byte, 1024)
				n, _ := io.ReadFull(r.Body, buf)
				bodyPreview = string(buf[:n])
				// Redact password fields for auth endpoints
				if strings.Contains(r.URL.Path, "/auth/login") {
					bodyPreview = redactPasswords(bodyPreview)
				}
				// Restore the body for downstream handlers
				r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf[:n]), r.Body))
			}

			// Determine auth type from headers
			authType := ""
			if r.Header.Get("X-Api-Key") != "" {
				authType = "api_key"
			} else if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				authType = "jwt"
			}

			// Log request start
			startArgs := []any{
				"event", "request_start",
				"method", r.Method,
				"path", r.URL.Path,
				"full_url", r.URL.String(),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.Header.Get("User-Agent"),
				"content_type", r.Header.Get("Content-Type"),
				"content_length", r.ContentLength,
				"referer", r.Header.Get("Referer"),
				"request_id", reqID,
			}
			if authType != "" {
				startArgs = append(startArgs, "auth_type", authType)
			}
			if r.URL.RawQuery != "" {
				startArgs = append(startArgs, "query_params", r.URL.RawQuery)
			}
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				startArgs = append(startArgs, "x_forwarded_for", xff)
			}
			if bodyPreview != "" {
				startArgs = append(startArgs, "request_body_preview", bodyPreview)
			}

			l.InfoCtx(ctx, fmt.Sprintf("→ %s %s", r.Method, r.URL.Path), startArgs...)

			rw := &responseWriter{ResponseWriter: w, status: 200}
			start := time.Now()

			next.ServeHTTP(rw, r.WithContext(ctx))

			dur := time.Since(start)
			durationMs := float64(dur.Microseconds()) / 1000.0

			// Build response log args
			endArgs := []any{
				"event", "request_end",
				"status", rw.status,
				"status_text", http.StatusText(rw.status),
				"response_size_bytes", rw.size,
				"duration_ms", durationMs,
				"request_id", reqID,
			}

			// Determine log level based on status code
			msg := fmt.Sprintf("← %d %s %s", rw.status, r.Method, r.URL.Path)
			level := slog.LevelInfo

			if rw.status >= 500 {
				level = slog.LevelError
			} else if rw.status >= 400 {
				level = slog.LevelWarn
			}

			// Slow request detection
			if slowThreshold > 0 && dur >= slowThreshold {
				level = slog.LevelWarn
				endArgs = append(endArgs, "slow_request", true, "threshold_ms", slowRequestMs)
				msg = fmt.Sprintf("⚠ SLOW %s", msg)
			}

			l.log(ctx, level, msg, endArgs...)
		})
	}
}

// redactPasswords replaces password values in JSON-like strings.
func redactPasswords(s string) string {
	// Simple redaction for common password field patterns
	for _, field := range []string{"password", "Password", "passwd", "secret"} {
		if idx := strings.Index(s, `"`+field+`"`); idx >= 0 {
			// Find the value start (after :")
			valStart := strings.Index(s[idx:], ":") + idx
			if valStart > idx {
				// Find opening quote of value
				qStart := strings.Index(s[valStart:], `"`) + valStart
				if qStart > valStart {
					// Find closing quote of value
					qEnd := strings.Index(s[qStart+1:], `"`) + qStart + 1
					if qEnd > qStart {
						s = s[:qStart+1] + "***" + s[qEnd:]
					}
				}
			}
		}
	}
	return s
}
