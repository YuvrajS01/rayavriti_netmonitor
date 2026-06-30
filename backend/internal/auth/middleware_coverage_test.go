package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to inject claims into request context before middleware runs
func reqWithClaims(claims *Claims) *http.Request {
	req := httptest.NewRequest("GET", "/api/test", nil)
	return req.WithContext(context.WithValue(req.Context(), ClaimsKey, claims))
}

// ── UserRateLimiter ───────────────────────────────────────────────────────────

func TestUserRateLimiter_AllowsBurst(t *testing.T) {
	t.Parallel()
	limiter := UserRateLimiter(context.Background(), nil)

	nextCalled := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(inner)

	for i := 0; i < 10; i++ {
		req := reqWithClaims(&Claims{UserID: 1, Username: "testuser", Role: "admin"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	assert.Equal(t, 10, nextCalled)
}

func TestUserRateLimiter_BlocksExcess(t *testing.T) {
	t.Parallel()
	limiter := UserRateLimiter(context.Background(), nil)

	nextCalled := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(inner)

	rateLimited := false
	for i := 0; i < 500; i++ {
		req := reqWithClaims(&Claims{UserID: 100, Username: "testuser", Role: "admin"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	assert.True(t, rateLimited, "should eventually get rate limited after many requests")
}

func TestUserRateLimiter_DifferentUsersIndependent(t *testing.T) {
	t.Parallel()
	limiter := UserRateLimiter(context.Background(), nil)

	counts := map[int64]int{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims != nil {
			counts[claims.UserID]++
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(inner)

	for i := 0; i < 50; i++ {
		req := reqWithClaims(&Claims{UserID: 1, Username: "user1", Role: "admin"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	for i := 0; i < 50; i++ {
		req := reqWithClaims(&Claims{UserID: 2, Username: "user2", Role: "admin"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	assert.Equal(t, 50, counts[1])
	assert.Equal(t, 50, counts[2])
}

func TestUserRateLimiter_NoClaims(t *testing.T) {
	t.Parallel()
	limiter := UserRateLimiter(context.Background(), nil)

	nextCalled := false
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserRateLimiter_APIKeyHigherLimit(t *testing.T) {
	t.Parallel()
	limiter := UserRateLimiter(context.Background(), nil)

	nextCalled := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled++
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(inner)

	for i := 0; i < 200; i++ {
		req := reqWithClaims(&Claims{UserID: 1, Username: "testuser", Role: "admin"})
		req.Header.Set("X-Api-Key", "test-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	assert.Equal(t, 200, nextCalled)
}

// ── Package-level session functions ──────────────────────────────────────────

func TestPackageLevel_SetSession(t *testing.T) {
	t.Parallel()
	DeleteSession("test-pkg-token")

	sess := &Session{
		UserID:   42,
		Username: "pkguser",
		Role:     "admin",
		ExpireAt: time.Now().Add(1 * time.Hour),
	}
	SetSession("test-pkg-token", sess)

	got, ok := GetSession("test-pkg-token")
	require.True(t, ok)
	assert.Equal(t, int64(42), got.UserID)
	assert.Equal(t, "pkguser", got.Username)

	DeleteSession("test-pkg-token")
}

func TestPackageLevel_GetSession_NotFound(t *testing.T) {
	t.Parallel()
	DeleteSession("nonexistent-pkg-token")
	got, ok := GetSession("nonexistent-pkg-token")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestPackageLevel_DeleteSession(t *testing.T) {
	t.Parallel()
	sess := &Session{
		UserID:   10,
		Username: "deleteme",
		Role:     "viewer",
		ExpireAt: time.Now().Add(1 * time.Hour),
	}
	SetSession("delete-pkg-token", sess)
	DeleteSession("delete-pkg-token")

	got, ok := GetSession("delete-pkg-token")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestPackageLevel_SetGetDelete_RoundTrip(t *testing.T) {
	t.Parallel()
	for i := 0; i < 50; i++ {
		tok := "roundtrip-token-" + string(rune('a'+i%26))
		sess := &Session{
			UserID:   int64(i),
			Username: "user",
			Role:     "viewer",
			ExpireAt: time.Now().Add(1 * time.Hour),
		}
		SetSession(tok, sess)
		got, ok := GetSession(tok)
		require.True(t, ok)
		assert.Equal(t, int64(i), got.UserID)
		DeleteSession(tok)
		_, ok = GetSession(tok)
		assert.False(t, ok)
	}
}

func TestPackageLevel_SessionExpired(t *testing.T) {
	t.Parallel()
	sess := &Session{
		UserID:   99,
		Username: "expired",
		Role:     "viewer",
		ExpireAt: time.Now().Add(-1 * time.Second),
	}
	SetSession("expired-pkg-token", sess)

	got, ok := GetSession("expired-pkg-token")
	assert.False(t, ok)
	assert.Nil(t, got)
}
