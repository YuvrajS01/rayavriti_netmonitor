// Package flowtest builds NetFlow v5/v9, IPFIX and sFlow v5 datagrams for
// tests and local end-to-end checks of the flow ingest server.
package flowtest

import (
	"encoding/binary"
	"net/netip"
	"time"
)

var be = binary.BigEndian

// U8, U16, U32 and U64 encode big-endian field values.
func U8(v uint8) []byte   { return []byte{v} }
func U16(v uint16) []byte { return be.AppendUint16(nil, v) }
func U32(v uint32) []byte { return be.AppendUint32(nil, v) }
func U64(v uint64) []byte { return be.AppendUint64(nil, v) }

// IP4 encodes an IPv4 address.
func IP4(s string) []byte {
	a := netip.MustParseAddr(s).As4()
	return a[:]
}

// Cat concatenates byte slices (one data record from its field values).
func Cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ── NetFlow v5 ───────────────────────────────────────────────────────────────

// V5Record is one NetFlow v5 flow record. First and Last are router uptime
// in milliseconds.
type V5Record struct {
	Src, Dst, NextHop string
	Input, Output     uint16
	Packets, Bytes    uint32
	First, Last       uint32
	SrcPort, DstPort  uint16
	TCPFlags          uint8
	Proto             uint8
	ToS               uint8
}

// NetFlowV5 builds a v5 datagram. samplingInterval is the raw header field
// (mode in the top two bits, interval in the low 14).
func NetFlowV5(sysUptimeMs uint32, export time.Time, seq uint32, samplingInterval uint16, recs ...V5Record) []byte {
	b := Cat(U16(5), U16(uint16(len(recs))), U32(sysUptimeMs), //nolint:gosec // test builder
		U32(uint32(export.Unix())), U32(uint32(export.Nanosecond())), //nolint:gosec // test builder
		U32(seq), U8(0), U8(0), U16(samplingInterval))
	for _, r := range recs {
		nh := "0.0.0.0"
		if r.NextHop != "" {
			nh = r.NextHop
		}
		b = append(b, Cat(IP4(r.Src), IP4(r.Dst), IP4(nh), U16(r.Input), U16(r.Output),
			U32(r.Packets), U32(r.Bytes), U32(r.First), U32(r.Last), U16(r.SrcPort), U16(r.DstPort),
			U8(0), U8(r.TCPFlags), U8(r.Proto), U8(r.ToS), U16(0), U16(0), U8(24), U8(24), U16(0))...)
	}
	return b
}

// ── NetFlow v9 / IPFIX ───────────────────────────────────────────────────────

// Field is a template field: information element type and length.
type Field struct{ Type, Length uint16 }

func fields(fs []Field) []byte {
	var b []byte
	for _, f := range fs {
		b = append(b, Cat(U16(f.Type), U16(f.Length))...)
	}
	return b
}

// set wraps body in a flowset/set header, padded to a 4-byte boundary.
func set(id uint16, body []byte) []byte {
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	return Cat(U16(id), U16(uint16(4+len(body))), body) //nolint:gosec // test builder
}

// V9Template is a v9 template flowset (ID 0) with one template.
func V9Template(id uint16, fs ...Field) []byte {
	return set(0, Cat(U16(id), U16(uint16(len(fs))), fields(fs))) //nolint:gosec // test builder
}

// V9OptionsTemplate is a v9 options template flowset (ID 1).
func V9OptionsTemplate(id uint16, scope, opts []Field) []byte {
	return set(1, Cat(U16(id), U16(uint16(4*len(scope))), U16(uint16(4*len(opts))), fields(scope), fields(opts))) //nolint:gosec // test builder
}

// IPFIXTemplate is an IPFIX template set (ID 2) with one template.
func IPFIXTemplate(id uint16, fs ...Field) []byte {
	return set(2, Cat(U16(id), U16(uint16(len(fs))), fields(fs))) //nolint:gosec // test builder
}

// IPFIXOptionsTemplate is an IPFIX options template set (ID 3).
func IPFIXOptionsTemplate(id uint16, scope, opts []Field) []byte {
	return set(3, Cat(U16(id), U16(uint16(len(scope)+len(opts))), U16(uint16(len(scope))), fields(scope), fields(opts))) //nolint:gosec // test builder
}

// DataSet is a data flowset for templateID holding the given records.
func DataSet(templateID uint16, records ...[]byte) []byte {
	return set(templateID, Cat(records...))
}

// NetFlowV9 builds a v9 datagram; count is the header's record count.
func NetFlowV9(sysUptimeMs uint32, export time.Time, seq, sourceID uint32, count uint16, sets ...[]byte) []byte {
	return Cat(U16(9), U16(count), U32(sysUptimeMs), U32(uint32(export.Unix())), U32(seq), U32(sourceID), Cat(sets...)) //nolint:gosec // test builder
}

// IPFIX builds an IPFIX message.
func IPFIX(export time.Time, seq, domain uint32, sets ...[]byte) []byte {
	body := Cat(sets...)
	return Cat(U16(10), U16(uint16(16+len(body))), U32(uint32(export.Unix())), U32(seq), U32(domain), body) //nolint:gosec // test builder
}

// ── sFlow v5 ─────────────────────────────────────────────────────────────────

// SFlowSample is a flow sample carrying one raw-packet-header record.
type SFlowSample struct {
	SamplingRate  uint32
	Input, Output uint32
	FrameLength   uint32
	Header        []byte // sampled frame, from the Ethernet header
}

// SFlowV5 builds an sFlow v5 datagram from an IPv4 agent.
func SFlowV5(agent string, seq, uptimeMs uint32, samples ...SFlowSample) []byte {
	b := Cat(U32(5), U32(1), IP4(agent), U32(0), U32(seq), U32(uptimeMs), U32(uint32(len(samples)))) //nolint:gosec // test builder
	for i, s := range samples {
		hdr := s.Header
		hlen := len(hdr)
		for len(hdr)%4 != 0 {
			hdr = append(hdr, 0)
		}
		// Raw packet header record (format 1): protocol 1 = Ethernet.
		rec := Cat(U32(1), U32(s.FrameLength), U32(4), U32(uint32(hlen)), hdr)                                     //nolint:gosec // test builder
		body := Cat(U32(uint32(i+1)), U32(s.Input&0xffffff), U32(s.SamplingRate), U32(s.SamplingRate*uint32(i+1)), //nolint:gosec // test builder
			U32(0), U32(s.Input), U32(s.Output), U32(1),
			U32(1), U32(uint32(len(rec))), rec) //nolint:gosec // test builder
		b = append(b, Cat(U32(1), U32(uint32(len(body))), body)...) //nolint:gosec // test builder
	}
	return b
}

// EthIPv4TCP returns Ethernet + IPv4 + TCP headers for a sampled packet.
func EthIPv4TCP(src, dst string, sport, dport uint16, flags, tos uint8, totalLen uint16) []byte {
	eth := Cat([]byte{0, 1, 2, 3, 4, 5}, []byte{6, 7, 8, 9, 10, 11}, U16(0x0800))
	ip := Cat(U8(0x45), U8(tos), U16(totalLen), U16(1), U16(0x4000), U8(64), U8(6), U16(0), IP4(src), IP4(dst))
	tcp := Cat(U16(sport), U16(dport), U32(1), U32(0), U8(0x50), U8(flags), U16(65535), U16(0), U16(0))
	return Cat(eth, ip, tcp)
}
