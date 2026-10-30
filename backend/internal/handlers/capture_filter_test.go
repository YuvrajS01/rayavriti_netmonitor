package handlers

import "testing"

func TestIsValidBPFFilter(t *testing.T) {
	tests := []struct {
		filter string
		want   bool
	}{
		{"tcp port 80", true},
		{"host 10.0.0.1 and not port 22", true},
		{"-w/etc/cron.d/x", false},
		{"  -w /app/public/index.html", false},
		{"port 80; rm -rf /", false},
	}
	for _, tt := range tests {
		if got := isValidBPFFilter(tt.filter); got != tt.want {
			t.Errorf("isValidBPFFilter(%q) = %v, want %v", tt.filter, got, tt.want)
		}
	}
}
