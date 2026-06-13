package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/cache"
)

// Session holds minimal data about an authenticated user session.
type Session struct {
	UserID   int64
	Username string
	Role     string
	ExpireAt time.Time
}

// SessionStore stores sessions in Redis with automatic expiry.
// Falls back to in-memory if Redis is nil.
type SessionStore struct {
	rdb    *cache.Redis
	mu     sync.RWMutex
	memory map[string]*Session
}

var defaultStore = &SessionStore{memory: map[string]*Session{}}

func NewSessionStore() *SessionStore {
	return &SessionStore{memory: map[string]*Session{}}
}

func NewRedisSessionStore(rdb *cache.Redis) *SessionStore {
	return &SessionStore{rdb: rdb, memory: map[string]*Session{}}
}

func (s *SessionStore) Set(token string, sess *Session) {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		key := fmt.Sprintf("nm:session:%s", HashToken(token))
		ttl := time.Until(sess.ExpireAt)
		if ttl <= 0 {
			return
		}
		_ = s.rdb.Set(ctx, key, sess, ttl)
		return
	}
	s.mu.Lock()
	s.memory[token] = sess
	s.mu.Unlock()
}

func (s *SessionStore) Get(token string) (*Session, bool) {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		key := fmt.Sprintf("nm:session:%s", HashToken(token))
		var sess Session
		found, err := s.rdb.Get(ctx, key, &sess)
		if err != nil || !found {
			return nil, false
		}
		if time.Now().After(sess.ExpireAt) {
			_ = s.rdb.Del(ctx, key)
			return nil, false
		}
		return &sess, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.memory[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.ExpireAt) {
		delete(s.memory, token)
		return nil, false
	}
	return sess, true
}

func (s *SessionStore) Delete(token string) {
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		key := fmt.Sprintf("nm:session:%s", HashToken(token))
		_ = s.rdb.Del(ctx, key)
		return
	}
	s.mu.Lock()
	delete(s.memory, token)
	s.mu.Unlock()
}

// SetDefaultStore replaces the package-level default store.
func SetDefaultStore(store *SessionStore) {
	defaultStore = store
}

// Package-level helpers using the default store.
func SetSession(token string, sess *Session)   { defaultStore.Set(token, sess) }
func GetSession(token string) (*Session, bool) { return defaultStore.Get(token) }
func DeleteSession(token string)               { defaultStore.Delete(token) }

// HashToken returns a SHA-256 hex digest of a token string for DB storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
