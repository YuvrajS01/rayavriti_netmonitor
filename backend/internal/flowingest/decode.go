package flowingest

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/netsampler/goflow2/v2/decoders/netflowlegacy"
	"github.com/netsampler/goflow2/v2/decoders/sflow"
	flowpb "github.com/netsampler/goflow2/v2/pb"
	"github.com/netsampler/goflow2/v2/producer"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// Protocol labels used in metrics.
const (
	protoNetFlowV5 = "netflow_v5"
	protoNetFlowV9 = "netflow_v9"
	protoIPFIX     = "ipfix"
	protoSFlow     = "sflow"
	protoUnknown   = "unknown"
)

// Decode-error reasons used in metrics.
const (
	reasonMalformed       = "malformed"
	reasonTemplateMissing = "template_missing"
	reasonExporterLimit   = "exporter_limit"
)

// maxFutureSkew is how far past the receive time a flow end time may be
// before it is treated as bogus (exporter clock wrong) and replaced.
const maxFutureSkew = time.Hour

// exporterIdleTTL is how long an exporter's template state is kept without
// traffic once the exporter table is full.
const exporterIdleTTL = time.Hour

// decodeError carries the metric reason for a failed datagram.
type decodeError struct {
	reason string
	err    error
}

func (e *decodeError) Error() string { return e.reason + ": " + e.err.Error() }
func (e *decodeError) Unwrap() error { return e.err }

// exporterState is the per-exporter v9/IPFIX state: templates (keyed inside
// by version + observation domain / source ID + template ID) and a producer,
// whose sampling-rate table is likewise keyed by version + observation
// domain. Keeping the producer per exporter lets eviction free both.
type exporterState struct {
	templates netflow.NetFlowTemplateSystem
	producer  producer.ProducerInterface
	lastSeen  time.Time
}

// maxTemplatesPerExporter bounds template memory per exporter: template IDs
// are 16-bit per observation domain and domains are 32-bit, so an exporter
// (or a spoofed source) could otherwise grow the table without limit.
const maxTemplatesPerExporter = 1024

// boundedTemplates is a template system that refuses new templates past
// maxTemplatesPerExporter (replacing an existing one is always allowed).
type boundedTemplates struct {
	netflow.NetFlowTemplateSystem
	mu    sync.Mutex
	count int
}

func (b *boundedTemplates) AddTemplate(version uint16, obsDomainID uint32, templateID uint16, template any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.GetTemplate(version, obsDomainID, templateID); err != nil {
		if b.count >= maxTemplatesPerExporter {
			return fmt.Errorf("template limit (%d) reached", maxTemplatesPerExporter)
		}
		b.count++
	}
	return b.NetFlowTemplateSystem.AddTemplate(version, obsDomainID, templateID, template)
}

func (b *boundedTemplates) RemoveTemplate(version uint16, obsDomainID uint32, templateID uint16) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, err := b.NetFlowTemplateSystem.RemoveTemplate(version, obsDomainID, templateID)
	if err == nil {
		b.count--
	}
	return t, err
}

// decoder turns raw datagrams into normalised flow records.
type decoder struct {
	cfg          protoproducer.ProtoProducerConfig
	stateless    producer.ProducerInterface // v5 and sFlow carry no session state
	maxExporters int
	maxAge       time.Duration
	deviceID     func(netip.Addr) (int64, bool)

	mu        sync.Mutex
	exporters map[netip.Addr]*exporterState
}

func newDecoder(maxExporters int, maxAge time.Duration, deviceID func(netip.Addr) (int64, bool)) (*decoder, error) {
	cfg, err := (&protoproducer.ProducerConfig{}).Compile()
	if err != nil {
		return nil, fmt.Errorf("flow producer config: %w", err)
	}
	p, err := protoproducer.CreateProtoProducer(cfg, protoproducer.CreateSamplingSystem)
	if err != nil {
		return nil, err
	}
	if deviceID == nil {
		deviceID = func(netip.Addr) (int64, bool) { return 0, false }
	}
	return &decoder{
		cfg:          cfg,
		stateless:    p,
		maxExporters: maxExporters,
		maxAge:       maxAge,
		deviceID:     deviceID,
		exporters:    make(map[netip.Addr]*exporterState),
	}, nil
}

// exporter returns (creating if needed) the template state for addr.
func (d *decoder) exporter(addr netip.Addr, now time.Time) (*exporterState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st, ok := d.exporters[addr]; ok {
		st.lastSeen = now
		return st, nil
	}
	if len(d.exporters) >= d.maxExporters {
		for a, st := range d.exporters {
			if now.Sub(st.lastSeen) > exporterIdleTTL {
				delete(d.exporters, a)
			}
		}
		if len(d.exporters) >= d.maxExporters {
			return nil, &decodeError{reasonExporterLimit, fmt.Errorf("exporter table full (%d)", d.maxExporters)}
		}
	}
	p, err := protoproducer.CreateProtoProducer(d.cfg, protoproducer.CreateSamplingSystem)
	if err != nil {
		return nil, err
	}
	st := &exporterState{templates: &boundedTemplates{NetFlowTemplateSystem: netflow.CreateTemplateSystem()}, producer: p, lastSeen: now}
	d.exporters[addr] = st
	return st, nil
}

// exporterCount reports how many exporters hold template state.
func (d *decoder) exporterCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.exporters)
}

// decodeNetFlow decodes a NetFlow v5, v9 or IPFIX datagram from src. It
// returns the protocol label, the flows decoded (possibly some, even with an
// error: a v9/IPFIX packet whose later sets fail still yields the earlier
// ones) and any decode error.
func (d *decoder) decodeNetFlow(data []byte, src netip.Addr, recv time.Time) (string, []models.Flow, error) {
	if len(data) < 2 {
		return protoUnknown, nil, &decodeError{reasonMalformed, errors.New("short datagram")}
	}
	args := &producer.ProduceArgs{
		Src:            netip.AddrPortFrom(src, 0),
		SamplerAddress: src,
		TimeReceived:   recv,
	}
	switch v := binary.BigEndian.Uint16(data); v {
	case 5:
		// The v5 decoder zero-fills records the datagram is too short for,
		// so check the advertised count against the length first.
		if len(data) < 24 {
			return protoNetFlowV5, nil, &decodeError{reasonMalformed, errors.New("short v5 header")}
		}
		if count := int(binary.BigEndian.Uint16(data[2:4])); len(data) < 24+count*48 {
			return protoNetFlowV5, nil, &decodeError{reasonMalformed, fmt.Errorf("v5 count %d exceeds datagram length %d", count, len(data))}
		}
		var pkt netflowlegacy.PacketNetFlowV5
		if err := netflowlegacy.DecodeMessageVersion(bytes.NewBuffer(data), &pkt); err != nil {
			return protoNetFlowV5, nil, &decodeError{reasonMalformed, err}
		}
		flows, err := d.produce(d.stateless, &pkt, args, src, recv, nil)
		return protoNetFlowV5, flows, err

	case 9, 10:
		proto := protoNetFlowV9
		if v == 10 {
			proto = protoIPFIX
		}
		st, err := d.exporter(src, recv)
		if err != nil {
			return proto, nil, err
		}
		var nfv9 netflow.NFv9Packet
		var ipfix netflow.IPFIXPacket
		decErr := netflow.DecodeMessageVersion(bytes.NewBuffer(data), st.templates, &nfv9, &ipfix)
		if decErr != nil {
			reason := reasonMalformed
			if errors.Is(decErr, netflow.ErrorTemplateNotFound) {
				reason = reasonTemplateMissing
			}
			decErr = &decodeError{reason, decErr}
		}
		var msg any = &nfv9
		sets := nfv9.FlowSets
		if v == 10 {
			msg, sets = &ipfix, ipfix.FlowSets
		}
		flows, err := d.produce(st.producer, msg, args, src, recv, recordSamplingRates(sets))
		if err != nil {
			return proto, flows, err
		}
		return proto, flows, decErr

	default:
		return protoUnknown, nil, &decodeError{reasonMalformed, fmt.Errorf("unsupported version %d", v)}
	}
}

// decodeSFlow decodes an sFlow v5 datagram. Counter samples produce no flows.
func (d *decoder) decodeSFlow(data []byte, src netip.Addr, recv time.Time) ([]models.Flow, error) {
	if err := checkSFlowRecordCounts(data); err != nil {
		return nil, &decodeError{reasonMalformed, err}
	}
	var pkt sflow.Packet
	if err := sflow.DecodeMessageVersion(bytes.NewBuffer(data), &pkt); err != nil {
		return nil, &decodeError{reasonMalformed, err}
	}
	args := &producer.ProduceArgs{
		Src:            netip.AddrPortFrom(src, 0),
		SamplerAddress: src,
		TimeReceived:   recv,
	}
	return d.produce(d.stateless, &pkt, args, src, recv, nil)
}

// produce runs a goflow2 producer and normalises its messages. perRecord,
// when it has one entry per message, supplies sampling rates carried in the
// data records themselves (v9/IPFIX), used when no options-template rate is
// known.
func (d *decoder) produce(p producer.ProducerInterface, msg any, args *producer.ProduceArgs, src netip.Addr, recv time.Time, perRecord []uint32) ([]models.Flow, error) {
	msgs, err := p.Produce(msg, args)
	defer p.Commit(msgs)
	if err != nil {
		return nil, &decodeError{reasonMalformed, err}
	}
	if len(perRecord) != len(msgs) {
		perRecord = nil
	}
	flows := make([]models.Flow, 0, len(msgs))
	for i, m := range msgs {
		fm, ok := m.(*protoproducer.ProtoProducerMessage)
		if !ok {
			continue
		}
		var fallback uint32
		if perRecord != nil {
			fallback = perRecord[i]
		}
		if f, ok := d.normalise(fm, src, recv, fallback); ok {
			flows = append(flows, f)
		}
	}
	return flows, nil
}

// normalise converts one goflow2 message into a models.Flow:
//   - timestamp is the flow end time, or the receive time when the exporter
//     sent none or an implausible one (over an hour ahead, or older than
//     retention);
//   - bytes and packets are scaled by the sampling rate;
//   - exporter is the sFlow agent address, else the datagram source.
//
// Records without both addresses (e.g. non-IP sFlow samples) are skipped.
func (d *decoder) normalise(m *protoproducer.ProtoProducerMessage, src netip.Addr, recv time.Time, recordRate uint32) (models.Flow, bool) {
	srcIP, ok1 := addrString(m.SrcAddr)
	dstIP, ok2 := addrString(m.DstAddr)
	if !ok1 || !ok2 {
		return models.Flow{}, false
	}

	start := nsTime(m.TimeFlowStartNs)
	end := nsTime(m.TimeFlowEndNs)
	ts := recv
	if d.plausible(end, recv) {
		ts = end
	}
	var dur float64
	if d.plausible(start, recv) && d.plausible(end, recv) && !end.Before(start) {
		dur = end.Sub(start).Seconds()
	}

	rate := m.SamplingRate
	if rate == 0 {
		rate = uint64(recordRate)
	}
	if rate == 0 {
		rate = 1
	}
	if rate > math.MaxInt32 {
		rate = math.MaxInt32
	}

	exporter := src
	if a, ok := netip.AddrFromSlice(m.SamplerAddress); ok && a.IsValid() && !a.IsUnspecified() {
		exporter = a.Unmap()
	}
	exporterStr := exporter.String()

	f := models.Flow{
		Timestamp:    ts,
		SrcIP:        srcIP,
		DstIP:        dstIP,
		SrcPort:      int(m.SrcPort),
		DstPort:      int(m.DstPort),
		Protocol:     protocolName(m.Proto),
		Bytes:        scale(m.Bytes, rate),
		Packets:      scale(m.Packets, rate),
		Duration:     dur,
		Exporter:     &exporterStr,
		InIf:         ifIndex(m.InIf, m.Type),
		OutIf:        ifIndex(m.OutIf, m.Type),
		ToS:          intPtr(int(m.IpTos & 0xff)),
		SamplingRate: intPtr(int(rate)),
	}
	if m.Proto == 6 {
		f.TCPFlags = intPtr(int(m.TcpFlags & 0xff))
	}
	if id, ok := d.deviceID(exporter); ok {
		f.DeviceID = &id
	}
	return f, true
}

// plausible reports whether an exporter-supplied time can be trusted.
func (d *decoder) plausible(t, recv time.Time) bool {
	if t.IsZero() || t.Unix() <= 0 {
		return false
	}
	return !t.After(recv.Add(maxFutureSkew)) && !t.Before(recv.Add(-d.maxAge))
}

func nsTime(ns uint64) time.Time {
	if ns == 0 || ns > math.MaxInt64 {
		return time.Time{}
	}
	return time.Unix(0, int64(ns)).UTC()
}

func addrString(b []byte) (string, bool) {
	a, ok := netip.AddrFromSlice(b)
	if !ok {
		return "", false
	}
	return a.Unmap().String(), true
}

// scale multiplies a sampled counter, saturating at MaxInt64.
func scale(v, rate uint64) int64 {
	if v == 0 {
		return 0
	}
	if v > math.MaxInt64/rate {
		return math.MaxInt64
	}
	return int64(v * rate) //nolint:gosec // overflow checked above
}

// ifIndex returns an interface index, decoding sFlow's format bits: values
// with a non-zero format (discarded / multiple interfaces) carry no index.
func ifIndex(v uint32, typ flowpb.FlowMessage_FlowType) *int {
	if typ == flowpb.FlowMessage_SFLOW_5 {
		if v>>30 != 0 {
			return nil
		}
		v &= 0x3fffffff
	}
	if v > math.MaxInt32 {
		return nil
	}
	return intPtr(int(v))
}

func intPtr(v int) *int { return &v }

func protocolName(p uint32) string {
	switch p {
	case 1:
		return "ICMP"
	case 2:
		return "IGMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 47:
		return "GRE"
	case 50:
		return "ESP"
	case 51:
		return "AH"
	case 58:
		return "ICMPv6"
	case 89:
		return "OSPF"
	case 132:
		return "SCTP"
	default:
		return strconv.FormatUint(uint64(p), 10)
	}
}

// Sampling-rate information elements that may appear in v9/IPFIX data
// records (rather than in an options record).
var samplingFields = []uint16{
	netflow.NFV9_FIELD_SAMPLING_INTERVAL,            // 34
	netflow.IPFIX_FIELD_samplingPacketInterval,      // 305
	netflow.NFV9_FIELD_FLOW_SAMPLER_RANDOM_INTERVAL, // 50
}

// recordSamplingRates returns, for each data record in order (the order
// goflow2's producer emits messages in), the sampling rate carried in that
// record, or 0.
func recordSamplingRates(sets []any) []uint32 {
	var out []uint32
	for _, s := range sets {
		ds, ok := s.(netflow.DataFlowSet)
		if !ok {
			continue
		}
		for _, rec := range ds.Records {
			var rate uint32
			for _, typ := range samplingFields {
				if found, err := protoproducer.NetFlowPopulate(rec.Values, typ, &rate); found && err == nil && rate > 0 {
					break
				}
				rate = 0
			}
			out = append(out, rate)
		}
	}
	return out
}
