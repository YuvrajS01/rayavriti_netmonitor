package collectors

import (
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

func octets(s string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte(s)}
}
func integer(v int) gosnmp.SnmpPDU { return gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: v} }

func TestLLDPNeighbors(t *testing.T) {
	now := time.Now()
	rem := map[string]map[string]gosnmp.SnmpPDU{
		// timeMark.localPortNum.remIndex
		"0.5.1": {
			"4": integer(lldpChassisMAC), "5": {Type: gosnmp.OctetString, Value: []byte{0x00, 0x1b, 0x54, 0xaa, 0xbb, 0xcc}},
			"6": integer(5), "7": octets("Gi1/0/48"), "8": octets("Uplink to core"), "9": octets("access-1.example.net"),
		},
		// Same remote listed again under a newer timeMark: deduped.
		"300.5.2": {
			"4": integer(lldpChassisMAC), "5": {Type: gosnmp.OctetString, Value: []byte{0x00, 0x1b, 0x54, 0xaa, 0xbb, 0xcc}},
			"6": integer(5), "7": octets("Gi1/0/48"),
		},
		"0.7.3": {
			"4": integer(7), "5": octets("ap-lobby"), "6": integer(lldpPortMAC),
			"7": {Type: gosnmp.OctetString, Value: []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}},
		},
		"0.9.4": {"4": integer(lldpChassisMAC)}, // no chassis ID: skipped
	}
	loc := map[string]map[string]gosnmp.SnmpPDU{
		"5": {"3": octets("Gi0/5"), "4": octets("GigabitEthernet0/5")},
		"7": {"3": {Type: gosnmp.OctetString, Value: []byte{0x01, 0x02}}, "4": octets("Port 7")},
	}
	man := map[string]map[string]gosnmp.SnmpPDU{
		// timeMark.localPortNum.remIndex.subtype.len.a.b.c.d
		"0.5.1.1.4.10.0.0.21":                        {"3": integer(2)},
		"0.7.3.2.16.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.1": {"3": integer(2)}, // IPv6: ignored
	}

	got := lldpNeighbors(42, now, rem, loc, man)
	if len(got) != 2 {
		t.Fatalf("expected 2 neighbours, got %d: %+v", len(got), got)
	}
	byPort := map[int]int{}
	for i, n := range got {
		byPort[n.LocalPortNum] = i
	}
	a := got[byPort[5]]
	if a.DeviceID != 42 || a.LocalPort != "Gi0/5" || a.RemoteChassisID != "00:1b:54:aa:bb:cc" ||
		a.RemotePortID != "Gi1/0/48" || a.RemoteSysName != "access-1.example.net" || a.RemoteMgmtIP != "10.0.0.21" {
		t.Fatalf("port 5 neighbour: %+v", a)
	}
	b := got[byPort[7]]
	if b.LocalPort != "Port 7" || b.RemoteChassisID != "ap-lobby" || b.RemotePortID != "de:ad:be:ef:00:01" || b.RemoteMgmtIP != "" {
		t.Fatalf("port 7 neighbour: %+v", b)
	}
}

func TestLLDPID(t *testing.T) {
	cases := []struct {
		val     []byte
		subtype int
		want    string
	}{
		{[]byte{1, 10, 0, 0, 1}, lldpChassisNetAddr, "10.0.0.1"},
		{[]byte("switch-1\x00"), 7, "switch-1"},
		{[]byte{0x01, 0xff}, 7, "01:ff"},
		{[]byte{0, 1, 2, 3, 4, 5}, lldpChassisMAC, "00:01:02:03:04:05"},
	}
	for _, c := range cases {
		if got := lldpID(gosnmp.SnmpPDU{Value: c.val}, c.subtype, lldpChassisMAC, lldpChassisNetAddr); got != c.want {
			t.Errorf("lldpID(%v, %d) = %q, want %q", c.val, c.subtype, got, c.want)
		}
	}
}

func TestLLDPSchedule(t *testing.T) {
	s := &lldpSchedule{last: map[int64]time.Time{}}
	now := time.Now()
	if !s.due(1, now) {
		t.Fatal("first poll must walk LLDP")
	}
	if s.due(1, now.Add(time.Minute)) {
		t.Fatal("walked again inside the interval")
	}
	if !s.due(2, now.Add(time.Minute)) {
		t.Fatal("schedule must be per device")
	}
	if !s.due(1, now.Add(lldpInterval)) {
		t.Fatal("not due after the interval")
	}
}
