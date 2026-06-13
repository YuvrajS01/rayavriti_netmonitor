package collectors

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildNetFlowV5Packet(count int) []byte {
	header := make([]byte, 24)
	binary.BigEndian.PutUint16(header[0:2], 5)
	binary.BigEndian.PutUint16(header[2:4], uint16(count))
	binary.BigEndian.PutUint32(header[8:12], 1000000)
	binary.BigEndian.PutUint32(header[16:20], 0)

	records := make([]byte, 0, count*48)
	for i := 0; i < count; i++ {
		rec := make([]byte, 48)
		rec[0] = 10
		rec[1] = 0
		rec[2] = 0
		rec[3] = byte(i + 1)
		rec[4] = 192
		rec[5] = 168
		rec[6] = 1
		rec[7] = 100
		binary.BigEndian.PutUint16(rec[32:34], uint16(1024+i))
		binary.BigEndian.PutUint16(rec[34:36], 80)
		rec[38] = 6
		binary.BigEndian.PutUint32(rec[20:24], uint32(1000*(i+1)))
		binary.BigEndian.PutUint32(rec[16:20], uint32(10*(i+1)))
		records = append(records, rec...)
	}
	return append(header, records...)
}

func TestParseNetFlowV5_ValidPacket(t *testing.T) {
	t.Parallel()
	data := buildNetFlowV5Packet(3)
	flows := parseNetFlowV5(data)
	assert.Len(t, flows, 3)
	assert.Equal(t, "10.0.0.1", flows[0].SrcIP)
	assert.Equal(t, "192.168.1.100", flows[0].DstIP)
	assert.Equal(t, 1024, flows[0].SrcPort)
	assert.Equal(t, 80, flows[0].DstPort)
	assert.Equal(t, "TCP", flows[0].Protocol)
	assert.Equal(t, int64(1000), flows[0].Bytes)
	assert.Equal(t, int64(10), flows[0].Packets)
}

func TestParseNetFlowV5_TooShort(t *testing.T) {
	t.Parallel()
	flows := parseNetFlowV5([]byte{0, 1, 2})
	assert.Nil(t, flows)
}

func TestParseNetFlowV5_WrongVersion(t *testing.T) {
	t.Parallel()
	data := make([]byte, 24)
	binary.BigEndian.PutUint16(data[0:2], 9)
	flows := parseNetFlowV5(data)
	assert.Nil(t, flows)
}

func TestParseNetFlowV5_TruncatedRecord(t *testing.T) {
	t.Parallel()
	data := buildNetFlowV5Packet(5)
	data = data[:120] // 24 + 2*48 = 120
	flows := parseNetFlowV5(data)
	assert.Len(t, flows, 2)
}

func TestParseNetFlowV5_ZeroCount(t *testing.T) {
	t.Parallel()
	data := buildNetFlowV5Packet(0)
	flows := parseNetFlowV5(data)
	assert.Empty(t, flows)
}

func TestProtoName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		proto    int
		expected string
	}{
		{6, "TCP"},
		{17, "UDP"},
		{1, "ICMP"},
		{99, "99"},
		{0, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, protoName(tt.proto))
		})
	}
}
