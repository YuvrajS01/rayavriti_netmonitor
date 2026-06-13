package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// HashAPIKey returns sha256 hex of the key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey creates a random 32-byte base64url key and its hash.
func GenerateAPIKey() (key, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	key = base64.RawURLEncoding.EncodeToString(b)
	hash = HashAPIKey(key)
	return
}
