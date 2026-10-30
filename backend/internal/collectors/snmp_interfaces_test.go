package collectors

import (
	"math"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

func TestRateTracker(t *testing.T) {
	tr := &rateTracker{last: map[int64]map[int]counterSample{}}
	t0 := time.Unix(1_700_000_000, 0)
	const gbit = 1_000_000_000

	if _, _, ok := tr.rate(1, 1, counterSample{in: 1000, out: 0, hc: true, at: t0}, gbit); ok {
		t.Fatal("first poll must only record a baseline")
	}
	in, out, ok := tr.rate(1, 1, counterSample{in: 1000 + 125_000_000, out: 12_500_000, hc: true, at: t0.Add(10 * time.Second)}, gbit)
	if !ok || in != 100_000_000 || out != 10_000_000 {
		t.Fatalf("rate = %v/%v/%v, want 100 Mbit/s in, 10 Mbit/s out", in, out, ok)
	}

	// 32-bit counter wraps once.
	tr.rate(2, 1, counterSample{in: math.MaxUint32 - 999, hc: false, at: t0}, gbit)
	in, _, ok = tr.rate(2, 1, counterSample{in: 1000, hc: false, at: t0.Add(time.Second)}, gbit)
	if !ok || in != 2000*8 {
		t.Fatalf("32-bit wrap: rate = %v (%v), want %v", in, ok, 2000*8)
	}

	// 64-bit counter going backwards is a reset, not a wrap.
	tr.rate(3, 1, counterSample{in: 5_000_000, hc: true, at: t0}, gbit)
	if _, _, ok := tr.rate(3, 1, counterSample{in: 10, hc: true, at: t0.Add(time.Second)}, gbit); ok {
		t.Fatal("64-bit counter reset must not produce a rate")
	}

	// Too long between polls.
	tr.rate(4, 1, counterSample{in: 0, hc: true, at: t0}, gbit)
	if _, _, ok := tr.rate(4, 1, counterSample{in: 1000, hc: true, at: t0.Add(time.Hour)}, gbit); ok {
		t.Fatal("a delta across a long gap must be discarded")
	}

	// Impossible rate for the line speed (counter discontinuity).
	tr.rate(5, 1, counterSample{in: 0, hc: true, at: t0}, 10_000_000)
	if _, _, ok := tr.rate(5, 1, counterSample{in: 1_000_000_000, hc: true, at: t0.Add(time.Second)}, 10_000_000); ok {
		t.Fatal("a rate far above line speed must be discarded")
	}
}

func TestInterfaceSamples_UsesHCCountersAndHighSpeed(t *testing.T) {
	interfaceRates = &rateTracker{last: map[int64]map[int]counterSample{}}
	pdu := func(v any) gosnmp.SnmpPDU { return gosnmp.SnmpPDU{Value: v} }
	row := func(in, out uint64) (map[string]map[string]gosnmp.SnmpPDU, map[string]map[string]gosnmp.SnmpPDU) {
		base := map[string]map[string]gosnmp.SnmpPDU{
			"7": {"2": pdu([]byte("eth0")), "5": pdu(uint32(math.MaxUint32)), "8": pdu(1), "10": pdu(uint32(1)), "16": pdu(uint32(1))},
			"9": {"2": pdu([]byte("unused")), "8": pdu(2), "10": pdu(uint32(0)), "16": pdu(uint32(0))},
		}
		x := map[string]map[string]gosnmp.SnmpPDU{
			"7": {"1": pdu([]byte("Gi1/0/7")), "6": pdu(in), "10": pdu(out), "15": pdu(uint32(10_000))},
		}
		return base, x
	}
	t0 := time.Unix(1_700_000_000, 0)
	b, x := row(0, 0)
	if got := interfaceSamples(42, t0, b, x); len(got) != 0 {
		t.Fatalf("baseline poll produced %d samples", len(got))
	}
	b, x = row(1_250_000_000, 625_000_000)
	got := interfaceSamples(42, t0.Add(10*time.Second), b, x)
	if len(got) != 1 {
		t.Fatalf("got %d samples, want 1 (down, idle interface skipped)", len(got))
	}
	s := got[0]
	if s.Name != "Gi1/0/7" || s.SpeedBps != 10_000_000_000 || s.InBps != 1e9 || s.OutBps != 5e8 {
		t.Fatalf("unexpected sample: %+v", s)
	}
	if s.Utilization == nil || *s.Utilization != 10 {
		t.Fatalf("utilization = %v, want 10%%", s.Utilization)
	}
}
