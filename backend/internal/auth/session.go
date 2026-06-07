package auth

import (
	"sync"
	"time"
)

// Session holds minimal data about an authenticated user session.
type Session struct {
	UserID   int64
	Username string
	Role     string
	ExpireAt time.Time
}

// SessionStore is an in-memory store keyed by refresh token.
type SessionStore struct {
	mu      sync.RWMutex
	entries map[string]*Session
}

var defaultStore = &SessionStore{entries: map[string]*Session{}}

func NewSessionStore() *SessionStore {
	return &SessionStore{entries: map[string]*Session{}}
}

func (s *SessionStore) Set(token string, sess *Session) {
	s.mu.Lock()
	s.entries[token] = sess
	s.mu.Unlock()
}

func (s *SessionStore) Get(token string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.entries[token]
	if !ok || time.Now().After(sess.ExpireAt) {
		return nil, false
	}
	return sess, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.entries, token)
	s.mu.Unlock()
}

// Package-level helpers using the default store.
func SetSession(token string, sess *Session)        { defaultStore.Set(token, sess) }
func GetSession(token string) (*Session, bool)      { return defaultStore.Get(token) }
func DeleteSession(token string)                    { defaultStore.Delete(token) }
