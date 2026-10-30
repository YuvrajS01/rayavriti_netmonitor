package config

import "testing"

func TestValidateSecrets(t *testing.T) {
	long := "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name    string
		env     string
		jwt     string
		enc     string
		wantErr bool
	}{
		{"production short jwt rejected", "production", "short", "", true},
		{"production long jwt ok", "production", long, "", false},
		{"development short jwt only warns", "development", "short", "", false},
		{"short encryption key rejected", "development", long, "short", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{App: AppConfig{AppEnv: tt.env}, Auth: AuthConfig{JWTSecret: tt.jwt, EncryptionKey: tt.enc}}
			if err := validateSecrets(cfg); (err != nil) != tt.wantErr {
				t.Fatalf("validateSecrets() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
