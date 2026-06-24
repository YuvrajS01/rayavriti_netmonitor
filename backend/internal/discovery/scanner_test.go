package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandCIDR(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		wantCount int
		wantFirst string
		wantLast  string
		wantErr   bool
	}{
		{
			name:      "/30 network",
			cidr:      "192.168.1.0/30",
			wantCount: 2,
			wantFirst: "192.168.1.1",
			wantLast:  "192.168.1.2",
		},
		{
			name:      "/29 network",
			cidr:      "10.0.0.0/29",
			wantCount: 6,
			wantFirst: "10.0.0.1",
			wantLast:  "10.0.0.6",
		},
		{
			name:      "/28 network",
			cidr:      "172.16.0.0/28",
			wantCount: 14,
			wantFirst: "172.16.0.1",
			wantLast:  "172.16.0.14",
		},
		{
			name:    "invalid CIDR",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
		{
			name:    "empty string",
			cidr:    "",
			wantErr: true,
		},
		{
			name:      "/31 network",
			cidr:      "192.168.1.0/31",
			wantCount: 2,
			wantFirst: "192.168.1.0",
			wantLast:  "192.168.1.1",
		},
		{
			name:      "/32 host",
			cidr:      "192.168.1.1/32",
			wantCount: 1,
			wantFirst: "192.168.1.1",
			wantLast:  "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := expandCIDR(tt.cidr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(ips), "unexpected IP count")
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirst, ips[0], "first IP mismatch")
				assert.Equal(t, tt.wantLast, ips[len(ips)-1], "last IP mismatch")
			}
		})
	}
}

func TestExpandCIDR_AllUnique(t *testing.T) {
	ips, err := expandCIDR("192.168.1.0/28")
	require.NoError(t, err)
	seen := make(map[string]bool)
	for _, ip := range ips {
		assert.False(t, seen[ip], "duplicate IP: %s", ip)
		seen[ip] = true
	}
}

func TestExpandCIDR_Sequential(t *testing.T) {
	ips, err := expandCIDR("10.10.10.0/30")
	require.NoError(t, err)
	require.Len(t, ips, 2)
	assert.Equal(t, "10.10.10.1", ips[0])
	assert.Equal(t, "10.10.10.2", ips[1])
}

func TestOUILookup(t *testing.T) {
	tests := []struct {
		name    string
		mac     string
		wantMfr string
	}{
		{
			name:    "Cisco prefix",
			mac:     "00:1A:2B:3C:4D:5E",
			wantMfr: "Cisco",
		},
		{
			name:    "VMware prefix",
			mac:     "00:50:56:12:34:56",
			wantMfr: "VMware",
		},
		{
			name:    "Dell prefix",
			mac:     "00:1E:65:AA:BB:CC",
			wantMfr: "Dell",
		},
		{
			name:    "HP prefix",
			mac:     "00:0E:7F:11:22:33",
			wantMfr: "HP",
		},
		{
			name:    "Intel prefix",
			mac:     "00:1B:21:44:55:66",
			wantMfr: "Intel",
		},
		{
			name:    "Juniper prefix",
			mac:     "00:1B:54:77:88:99",
			wantMfr: "Juniper",
		},
		{
			name:    "Hyper-V prefix",
			mac:     "00:15:5D:AA:BB:CC",
			wantMfr: "Microsoft",
		},
		{
			name:    "QEMU/KVM prefix",
			mac:     "52:54:00:12:34:56",
			wantMfr: "QEMU/KVM",
		},
		{
			name:    "VirtualBox prefix",
			mac:     "08:00:27:12:34:56",
			wantMfr: "VirtualBox",
		},
		{
			name:    "Xen prefix",
			mac:     "00:16:3E:12:34:56",
			wantMfr: "Xen",
		},
		{
			name:    "Raspberry Pi",
			mac:     "B8:27:EB:12:34:56",
			wantMfr: "Raspberry Pi",
		},
		{
			name:    "Belkin prefix",
			mac:     "30:23:03:12:34:56",
			wantMfr: "Belkin",
		},
		{
			name:    "Linksys prefix",
			mac:     "00:04:5A:12:34:56",
			wantMfr: "Linksys",
		},
		{
			name:    "unknown prefix",
			mac:     "FF:FF:FF:12:34:56",
			wantMfr: "",
		},
		{
			name:    "short MAC",
			mac:     "00:1A",
			wantMfr: "",
		},
		{
			name:    "empty MAC",
			mac:     "",
			wantMfr: "",
		},
		{
			name:    "lowercase input",
			mac:     "00:1a:2b:3c:4d:5e",
			wantMfr: "Cisco",
		},
		{
			name:    "mixed case input",
			mac:     "00:50:56:AB:CD:EF",
			wantMfr: "VMware",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ouiLookup(tt.mac)
			assert.Equal(t, tt.wantMfr, got)
		})
	}
}

func TestGuessCategoryFromPorts(t *testing.T) {
	tests := []struct {
		name         string
		ports        []int
		wantCategory string
	}{
		{
			name:         "printer with 9100",
			ports:        []int{80, 9100},
			wantCategory: "printer",
		},
		{
			name:         "printer with 515",
			ports:        []int{515},
			wantCategory: "printer",
		},
		{
			name:         "printer with both 515 and 9100",
			ports:        []int{515, 9100, 80, 161},
			wantCategory: "printer",
		},
		{
			name:         "camera with RTSP",
			ports:        []int{80, 554},
			wantCategory: "camera",
		},
		{
			name:         "workstation with RDP",
			ports:        []int{3389, 135, 445},
			wantCategory: "workstation",
		},
		{
			name:         "managed switch with SNMP and HTTP",
			ports:        []int{22, 80, 161},
			wantCategory: "managed_switch",
		},
		{
			name:         "managed switch with SNMP and SSH",
			ports:        []int{22, 161, 443},
			wantCategory: "managed_switch",
		},
		{
			name:         "SNMP device only",
			ports:        []int{161},
			wantCategory: "snmp_device",
		},
		{
			name:         "server with SSH and HTTP",
			ports:        []int{22, 80, 443},
			wantCategory: "server",
		},
		{
			name:         "server with SSH only",
			ports:        []int{22},
			wantCategory: "server",
		},
		{
			name:         "server with HTTP only",
			ports:        []int{80},
			wantCategory: "server",
		},
		{
			name:         "server with HTTPS only",
			ports:        []int{443},
			wantCategory: "server",
		},
		{
			name:         "server with FTP",
			ports:        []int{21},
			wantCategory: "server",
		},
		{
			name:         "server with Telnet",
			ports:        []int{23},
			wantCategory: "server",
		},
		{
			name:         "server with DNS",
			ports:        []int{53},
			wantCategory: "server",
		},
		{
			name:         "unknown - no common ports",
			ports:        []int{12345, 67890},
			wantCategory: "unknown",
		},
		{
			name:         "empty ports",
			ports:        []int{},
			wantCategory: "unknown",
		},
		{
			name:         "nil ports",
			ports:        nil,
			wantCategory: "unknown",
		},
		{
			name:         "printer takes priority over server",
			ports:        []int{22, 80, 9100, 161},
			wantCategory: "printer",
		},
		{
			name:         "camera takes priority over server",
			ports:        []int{22, 80, 554},
			wantCategory: "camera",
		},
		{
			name:         "workstation takes priority over server",
			ports:        []int{22, 80, 3389},
			wantCategory: "workstation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := guessCategoryFromPorts(tt.ports)
			assert.Equal(t, tt.wantCategory, got)
		})
	}
}

func TestExcludeIPs(t *testing.T) {
	all := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5"}
	known := map[string]bool{
		"10.0.0.2": true,
		"10.0.0.4": true,
	}
	result := excludeIPs(all, known)
	expected := []string{"10.0.0.1", "10.0.0.3", "10.0.0.5"}
	assert.Equal(t, expected, result)
}

func TestExcludeIPs_EmptyKnown(t *testing.T) {
	all := []string{"10.0.0.1", "10.0.0.2"}
	known := map[string]bool{}
	result := excludeIPs(all, known)
	assert.Equal(t, all, result)
}

func TestExcludeIPs_AllKnown(t *testing.T) {
	all := []string{"10.0.0.1", "10.0.0.2"}
	known := map[string]bool{
		"10.0.0.1": true,
		"10.0.0.2": true,
	}
	result := excludeIPs(all, known)
	assert.Empty(t, result)
}

func TestExcludeIPs_AllExcluded(t *testing.T) {
	all := []string{"10.0.0.1"}
	known := map[string]bool{"10.0.0.1": true}
	result := excludeIPs(all, known)
	assert.Empty(t, result)
}

func TestExcludeIPs_NoneExcluded(t *testing.T) {
	all := []string{"10.0.0.1", "10.0.0.2"}
	known := map[string]bool{"10.0.0.3": true}
	result := excludeIPs(all, known)
	assert.Equal(t, all, result)
}

func TestParsePingRTT(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   float64
	}{
		{
			name:   "Linux output",
			output: "64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time=1.23 ms",
			want:   1.23,
		},
		{
			name:   "no time field",
			output: "64 bytes from 10.0.0.1: icmp_seq=1 ttl=64",
			want:   0,
		},
		{
			name:   "empty string",
			output: "",
			want:   0,
		},
		{
			name:   "time with angle bracket",
			output: "64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time<1 ms",
			want:   1,
		},
		{
			name:   "integer time",
			output: "64 bytes from 10.0.0.1: icmp_seq=1 ttl=64 time=3 ms",
			want:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePingRTT(tt.output)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStartScanRequest_Validate(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name    string
		req     StartScanRequest
		wantErr bool
	}{
		{
			name: "valid full scan",
			req: StartScanRequest{
				Subnet:   "192.168.1.0/24",
				ScanType: "full",
			},
			wantErr: false,
		},
		{
			name: "valid ping_only",
			req: StartScanRequest{
				Subnet:   "10.0.0.0/16",
				ScanType: "ping_only",
			},
			wantErr: false,
		},
		{
			name: "valid ping_snmp",
			req: StartScanRequest{
				Subnet:   "172.16.0.0/24",
				ScanType: "ping_snmp",
			},
			wantErr: false,
		},
		{
			name: "defaults to full",
			req: StartScanRequest{
				Subnet: "192.168.1.0/24",
			},
			wantErr: false,
		},
		{
			name: "missing subnet",
			req: StartScanRequest{
				ScanType: "full",
			},
			wantErr: true,
		},
		{
			name: "invalid CIDR",
			req: StartScanRequest{
				Subnet:   "not-a-cidr",
				ScanType: "full",
			},
			wantErr: true,
		},
		{
			name: "invalid scan type",
			req: StartScanRequest{
				Subnet:   "192.168.1.0/24",
				ScanType: "aggressive",
			},
			wantErr: true,
		},
		{
			name: "valid with exclude known",
			req: StartScanRequest{
				Subnet:       "192.168.1.0/24",
				ScanType:     "full",
				ExcludeKnown: boolPtr(true),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.req.ScanType == "" {
					assert.Equal(t, "full", tt.req.ScanType, "scan type should default to full after validation")
				}
			}
		})
	}
}
