package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword_Produces_ScryptFormat(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("password123")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "scrypt:"))

	parts := strings.Split(hash, ":")
	assert.Len(t, parts, 3)
	// salt is 64 hex chars (32 bytes)
	assert.Len(t, parts[1], 64)
	_, err = hex.DecodeString(parts[1])
	require.NoError(t, err)
	// hash is 64 hex chars (32 bytes)
	assert.Len(t, parts[2], 64)
	_, err = hex.DecodeString(parts[2])
	require.NoError(t, err)
}

func TestHashPassword_Unique_Salts(t *testing.T) {
	t.Parallel()
	hash1, err := HashPassword("password123")
	require.NoError(t, err)
	hash2, err := HashPassword("password123")
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2)
}

func TestCheckPassword_Scrypt_Correct(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("password123")
	require.NoError(t, err)
	assert.True(t, CheckPassword("password123", hash))
}

func TestCheckPassword_Scrypt_Wrong(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("password123")
	require.NoError(t, err)
	assert.False(t, CheckPassword("wrong", hash))
}

func TestCheckPassword_SHA256_Legacy_Correct(t *testing.T) {
	t.Parallel()
	h := sha256.Sum256([]byte("admin"))
	stored := "sha256:" + hex.EncodeToString(h[:])
	assert.True(t, CheckPassword("admin", stored))
}

func TestCheckPassword_SHA256_Legacy_Wrong(t *testing.T) {
	t.Parallel()
	h := sha256.Sum256([]byte("admin"))
	stored := "sha256:" + hex.EncodeToString(h[:])
	assert.False(t, CheckPassword("wrong", stored))
}

func TestCheckPassword_InvalidFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		password string
		stored   string
	}{
		{"plaintext", "password", "plaintext"},
		{"empty", "password", ""},
		{"too few parts", "password", "scrypt:too:few"},
		{"not hex salt", "password", "scrypt:notHex:abcd"},
		{"not hex hash", "password", "scrypt:abcd:notHex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, CheckPassword(tt.password, tt.stored))
		})
	}
}
