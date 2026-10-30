package collectors

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// Interface throughput comes from octet-counter deltas between consecutive
// polls of the same device, so the collector keeps the previous counters per
// interface. The first poll of an interface establishes the baseline and has
// no rate yet.

type counterSample struct {
	in, out uint64
	hc      bool // 64-bit ifHC* counters (else 32-bit ifIn/OutOctets)
	at      time.Time
}

type rateTracker struct {
	mu   sync.Mutex
	last map[int64]map[int]counterSample // deviceID → ifIndex → sample
}

var interfaceRates = &rateTracker{last: map[int64]map[int]counterSample{}}

// maxRateGap discards deltas spanning longer than this; a counter seen after
// a long gap may have wrapped more than once.
const maxRateGap = 30 * time.Minute

// counterDelta returns cur-prev, handling a single wrap of a 32-bit counter.
// A 64-bit counter going backwards is a reset (agent reboot), not a wrap.
func counterDelta(prev, cur uint64, hc bool) (uint64, bool) {
	if cur >= prev {
		return cur - prev, true
	}
	if hc {
		return 0, false
	}
	return cur + (math.MaxUint32 + 1 - prev), true
}

// rate updates the stored counters and returns bits/second for in and out.
func (t *rateTracker) rate(deviceID int64, ifIndex int, cur counterSample, speedBps int64) (inBps, outBps float64, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	dev := t.last[deviceID]
	if dev == nil {
		dev = map[int]counterSample{}
		t.last[deviceID] = dev
	}
	prev, seen := dev[ifIndex]
	dev[ifIndex] = cur
	if !seen || prev.hc != cur.hc {
		return 0, 0, false
	}
	dt := cur.at.Sub(prev.at)
	if dt <= 0 || dt > maxRateGap {
		return 0, 0, false
	}
	dIn, okIn := counterDelta(prev.in, cur.in, cur.hc)
	dOut, okOut := counterDelta(prev.out, cur.out, cur.hc)
	if !okIn || !okOut {
		return 0, 0, false
	}
	inBps = float64(dIn) * 8 / dt.Seconds()
	outBps = float64(dOut) * 8 / dt.Seconds()
	// A rate far above line speed means a counter discontinuity, not traffic.
	if speedBps > 0 && (inBps > 1.5*float64(speedBps) || outBps > 1.5*float64(speedBps)) {
		return 0, 0, false
	}
	return inBps, outBps, true
}

// forget drops state for interfaces not seen in the latest poll.
func (t *rateTracker) forget(deviceID int64, seen map[int]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for idx := range t.last[deviceID] {
		if !seen[idx] {
			delete(t.last[deviceID], idx)
		}
	}
}

func pduUint64(pdu gosnmp.SnmpPDU) (uint64, bool) {
	switch v := pdu.Value.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case int:
		if v >= 0 {
			return uint64(v), true
		}
	}
	return 0, false
}

func pduString(pdu gosnmp.SnmpPDU) string {
	if b, ok := pdu.Value.([]byte); ok {
		return string(b)
	}
	return ""
}

// interfaceSamples builds per-interface throughput samples from IF-MIB
// ifTable/ifXTable rows, preferring 64-bit counters and ifName/ifHighSpeed.
// Interfaces that are down and have never carried traffic are skipped.
func interfaceSamples(deviceID int64, now time.Time, baseTable, xTable map[string]map[string]gosnmp.SnmpPDU) []models.InterfaceSample {
	samples := make([]models.InterfaceSample, 0, len(baseTable))
	seen := map[int]bool{}
	for idx, row := range baseTable {
		var ifIndex int
		if _, err := fmt.Sscanf(idx, "%d", &ifIndex); err != nil {
			continue
		}
		name := pduString(row["2"])
		speed := int64(pduToFloat64(row["5"]))
		oper := int(pduToFloat64(row["8"]))
		in, _ := pduUint64(row["10"])
		out, _ := pduUint64(row["16"])
		hc := false
		if x, ok := xTable[idx]; ok {
			if n := pduString(x["1"]); n != "" {
				name = n
			}
			hcIn, okIn := pduUint64(x["6"])
			hcOut, okOut := pduUint64(x["10"])
			if okIn && okOut {
				in, out, hc = hcIn, hcOut, true
			}
			if hs := int64(pduToFloat64(x["15"])); hs > 0 {
				speed = hs * 1_000_000
			}
		}
		if name == "" {
			name = fmt.Sprintf("if%d", ifIndex)
		}
		if oper != 1 && in == 0 && out == 0 {
			continue
		}
		seen[ifIndex] = true
		s := models.InterfaceSample{DeviceID: deviceID, Time: now, IfIndex: ifIndex, Name: name, OperStatus: oper, SpeedBps: speed}
		inBps, outBps, ok := interfaceRates.rate(deviceID, ifIndex, counterSample{in: in, out: out, hc: hc, at: now}, speed)
		if !ok {
			// Baseline poll (or discontinuity): counters recorded, no rate.
			continue
		}
		s.InBps, s.OutBps = inBps, outBps
		if speed > 0 {
			u := math.Round(math.Max(inBps, outBps)/float64(speed)*1000) / 10
			s.Utilization = &u
		}
		samples = append(samples, s)
	}
	interfaceRates.forget(deviceID, seen)
	return samples
}
