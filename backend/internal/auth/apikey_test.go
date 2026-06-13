package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	t.Parallel()
	key, hash, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key)
	assert.Len(t, hash, 64) // SHA-256 hex
	assert.Equal(t, HashAPIKey(key), hash)
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	t.Parallel()
	key1, hash1, err := GenerateAPIKey()
	require.NoError(t, err)
	key2, hash2, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, hash1, hash2)
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	t.Parallel()
	key := "test-api-key"
	h1 := HashAPIKey(key)
	h2 := HashAPIKey(key)
	assert.Equal(t, h1, h2)
}

func TestHashAPIKey_KnownVector(t *testing.T) {
	t.Parallel()
	expected := sha256.Sum256([]byte("test-key"))
	expectedHex := hex.EncodeToString(expected[:])
	assert.Equal(t, expectedHex, HashAPIKey("test-key"))
}
