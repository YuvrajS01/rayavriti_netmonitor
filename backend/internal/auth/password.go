package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

const scryptN, scryptR, scryptP, scryptLen = 32768, 8, 1, 32

// HashPassword returns "scrypt:<salt_hex>:<hash_hex>"
func HashPassword(password string) (string, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptLen)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scrypt:%s:%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// CheckPassword verifies scrypt or legacy sha256 hashes.
func CheckPassword(password, stored string) bool {
	if strings.HasPrefix(stored, "scrypt:") {
		parts := strings.Split(stored, ":")
		if len(parts) != 3 {
			return false
		}
		salt, err := hex.DecodeString(parts[1])
		if err != nil {
			return false
		}
		want, err := hex.DecodeString(parts[2])
		if err != nil {
			return false
		}
		got, err := scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, scryptLen)
		if err != nil {
			return false
		}
		return hex.EncodeToString(got) == hex.EncodeToString(want)
	}
	// legacy sha256:<hex>
	if strings.HasPrefix(stored, "sha256:") {
		h := sha256.Sum256([]byte(password))
		return "sha256:"+hex.EncodeToString(h[:]) == stored
	}
	return false
}
