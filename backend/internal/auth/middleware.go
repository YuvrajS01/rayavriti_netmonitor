package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
	"golang.org/x/time/rate"
)

type contextKey int

const claimsKey contextKey = 0

// RequireAuth validates JWT from Authorization: Bearer or X-Api-Key header.
// apiKeyLookup is called when an X-Api-Key header is present; it should return Claims or nil.
func RequireAuth(secret string, apiKeyLookup func(ctx context.Context, hash string) (*Claims, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// API key check
			if key := r.Header.Get("X-Api-Key"); key != "" && apiKeyLookup != nil {
				hash := HashAPIKey(key)
				claims, err := apiKeyLookup(r.Context(), hash)
				if err == nil && claims != nil {
					next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
					return
				}
			}
			// JWT check
			raw := r.Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				sendUnauth(w, "missing or invalid authorization")
				return
			}
			claims, err := ValidateToken(strings.TrimPrefix(raw, "Bearer "), secret)
			if err != nil {
				sendUnauth(w, "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
		})
	}
}

// GetClaims retrieves Claims from context (set by RequireAuth).
func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// roleWeight maps roles to numeric weight for hierarchy checks.
var roleWeight = map[string]int{
	"viewer":        1,
	"dept_admin":    2,
	"network_admin": 3,
	"admin":         4,
	"super_admin":   4,
}

// RequireRole ensures the authenticated user has at least the given role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				sendUnauth(w, "not authenticated")
				return
			}
			if roleWeight[claims.Role] < roleWeight[role] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sendUnauth(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
}

type rateLimitKey struct {
	userID int64
	apiKey bool
}

type rateLimitClient struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// UserRateLimiter creates a per-user/API-key rate limiter middleware.
// JWT users: 5000 req/hr, API keys: 10000 req/hr.
// If rdb is provided, uses Redis-backed sliding window rate limiting.
func UserRateLimiter(ctx context.Context, rdb *cache.Redis) func(http.Handler) http.Handler {
	if rdb != nil {
		return redisUserRateLimiter(rdb)
	}
	return inMemoryUserRateLimiter(ctx)
}

func redisUserRateLimiter(rdb *cache.Redis) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				next.ServeHTTP(w, r)
				return
			}

			isAPIKey := r.Header.Get("X-Api-Key") != ""
			var key string
			var limit int
			if isAPIKey {
				key = fmt.Sprintf("nm:rl:apikey:%d", claims.UserID)
				limit = 10000
			} else {
				key = fmt.Sprintf("nm:rl:user:%d", claims.UserID)
				limit = 5000
			}

			allowed, remaining, resetAt, _ := rdb.RateLimit(r.Context(), key, limit, time.Hour)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "rate limit exceeded",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func inMemoryUserRateLimiter(ctx context.Context) func(http.Handler) http.Handler {
	var mu sync.Mutex
	clients := map[rateLimitKey]*rateLimitClient{}

	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				for k, c := range clients {
					if time.Since(c.lastSeen) > 5*time.Minute {
						delete(clients, k)
					}
				}
				mu.Unlock()
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				next.ServeHTTP(w, r)
				return
			}

			key := rateLimitKey{userID: claims.UserID}
			isAPIKey := r.Header.Get("X-Api-Key") != ""
			if isAPIKey {
				key.apiKey = true
			}

			var limit rate.Limit
			var burst int
			if isAPIKey {
				limit = rate.Limit(10000.0 / 3600.0)
				burst = 200
			} else {
				limit = rate.Limit(5000.0 / 3600.0)
				burst = 100
			}

			mu.Lock()
			c, ok := clients[key]
			if !ok {
				c = &rateLimitClient{lim: rate.NewLimiter(limit, burst)}
				clients[key] = c
			}
			c.lastSeen = time.Now()
			mu.Unlock()

			if !c.lim.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"error":   "rate limit exceeded",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
