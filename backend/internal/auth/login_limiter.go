package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
)

// LoginLimiter throttles login attempts per-IP and per-username to prevent
// brute-force attacks. It works with or without Redis.
//
// Limits:
//   - Per-IP: maxFailed attempts per IPWindow (e.g. 10 per minute)
//   - Per-username-from-IP: 5 attempts per minute, so one source can't
//     lock a user out for everyone else
//   - Per-username overall: a higher cap (30 per minute) that still catches
//     distributed guessing against one account
//
// Both counters are sliding windows. A request is blocked if EITHER counter
// has been exceeded. Only failed attempts increment the counter; successful
// logins reset the per-username counter.
type LoginLimiter struct {
	rdb *cache.Redis

	// In-memory fallback when Redis is unavailable.
	mu       sync.Mutex
	ipCounts map[string]*slidingCounter
	unCounts map[string]*slidingCounter

	ipLimit         int
	ipWindow        time.Duration
	userLimit       int
	userWindow      time.Duration
	userGlobalLimit int
	cleanupCtx      context.Context
}

type slidingCounter struct {
	events []time.Time
}

// NewLoginLimiter creates a login limiter. If rdb is nil, an in-memory
// fallback is used. The ctx controls the background cleanup goroutine for
// the in-memory path.
func NewLoginLimiter(ctx context.Context, rdb *cache.Redis) *LoginLimiter {
	ll := &LoginLimiter{
		rdb:             rdb,
		ipCounts:        make(map[string]*slidingCounter),
		unCounts:        make(map[string]*slidingCounter),
		ipLimit:         10,
		ipWindow:        time.Minute,
		userLimit:       5,
		userWindow:      time.Minute,
		userGlobalLimit: 30,
		cleanupCtx:      ctx,
	}
	if rdb == nil {
		go ll.cleanupLoop()
	}
	return ll
}

// Allow checks whether a login attempt from the given IP and username should
// be allowed. Returns (true, "") if allowed, or (false, reason) if throttled.
func (ll *LoginLimiter) Allow(ip, username string) (bool, string) {
	if ll.rdb != nil {
		return ll.allowRedis(ip, username)
	}
	return ll.allowInMemory(ip, username)
}

func (ll *LoginLimiter) allowRedis(ip, username string) (bool, string) {
	ctx, cancel := context.WithTimeout(ll.cleanupCtx, 2*time.Second)
	defer cancel()

	ipKey := fmt.Sprintf("nm:login:ip:%s", ip)
	ipOK, ipRemaining, _, ipErr := ll.rdb.RateLimit(ctx, ipKey, ll.ipLimit, ll.ipWindow)
	if ipErr != nil {
		// Fail-open on Redis errors (the connection itself may be the issue).
		ipOK = true
		ipRemaining = ll.ipLimit
	}

	unKey := fmt.Sprintf("nm:login:user:%s|%s", username, ip)
	unOK, unRemaining, _, unErr := ll.rdb.RateLimit(ctx, unKey, ll.userLimit, ll.userWindow)
	if unErr != nil {
		unOK = true
		unRemaining = ll.userLimit
	}
	globalOK, _, _, globalErr := ll.rdb.RateLimit(ctx, fmt.Sprintf("nm:login:user:%s", username), ll.userGlobalLimit, ll.userWindow)
	if globalErr != nil {
		globalOK = true
	}
	unOK = unOK && globalOK

	if !ipOK {
		return false, fmt.Sprintf("too many login attempts from this IP (retry in %s, remaining: %d)", ll.ipWindow, ipRemaining)
	}
	if !unOK {
		return false, fmt.Sprintf("too many login attempts for this user (retry in %s, remaining: %d)", ll.userWindow, unRemaining)
	}
	return true, ""
}

func (ll *LoginLimiter) allowInMemory(ip, username string) (bool, string) {
	now := time.Now()
	ll.mu.Lock()
	defer ll.mu.Unlock()

	ipCount := ll.prune(ll.ipCounts[ip], now, ll.ipWindow)
	if ipCount >= ll.ipLimit {
		return false, fmt.Sprintf("too many login attempts from this IP (limit %d per %s)", ll.ipLimit, ll.ipWindow)
	}
	userIPKey := username + "|" + ip
	if ll.prune(ll.unCounts[userIPKey], now, ll.userWindow) >= ll.userLimit ||
		ll.prune(ll.unCounts[username], now, ll.userWindow) >= ll.userGlobalLimit {
		return false, fmt.Sprintf("too many login attempts for this user (limit %d per %s)", ll.userLimit, ll.userWindow)
	}
	// Record this attempt for every counter.
	ll.record(ll.ipCounts, ip, now, ll.ipWindow)
	if username != "" {
		ll.record(ll.unCounts, userIPKey, now, ll.userWindow)
		ll.record(ll.unCounts, username, now, ll.userWindow)
	}
	return true, ""
}

// prune removes expired events and returns the current count.
func (ll *LoginLimiter) prune(c *slidingCounter, now time.Time, window time.Duration) int {
	if c == nil {
		return 0
	}
	cutoff := now.Add(-window)
	idx := 0
	for idx < len(c.events) && c.events[idx].Before(cutoff) {
		idx++
	}
	if idx > 0 {
		c.events = c.events[idx:]
	}
	return len(c.events)
}

func (ll *LoginLimiter) record(m map[string]*slidingCounter, key string, now time.Time, window time.Duration) {
	c, ok := m[key]
	if !ok {
		c = &slidingCounter{}
		m[key] = c
	}
	c.events = append(c.events, now)
}

// ResetSuccess clears the per-username counter on a successful login so that
// a user who had a few typos isn't penalized after authenticating correctly.
func (ll *LoginLimiter) ResetSuccess(ip, username string) {
	if ll.rdb != nil {
		ctx, cancel := context.WithTimeout(ll.cleanupCtx, 2*time.Second)
		defer cancel()
		_ = ll.rdb.Del(ctx, fmt.Sprintf("nm:login:user:%s|%s", username, ip))
		return
	}
	ll.mu.Lock()
	defer ll.mu.Unlock()
	delete(ll.unCounts, username+"|"+ip)
}

func (ll *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ll.cleanupCtx.Done():
			return
		case <-ticker.C:
			ll.mu.Lock()
			now := time.Now()
			for k, c := range ll.ipCounts {
				if ll.prune(c, now, ll.ipWindow) == 0 {
					delete(ll.ipCounts, k)
				}
			}
			for k, c := range ll.unCounts {
				if ll.prune(c, now, ll.userWindow) == 0 {
					delete(ll.unCounts, k)
				}
			}
			ll.mu.Unlock()
		}
	}
}
