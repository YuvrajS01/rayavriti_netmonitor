package collectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSNMPCollector_Name(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	assert.Equal(t, "snmp", c.Name())
}

func TestSNMPCollector_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ Collector = SNMPCollector{}
}

func TestSNMPCollector_NameConsistency(t *testing.T) {
	t.Parallel()
	c := SNMPCollector{}
	for i := 0; i < 10; i++ {
		assert.Equal(t, "snmp", c.Name())
	}
}

func TestProtoNameFromNum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int
		expected string
	}{
		{1, "ICMP"},
		{2, "IGMP"},
		{6, "TCP"},
		{17, "UDP"},
		{47, "GRE"},
		{50, "ESP"},
		{51, "AH"},
		{58, "ICMPv6"},
		{89, "OSPF"},
		{132, "SCTP"},
		{999, "PROTO_999"},
		{0, "PROTO_0"},
		{-1, "PROTO_-1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			result := protoNameFromNum(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNetflowProtoName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    int
		expected string
	}{
		{6, "TCP"},
		{17, "UDP"},
		{1, "ICMP"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			result := GetNetflowProtoName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeCounter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		expected int64
	}{
		{"nil", nil, 0},
		{"uint64", uint64(42), 42},
		{"uint32", uint32(100), 100},
		{"int", int(7), 7},
		{"int64", int64(-5), -5},
		{"empty_bytes", []byte{}, 0},
		{"single_byte", []byte{0xFF}, 255},
		{"two_bytes", []byte{0x01, 0x00}, 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := NormalizeCounter(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
