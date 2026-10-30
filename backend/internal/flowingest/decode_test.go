package flowingest

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ft "github.com/rayavriti/netmonitor-backend/internal/flowingest/flowtest"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

var (
	exporterA = netip.MustParseAddr("192.0.2.10")
	exporterB = netip.MustParseAddr("192.0.2.20")
	// recv is the receive time; export times are set relative to it.
	recv = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
)

func newTestDecoder(t *testing.T) *decoder {
	t.Helper()
	d, err := newDecoder(16, 7*24*time.Hour, func(a netip.Addr) (int64, bool) {
		if a == exporterA {
			return 42, true
		}
		return 0, false
	})
	require.NoError(t, err)
	return d
}

func reason(err error) string {
	var de *decodeError
	if errors.As(err, &de) {
		return de.reason
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

func TestDecode_NetFlowV5(t *testing.T) {
	d := newTestDecoder(t)
	// Exported 2s before receipt; router uptime 100s. The flow ran from
	// uptime 40s to 90s, i.e. 62s..12s before receipt.
	export := recv.Add(-2 * time.Second)
	pkt := ft.NetFlowV5(100_000, export, 7, 0x4000|100, ft.V5Record{
		Src: "10.0.0.1", Dst: "10.0.0.2", Input: 3, Output: 5,
		Packets: 10, Bytes: 1500, First: 40_000, Last: 90_000,
		SrcPort: 51000, DstPort: 443, TCPFlags: 0x1b, Proto: 6, ToS: 0x28,
	})

	proto, flows, err := d.decodeNetFlow(pkt, exporterA, recv)
	require.NoError(t, err)
	assert.Equal(t, protoNetFlowV5, proto)
	require.Len(t, flows, 1)
	f := flows[0]
	assert.Equal(t, models.Flow{
		Timestamp: export.Add(-10 * time.Second),
		SrcIP:     "10.0.0.1", DstIP: "10.0.0.2", SrcPort: 51000, DstPort: 443,
		Protocol: "TCP",
		Bytes:    150_000, // 1500 * sampling interval 100
		Packets:  1000,
		Duration: 50,
		Exporter: ptr("192.0.2.10"), DeviceID: ptr(int64(42)),
		InIf: ptr(3), OutIf: ptr(5), TCPFlags: ptr(0x1b), ToS: ptr(0x28),
		SamplingRate: ptr(100),
	}, f)
}

func TestDecode_NetFlowV5_Unsampled_UnknownExporter(t *testing.T) {
	d := newTestDecoder(t)
	pkt := ft.NetFlowV5(100_000, recv, 1, 0, ft.V5Record{
		Src: "10.0.0.1", Dst: "10.0.0.2", Packets: 2, Bytes: 120, First: 99_000, Last: 100_000, Proto: 17, SrcPort: 53, DstPort: 5353,
	})
	_, flows, err := d.decodeNetFlow(pkt, exporterB, recv)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	assert.Equal(t, int64(120), flows[0].Bytes)
	assert.Equal(t, 1, *flows[0].SamplingRate)
	assert.Equal(t, "UDP", flows[0].Protocol)
	assert.Nil(t, flows[0].TCPFlags, "TCP flags are only kept for TCP")
	assert.Nil(t, flows[0].DeviceID)
	assert.Equal(t, "192.0.2.20", *flows[0].Exporter)
}

func TestDecode_NetFlowV5_Truncated(t *testing.T) {
	d := newTestDecoder(t)
	pkt := ft.NetFlowV5(1, recv, 1, 0, ft.V5Record{Src: "10.0.0.1", Dst: "10.0.0.2"}, ft.V5Record{Src: "10.0.0.1", Dst: "10.0.0.2"})
	_, flows, err := d.decodeNetFlow(pkt[:len(pkt)-10], exporterA, recv)
	assert.Equal(t, reasonMalformed, reason(err))
	assert.Empty(t, flows)

	proto, _, err := d.decodeNetFlow([]byte{0, 7, 0, 0}, exporterA, recv)
	assert.Equal(t, protoUnknown, proto)
	assert.Equal(t, reasonMalformed, reason(err))
}

// v9 template used by the tests below.
var v9Fields = []ft.Field{
	{Type: 8, Length: 4},  // IPV4_SRC_ADDR
	{Type: 12, Length: 4}, // IPV4_DST_ADDR
	{Type: 7, Length: 2},  // L4_SRC_PORT
	{Type: 11, Length: 2}, // L4_DST_PORT
	{Type: 4, Length: 1},  // PROTOCOL
	{Type: 1, Length: 4},  // IN_BYTES
	{Type: 2, Length: 4},  // IN_PKTS
	{Type: 22, Length: 4}, // FIRST_SWITCHED
	{Type: 21, Length: 4}, // LAST_SWITCHED
	{Type: 10, Length: 2}, // INPUT_SNMP
	{Type: 14, Length: 2}, // OUTPUT_SNMP
	{Type: 6, Length: 1},  // TCP_FLAGS
	{Type: 5, Length: 1},  // SRC_TOS
}

func v9Record(first, last uint32) []byte {
	return ft.Cat(ft.IP4("10.1.1.1"), ft.IP4("10.2.2.2"), ft.U16(40000), ft.U16(22), ft.U8(6),
		ft.U32(2000), ft.U32(20), ft.U32(first), ft.U32(last), ft.U16(7), ft.U16(9), ft.U8(0x18), ft.U8(0xb8))
}

func TestDecode_NetFlowV9_TemplateThenData(t *testing.T) {
	d := newTestDecoder(t)
	const uptime = 500_000
	export := recv.Truncate(time.Second)

	// Datagram 1: template only.
	_, flows, err := d.decodeNetFlow(ft.NetFlowV9(uptime, export, 1, 3, 1, ft.V9Template(256, v9Fields...)), exporterA, recv)
	require.NoError(t, err)
	assert.Empty(t, flows)

	// Datagram 2: data only, decoded with the stored template.
	pkt := ft.NetFlowV9(uptime, export, 2, 3, 1, ft.DataSet(256, v9Record(uptime-30_000, uptime-5_000)))
	proto, flows, err := d.decodeNetFlow(pkt, exporterA, recv)
	require.NoError(t, err)
	assert.Equal(t, protoNetFlowV9, proto)
	require.Len(t, flows, 1)
	f := flows[0]
	assert.Equal(t, export.Add(-5*time.Second), f.Timestamp)
	assert.InDelta(t, 25.0, f.Duration, 1e-9)
	assert.Equal(t, "10.1.1.1", f.SrcIP)
	assert.Equal(t, "10.2.2.2", f.DstIP)
	assert.Equal(t, 40000, f.SrcPort)
	assert.Equal(t, 22, f.DstPort)
	assert.Equal(t, "TCP", f.Protocol)
	assert.Equal(t, int64(2000), f.Bytes)
	assert.Equal(t, int64(20), f.Packets)
	assert.Equal(t, 7, *f.InIf)
	assert.Equal(t, 9, *f.OutIf)
	assert.Equal(t, 0x18, *f.TCPFlags)
	assert.Equal(t, 0xb8, *f.ToS)
	assert.Equal(t, 1, *f.SamplingRate)
	assert.Equal(t, int64(42), *f.DeviceID)

	// Template state is per exporter: the same data from another exporter
	// has no template.
	_, flows, err = d.decodeNetFlow(pkt, exporterB, recv)
	assert.Equal(t, reasonTemplateMissing, reason(err))
	assert.Empty(t, flows)

	// ...and per source ID within an exporter.
	other := ft.NetFlowV9(uptime, export, 3, 4, 1, ft.DataSet(256, v9Record(uptime-30_000, uptime-5_000)))
	_, _, err = d.decodeNetFlow(other, exporterA, recv)
	assert.Equal(t, reasonTemplateMissing, reason(err))
}

func TestDecode_NetFlowV9_OptionsSamplingRate(t *testing.T) {
	d := newTestDecoder(t)
	const uptime = 500_000
	export := recv.Truncate(time.Second)
	// Options template 257: scope System (1, 4 bytes), option SAMPLING_INTERVAL.
	opts := ft.V9OptionsTemplate(257, []ft.Field{{Type: 1, Length: 4}}, []ft.Field{{Type: 34, Length: 4}})
	optData := ft.DataSet(257, ft.Cat(ft.U32(0), ft.U32(512)))
	_, _, err := d.decodeNetFlow(ft.NetFlowV9(uptime, export, 1, 0, 3, ft.V9Template(256, v9Fields...), opts, optData), exporterA, recv)
	require.NoError(t, err)

	_, flows, err := d.decodeNetFlow(ft.NetFlowV9(uptime, export, 2, 0, 1, ft.DataSet(256, v9Record(uptime-1000, uptime-1000))), exporterA, recv)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	assert.Equal(t, 512, *flows[0].SamplingRate)
	assert.Equal(t, int64(2000*512), flows[0].Bytes)
	assert.Equal(t, int64(20*512), flows[0].Packets)
}

func TestDecode_NetFlowV9_RecordSamplingRate(t *testing.T) {
	d := newTestDecoder(t)
	const uptime = 500_000
	export := recv.Truncate(time.Second)
	fields := append(append([]ft.Field{}, v9Fields...), ft.Field{Type: 34, Length: 4})
	rec := ft.Cat(v9Record(uptime-1000, uptime-1000), ft.U32(64))
	pkt := ft.NetFlowV9(uptime, export, 1, 0, 2, ft.V9Template(256, fields...), ft.DataSet(256, rec, rec))
	_, flows, err := d.decodeNetFlow(pkt, exporterA, recv)
	require.NoError(t, err)
	require.Len(t, flows, 2)
	for _, f := range flows {
		assert.Equal(t, 64, *f.SamplingRate)
		assert.Equal(t, int64(2000*64), f.Bytes)
	}
}

var ipfixFields = []ft.Field{
	{Type: 8, Length: 4},   // sourceIPv4Address
	{Type: 12, Length: 4},  // destinationIPv4Address
	{Type: 7, Length: 2},   // sourceTransportPort
	{Type: 11, Length: 2},  // destinationTransportPort
	{Type: 4, Length: 1},   // protocolIdentifier
	{Type: 1, Length: 8},   // octetDeltaCount
	{Type: 2, Length: 8},   // packetDeltaCount
	{Type: 152, Length: 8}, // flowStartMilliseconds
	{Type: 153, Length: 8}, // flowEndMilliseconds
	{Type: 10, Length: 4},  // ingressInterface
	{Type: 14, Length: 4},  // egressInterface
	{Type: 6, Length: 2},   // tcpControlBits
	{Type: 5, Length: 1},   // ipClassOfService
	{Type: 305, Length: 4}, // samplingPacketInterval
}

func ipfixRecord(start, end time.Time, rate uint32) []byte {
	return ft.Cat(ft.IP4("172.16.0.5"), ft.IP4("8.8.8.8"), ft.U16(33333), ft.U16(443), ft.U8(6),
		ft.U64(9000), ft.U64(6), ft.U64(uint64(start.UnixMilli())), ft.U64(uint64(end.UnixMilli())),
		ft.U32(11), ft.U32(12), ft.U16(0x02), ft.U8(0x20), ft.U32(rate))
}

func TestDecode_IPFIX_TemplateThenData(t *testing.T) {
	d := newTestDecoder(t)
	export := recv.Truncate(time.Second)
	_, flows, err := d.decodeNetFlow(ft.IPFIX(export, 1, 99, ft.IPFIXTemplate(300, ipfixFields...)), exporterA, recv)
	require.NoError(t, err)
	assert.Empty(t, flows)

	start, end := recv.Add(-30*time.Second), recv.Add(-5*time.Second)
	proto, flows, err := d.decodeNetFlow(ft.IPFIX(export, 2, 99, ft.DataSet(300, ipfixRecord(start, end, 10))), exporterA, recv)
	require.NoError(t, err)
	assert.Equal(t, protoIPFIX, proto)
	require.Len(t, flows, 1)
	assert.Equal(t, models.Flow{
		Timestamp: end,
		SrcIP:     "172.16.0.5", DstIP: "8.8.8.8", SrcPort: 33333, DstPort: 443,
		Protocol: "TCP", Bytes: 90_000, Packets: 60, Duration: 25,
		Exporter: ptr("192.0.2.10"), DeviceID: ptr(int64(42)),
		InIf: ptr(11), OutIf: ptr(12), TCPFlags: ptr(0x02), ToS: ptr(0x20),
		SamplingRate: ptr(10),
	}, flows[0])

	// Observation domain 100 has no template.
	_, _, err = d.decodeNetFlow(ft.IPFIX(export, 3, 100, ft.DataSet(300, ipfixRecord(start, end, 10))), exporterA, recv)
	assert.Equal(t, reasonTemplateMissing, reason(err))
}

func TestDecode_IPFIX_ImplausibleTimes(t *testing.T) {
	d := newTestDecoder(t)
	export := recv.Truncate(time.Second)
	_, _, err := d.decodeNetFlow(ft.IPFIX(export, 1, 1, ft.IPFIXTemplate(300, ipfixFields...)), exporterA, recv)
	require.NoError(t, err)

	cases := map[string][2]time.Time{
		"future":   {recv.Add(2 * time.Hour), recv.Add(3 * time.Hour)},
		"too old":  {recv.Add(-30 * 24 * time.Hour), recv.Add(-29 * 24 * time.Hour)},
		"epoch":    {time.Unix(0, 0), time.Unix(0, 0)},
		"inverted": {recv.Add(-time.Second), recv.Add(-time.Minute)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, flows, err := d.decodeNetFlow(ft.IPFIX(export, 2, 1, ft.DataSet(300, ipfixRecord(c[0], c[1], 0))), exporterA, recv)
			require.NoError(t, err)
			require.Len(t, flows, 1)
			if name == "inverted" {
				assert.Equal(t, c[1], flows[0].Timestamp, "end time is plausible")
			} else {
				assert.Equal(t, recv, flows[0].Timestamp, "falls back to receive time")
			}
			assert.Zero(t, flows[0].Duration)
			assert.Equal(t, 1, *flows[0].SamplingRate)
		})
	}
}

func TestDecode_IPFIX_OptionsSamplingRate(t *testing.T) {
	d := newTestDecoder(t)
	export := recv.Truncate(time.Second)
	fields := ipfixFields[:len(ipfixFields)-1] // no per-record rate
	opts := ft.IPFIXOptionsTemplate(301, []ft.Field{{Type: 149, Length: 4}}, []ft.Field{{Type: 305, Length: 4}})
	optData := ft.DataSet(301, ft.Cat(ft.U32(7), ft.U32(2048)))
	_, _, err := d.decodeNetFlow(ft.IPFIX(export, 1, 7, ft.IPFIXTemplate(300, fields...), opts, optData), exporterA, recv)
	require.NoError(t, err)

	rec := ipfixRecord(recv.Add(-time.Second), recv, 0)
	rec = rec[:len(rec)-4]
	_, flows, err := d.decodeNetFlow(ft.IPFIX(export, 2, 7, ft.DataSet(300, rec)), exporterA, recv)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	assert.Equal(t, 2048, *flows[0].SamplingRate)
	assert.Equal(t, int64(9000*2048), flows[0].Bytes)
}

func TestDecode_SFlowFlowSample(t *testing.T) {
	d := newTestDecoder(t)
	pkt := ft.SFlowV5("192.0.2.10", 1, 1000, ft.SFlowSample{
		SamplingRate: 1000, Input: 3, Output: 7, FrameLength: 1514,
		Header: ft.EthIPv4TCP("10.9.0.1", "10.9.0.2", 44321, 80, 0x12, 0xb8, 1500),
	})
	// The agent address in the datagram, not the UDP source, is the exporter.
	flows, err := d.decodeSFlow(pkt, netip.MustParseAddr("198.51.100.1"), recv)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	assert.Equal(t, models.Flow{
		Timestamp: recv,
		SrcIP:     "10.9.0.1", DstIP: "10.9.0.2", SrcPort: 44321, DstPort: 80,
		Protocol: "TCP", Bytes: 1_514_000, Packets: 1000,
		Exporter: ptr("192.0.2.10"), DeviceID: ptr(int64(42)),
		InIf: ptr(3), OutIf: ptr(7), TCPFlags: ptr(0x12), ToS: ptr(0xb8),
		SamplingRate: ptr(1000),
	}, flows[0])
}

func TestDecode_SFlowDiscardedOutput(t *testing.T) {
	d := newTestDecoder(t)
	pkt := ft.SFlowV5("192.0.2.10", 1, 1000, ft.SFlowSample{
		SamplingRate: 1, Input: 3, Output: 0x40000000 | 2, FrameLength: 64,
		Header: ft.EthIPv4TCP("10.9.0.1", "10.9.0.2", 1, 2, 0, 0, 40),
	})
	flows, err := d.decodeSFlow(pkt, exporterA, recv)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	assert.Nil(t, flows[0].OutIf, "discard format carries no ifIndex")
}

func TestDecode_SFlowExpandedSampleRecordCap(t *testing.T) {
	d := newTestDecoder(t)
	// One expanded flow sample (format 3) advertising 2^31 records.
	body := ft.Cat(ft.U32(1), ft.U32(0), ft.U32(1), ft.U32(1), ft.U32(1), ft.U32(0),
		ft.U32(0), ft.U32(1), ft.U32(0), ft.U32(2), ft.U32(1<<31))
	pkt := ft.Cat(ft.U32(5), ft.U32(1), ft.IP4("192.0.2.10"), ft.U32(0), ft.U32(1), ft.U32(1), ft.U32(1),
		ft.U32(3), ft.U32(uint32(len(body))), body)
	flows, err := d.decodeSFlow(pkt, exporterA, recv)
	assert.Equal(t, reasonMalformed, reason(err))
	assert.Empty(t, flows)
}

func TestDecode_ExporterLimit(t *testing.T) {
	d, err := newDecoder(1, time.Hour, nil)
	require.NoError(t, err)
	tmpl := ft.NetFlowV9(1, recv, 1, 0, 1, ft.V9Template(256, v9Fields...))
	_, _, err = d.decodeNetFlow(tmpl, exporterA, recv)
	require.NoError(t, err)
	_, _, err = d.decodeNetFlow(tmpl, exporterB, recv)
	assert.Equal(t, reasonExporterLimit, reason(err))

	// Once the first exporter has been idle past the TTL it is evicted.
	_, _, err = d.decodeNetFlow(tmpl, exporterB, recv.Add(2*exporterIdleTTL))
	require.NoError(t, err)
	assert.Equal(t, 1, d.exporterCount())
}

func TestBoundedTemplates(t *testing.T) {
	d := newTestDecoder(t)
	st, err := d.exporter(exporterA, recv)
	require.NoError(t, err)
	for i := 0; i < maxTemplatesPerExporter; i++ {
		require.NoError(t, st.templates.AddTemplate(9, uint32(i), 256, nil))
	}
	assert.Error(t, st.templates.AddTemplate(9, 99999, 256, nil))
	assert.NoError(t, st.templates.AddTemplate(9, 0, 256, nil), "replacing an existing template is allowed")
	_, err = st.templates.RemoveTemplate(9, 0, 256)
	require.NoError(t, err)
	assert.NoError(t, st.templates.AddTemplate(9, 99999, 256, nil))
}
