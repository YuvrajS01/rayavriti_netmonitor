package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	sess := &Session{UserID: 1, Username: "admin", Role: "admin", ExpireAt: time.Now().Add(1 * time.Hour)}
	store.Set("token123", sess)

	got, ok := store.Get("token123")
	require.True(t, ok)
	assert.Equal(t, sess.UserID, got.UserID)
	assert.Equal(t, sess.Username, got.Username)
	assert.Equal(t, sess.Role, got.Role)
}

func TestSessionStore_Get_Expired(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	sess := &Session{UserID: 1, Username: "admin", Role: "admin", ExpireAt: time.Now().Add(1 * time.Millisecond)}
	store.Set("token123", sess)

	time.Sleep(5 * time.Millisecond)

	got, ok := store.Get("token123")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestSessionStore_Get_NonExistent(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	got, ok := store.Get("unknown")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestSessionStore_Delete(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	sess := &Session{UserID: 1, Username: "admin", Role: "admin", ExpireAt: time.Now().Add(1 * time.Hour)}
	store.Set("token123", sess)
	store.Delete("token123")

	got, ok := store.Get("token123")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestSessionStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	var wg sync.WaitGroup
	const goroutines = 100

	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			token := "token" + string(rune('0'+id%10))
			sess := &Session{UserID: int64(id), Username: "user", Role: "viewer", ExpireAt: time.Now().Add(1 * time.Hour)}
			store.Set(token, sess)
		}(i)
		go func(id int) {
			defer wg.Done()
			token := "token" + string(rune('0'+id%10))
			store.Get(token)
		}(i)
		go func(id int) {
			defer wg.Done()
			token := "token" + string(rune('0'+id%10))
			store.Delete(token)
		}(i)
	}
	wg.Wait()
}

func TestSessionStore_MemoryCleanup(t *testing.T) {
	t.Parallel()
	store := NewSessionStore()
	for i := 0; i < 100; i++ {
		sess := &Session{UserID: int64(i), Username: "user", Role: "viewer", ExpireAt: time.Now().Add(1 * time.Millisecond)}
		store.Set("tok"+string(rune('A'+i%26)), sess)
	}
	time.Sleep(10 * time.Millisecond)

	// Expired entries are cleaned up lazily on Get
	for i := 0; i < 100; i++ {
		store.Get("tok" + string(rune('A'+i%26)))
	}

	store.mu.Lock()
	assert.Equal(t, 0, len(store.entries))
	store.mu.Unlock()
}
