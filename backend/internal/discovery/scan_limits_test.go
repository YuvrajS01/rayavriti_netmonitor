package discovery

import "testing"

func TestStartScanRequest_Limits(t *testing.T) {
	tests := []struct {
		subnet  string
		wantErr bool
	}{
		{"192.168.1.0/24", false},
		{"10.20.0.0/16", false},
		{"10.0.0.0/8", true},       // too large to expand safely
		{"0.0.0.0/0", true},        // would OOM the process
		{"fd00::/64", true},        // IPv6 sweeps are not supported
		{"127.0.0.0/24", true},     // loopback
		{"169.254.169.0/24", true}, // link-local / cloud metadata
	}
	for _, tt := range tests {
		r := &StartScanRequest{Subnet: tt.subnet}
		if err := r.Validate(); (err != nil) != tt.wantErr {
			t.Errorf("Validate(%s) error = %v, wantErr %v", tt.subnet, err, tt.wantErr)
		}
	}
}
