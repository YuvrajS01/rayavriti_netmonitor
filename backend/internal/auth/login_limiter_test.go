package auth

import (
	"context"
	"testing"
	"time"
)

func TestLoginLimiter_InMemory_Allow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil) // nil rdb → in-memory mode

	// First few attempts should be allowed.
	for i := 0; i < 5; i++ {
		allowed, reason := ll.Allow("192.168.1.1", "testuser")
		if !allowed {
			t.Fatalf("attempt %d: expected allowed, got blocked: %s", i+1, reason)
		}
	}
}

func TestLoginLimiter_InMemory_IPThrottle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil)
	ll.ipLimit = 3 // lower for testing

	// Exhaust the IP limit.
	for i := 0; i < 3; i++ {
		_, _ = ll.Allow("10.0.0.1", "user1")
	}
	// 4th attempt from the same IP should be blocked.
	allowed, reason := ll.Allow("10.0.0.1", "user2")
	if allowed {
		t.Fatal("expected IP throttle to block 4th attempt")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason string")
	}
}

func TestLoginLimiter_InMemory_UsernameThrottle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil)
	ll.userLimit = 3 // lower for testing

	// Exhaust the username limit from different IPs.
	for i := 0; i < 3; i++ {
		_, _ = ll.Allow("10.0.0."+string(rune('0'+i)), "targetuser")
	}
	// 4th attempt for the same username from a new IP should be blocked.
	allowed, reason := ll.Allow("192.168.1.99", "targetuser")
	if allowed {
		t.Fatal("expected username throttle to block 4th attempt")
	}
	if reason == "" {
		t.Fatal("expected non-empty reason string")
	}
}

func TestLoginLimiter_InMemory_ResetSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil)
	ll.userLimit = 3

	// Use up some of the username limit.
	_, _ = ll.Allow("10.0.0.1", "resetuser")
	_, _ = ll.Allow("10.0.0.2", "resetuser")

	// Reset on success.
	ll.ResetSuccess("resetuser")

	// Should be able to make 3 more attempts.
	for i := 0; i < 3; i++ {
		allowed, _ := ll.Allow("10.0.0."+string(rune('3'+i)), "resetuser")
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed after reset", i+1)
		}
	}
}

func TestLoginLimiter_InMemory_DifferentIPsIndependent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil)
	ll.ipLimit = 2

	// Exhaust IP 1.
	_, _ = ll.Allow("10.0.0.1", "user1")
	_, _ = ll.Allow("10.0.0.1", "user2")

	// IP 2 should still be allowed.
	allowed, _ := ll.Allow("10.0.0.2", "user3")
	if !allowed {
		t.Fatal("expected different IP to be independent")
	}
}

func TestLoginLimiter_InMemory_CleanupRemovesStale(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ll := NewLoginLimiter(ctx, nil)
	ll.ipLimit = 1

	// Add an entry.
	_, _ = ll.Allow("10.0.0.1", "user1")

	// Manually expire the entries by backdating them.
	ll.mu.Lock()
	for _, c := range ll.ipCounts {
		for i := range c.events {
			c.events[i] = time.Now().Add(-2 * time.Minute)
		}
	}
	for _, c := range ll.unCounts {
		for i := range c.events {
			c.events[i] = time.Now().Add(-2 * time.Minute)
		}
	}
	ll.mu.Unlock()

	// After expiry, the IP should be allowed again.
	allowed, _ := ll.Allow("10.0.0.1", "user1")
	if !allowed {
		t.Fatal("expected stale entries to be pruned, allowing new attempt")
	}
}
