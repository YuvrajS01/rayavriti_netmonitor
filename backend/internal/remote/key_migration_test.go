package remote

import "testing"

func TestStore_EncryptionKeyMigratesLegacyCredentials(t *testing.T) {
	const jwt = "legacy-jwt-secret-that-is-at-least-32-chars"
	legacyOnly, err := NewStore(nil, "", jwt)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := legacyOnly.encrypt("remote-api-key")
	if err != nil {
		t.Fatal(err)
	}

	withKey, err := NewStore(nil, "a-separate-encryption-key-of-32+-characters", jwt)
	if err != nil {
		t.Fatal(err)
	}
	plain, legacy, err := withKey.decrypt(sealed)
	if err != nil || plain != "remote-api-key" || !legacy {
		t.Fatalf("legacy credential: plain=%q legacy=%v err=%v", plain, legacy, err)
	}

	resealed, err := withKey.encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	plain, legacy, err = withKey.decrypt(resealed)
	if err != nil || plain != "remote-api-key" || legacy {
		t.Fatalf("re-encrypted credential: plain=%q legacy=%v err=%v", plain, legacy, err)
	}
	if _, _, err := legacyOnly.decrypt(resealed); err == nil {
		t.Fatal("credentials sealed with ENCRYPTION_KEY must not open with the JWT-derived key")
	}
}
