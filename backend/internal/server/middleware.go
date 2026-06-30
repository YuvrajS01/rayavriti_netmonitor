package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"github.com/rayavriti/netmonitor-backend/internal/logging"
	"golang.org/x/time/rate"
)

type requestIDKey struct{}

// RequestID adds a unique request ID to each request and response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			}
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// SecurityHeaders adds helmet-equivalent security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Only set HSTS when the request is over TLS (or via a known HTTPS proxy)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self' ws: wss:")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Embedder-Policy", "credentialless")
		next.ServeHTTP(w, r)
	})
}

type rateLimitClient struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// RateLimiter creates a per-IP rate limiter middleware with automatic cleanup
// of stale entries to prevent memory leaks.
// If rdb is provided, uses Redis-backed sliding window rate limiting.
func RateLimiter(ctx context.Context, rps float64, burst int, rdb *cache.Redis) func(http.Handler) http.Handler {
	if rdb != nil {
		return redisRateLimiter(rdb, burst)
	}
	return inMemoryRateLimiter(ctx, rps, burst)
}

func redisRateLimiter(rdb *cache.Redis, burst int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			key := fmt.Sprintf("nm:rl:ip:%s", ip)
			allowed, remaining, resetAt, _ := rdb.RateLimit(r.Context(), key, burst, time.Second)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
			if !allowed {
				SendError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func inMemoryRateLimiter(ctx context.Context, rps float64, burst int) func(http.Handler) http.Handler {
	var mu sync.Mutex
	clients := map[string]*rateLimitClient{}

	// Background cleanup of stale rate limiters (every 3 minutes)
	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				for ip, c := range clients {
					if time.Since(c.lastSeen) > 5*time.Minute {
						delete(clients, ip)
					}
				}
				mu.Unlock()
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			mu.Lock()
			c, ok := clients[ip]
			if !ok {
				c = &rateLimitClient{lim: rate.NewLimiter(rate.Limit(rps), burst)}
				clients[ip] = c
			}
			c.lastSeen = time.Now()
			mu.Unlock()
			if !c.lim.Allow() {
				SendError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Recovery recovers from panics and returns a 500 error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				SendError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestSize limits the maximum request body size.
func RequestSize(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

// AuditLog returns middleware that logs write operations (POST, PUT, DELETE)
// using the provided audit logger. Requires RequireAuth to have run first.
func AuditLog(al *logging.AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
				return
			}

			claims := auth.GetClaims(r.Context())
			actor := "system"
			var userID int64
			if claims != nil {
				actor = "user:" + claims.Username
				userID = claims.UserID
			}

			al.LogEvent(r.Context(), logging.AuditEvent{
				EventType:    "api." + strings.ToLower(r.Method),
				Severity:     "info",
				Actor:        actor,
				ActorIP:      r.RemoteAddr,
				ResourceType: extractResourceType(r.URL.Path),
				Description:  r.Method + " " + r.URL.Path,
				Details: map[string]any{
					"user_id": userID,
					"method":  r.Method,
					"path":    r.URL.Path,
				},
			})
		})
	}
}

func extractResourceType(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "v1" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) >= 2 {
		return parts[1]
	}
	return "unknown"
}
