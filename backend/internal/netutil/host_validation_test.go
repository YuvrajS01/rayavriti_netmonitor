package netutil

import (
	"testing"
)

func TestValidateHost_IPLiteral(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		policy  HostPolicy
		wantErr bool
	}{
		{"public IP allowed", "8.8.8.8", DefaultHostPolicy(), false},
		{"loopback blocked", "127.0.0.1", DefaultHostPolicy(), true},
		{"loopback IPv6 blocked", "::1", DefaultHostPolicy(), true},
		{"link-local blocked", "169.254.169.254", DefaultHostPolicy(), true},
		{"unspecified blocked", "0.0.0.0", DefaultHostPolicy(), true},
		{"private allowed by default", "10.0.0.1", DefaultHostPolicy(), false},
		{"private blocked by strict", "10.0.0.1", StrictHostPolicy(), true},
		{"RFC1918 172.16 blocked by strict", "172.16.0.1", StrictHostPolicy(), true},
		{"RFC1918 192.168 blocked by strict", "192.168.1.1", StrictHostPolicy(), true},
		{"public allowed by strict", "8.8.8.8", StrictHostPolicy(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHost(tt.host, tt.policy)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHost(%q) error = %v, wantErr %v", tt.host, err, tt.wantErr)
			}
		})
	}
}

func TestValidateHost_HostPort(t *testing.T) {
	// Port should be stripped before validation.
	if err := ValidateHost("127.0.0.1:8080", DefaultHostPolicy()); err == nil {
		t.Fatal("expected error for loopback with port")
	}
	if err := ValidateHost("8.8.8.8:443", DefaultHostPolicy()); err != nil {
		t.Fatalf("expected no error for public IP with port, got %v", err)
	}
}

func TestValidateHost_Empty(t *testing.T) {
	if err := ValidateHost("", DefaultHostPolicy()); err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		policy  HostPolicy
		wantErr bool
	}{
		{"valid https", "https://example.com/path", DefaultHostPolicy(), false},
		{"valid http", "http://example.com", DefaultHostPolicy(), false},
		{"metadata endpoint blocked", "http://169.254.169.254/latest/meta-data/", DefaultHostPolicy(), true},
		{"no scheme", "example.com", DefaultHostPolicy(), true},
		{"ftp scheme", "ftp://example.com", DefaultHostPolicy(), true},
		{"no host", "https:///path", DefaultHostPolicy(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url, tt.policy)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestCloudMetadataBlocked(t *testing.T) {
	// The specific AWS/GCP/Azure metadata endpoint must always be blocked.
	if err := ValidateHost("169.254.169.254", DefaultHostPolicy()); err == nil {
		t.Fatal("cloud metadata endpoint 169.254.169.254 must be blocked")
	}
	if err := ValidateHost("169.254.170.2", DefaultHostPolicy()); err == nil {
		t.Fatal("link-local 169.254.170.2 must be blocked")
	}
}
