package logging

import (
	"fmt"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// RequestLogger returns middleware that logs each HTTP request/response.
func RequestLogger(logger *Logger) func(http.Handler) http.Handler {
	l := logger.With("http")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := GenerateRequestID()
			ctx := WithRequestID(r.Context(), reqID)
			w.Header().Set("X-Request-ID", reqID)

			rw := &responseWriter{ResponseWriter: w, status: 200}
			start := time.Now()

			l.InfoCtx(ctx, fmt.Sprintf("→ %s %s", r.Method, r.URL.Path),
				"method", r.Method,
				"path", r.URL.Path,
				"remote", r.RemoteAddr,
				"request_id", reqID,
			)

			next.ServeHTTP(rw, r.WithContext(ctx))

			dur := time.Since(start)
			l.InfoCtx(ctx, fmt.Sprintf("← %d %s %s", rw.status, r.Method, r.URL.Path),
				"status", rw.status,
				"bytes", rw.size,
				"duration_ms", float64(dur.Milliseconds()),
				"request_id", reqID,
			)
		})
	}
}
