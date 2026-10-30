package collectors

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gosnmp/gosnmp"
	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// LLDP-MIB (IEEE 802.1AB) entry OIDs.
const (
	oidLLDPLocPortEntry    = ".1.0.8802.1.1.2.1.3.7.1"
	oidLLDPRemEntry        = ".1.0.8802.1.1.2.1.4.1.1"
	oidLLDPRemManAddrEntry = ".1.0.8802.1.1.2.1.4.2.1"
)

// LLDP chassis/port ID subtypes that carry binary values.
const (
	lldpChassisMAC     = 4
	lldpChassisNetAddr = 5
	lldpPortMAC        = 3
	lldpPortNetAddr    = 4
)

// lldpInterval is how often a device's neighbour tables are walked.
// Neighbours change rarely, so walking them on every poll is wasted load.
const lldpInterval = 10 * time.Minute

type lldpSchedule struct {
	mu   sync.Mutex
	last map[int64]time.Time
}

var lldpDue = &lldpSchedule{last: map[int64]time.Time{}}

// due reports whether deviceID's neighbours should be walked now, and if
// so records the attempt.
func (s *lldpSchedule) due(deviceID int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.last[deviceID]; ok && now.Sub(t) < lldpInterval {
		return false
	}
	s.last[deviceID] = now
	return true
}

// collectLLDP walks the LLDP local port, remote and management address
// tables. ok is false when the walk failed; an agent without LLDP-MIB
// returns ok with no neighbours.
func collectLLDP(g *gosnmp.GoSNMP, deviceID int64, now time.Time) (neighbors []models.LLDPNeighbor, ok bool) {
	rem, err := collectTable(g, oidLLDPRemEntry, []int{4, 5, 6, 7, 8, 9}, 20)
	if err != nil {
		return nil, false
	}
	if len(rem) == 0 {
		return []models.LLDPNeighbor{}, true
	}
	loc, err := collectTable(g, oidLLDPLocPortEntry, []int{3, 4}, 40)
	if err != nil {
		loc = nil
	}
	man, err := collectTable(g, oidLLDPRemManAddrEntry, []int{3}, 20)
	if err != nil {
		man = nil
	}
	return lldpNeighbors(deviceID, now, rem, loc, man), true
}

// lldpNeighbors builds neighbours from walked tables. lldpRemTable rows are
// indexed timeMark.localPortNum.remIndex; lldpRemManAddrTable rows append
// addrSubtype.addrLen.addr... to that index.
func lldpNeighbors(deviceID int64, now time.Time, rem, loc, man map[string]map[string]gosnmp.SnmpPDU) []models.LLDPNeighbor {
	mgmt := map[string]string{}
	for idx := range man {
		parts := strings.Split(idx, ".")
		// IPv4 only: subtype 1, length 4.
		if len(parts) == 9 && parts[3] == "1" && parts[4] == "4" {
			key := strings.Join(parts[1:3], ".")
			if _, seen := mgmt[key]; !seen {
				mgmt[key] = strings.Join(parts[5:], ".")
			}
		}
	}

	out := make([]models.LLDPNeighbor, 0, len(rem))
	seen := map[string]bool{}
	for idx, row := range rem {
		parts := strings.Split(idx, ".")
		if len(parts) != 3 {
			continue
		}
		localPort, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		chassis := lldpID(row["5"], pduInt(row["4"]), lldpChassisMAC, lldpChassisNetAddr)
		portID := lldpID(row["7"], pduInt(row["6"]), lldpPortMAC, lldpPortNetAddr)
		if chassis == "" {
			continue
		}
		// A timeMark change can list the same remote twice; keep one.
		dedupe := parts[1] + "|" + chassis + "|" + portID
		if seen[dedupe] {
			continue
		}
		seen[dedupe] = true

		n := models.LLDPNeighbor{
			DeviceID:        deviceID,
			LocalPortNum:    localPort,
			LocalPort:       lldpLocalPort(loc, parts[1]),
			RemoteChassisID: chassis,
			RemoteSysName:   printable(pduString(row["9"])),
			RemotePortID:    portID,
			RemotePortDesc:  printable(pduString(row["8"])),
			RemoteMgmtIP:    mgmt[parts[1]+"."+parts[2]],
			FirstSeen:       now,
			LastSeen:        now,
		}
		out = append(out, n)
	}
	return out
}

func lldpLocalPort(loc map[string]map[string]gosnmp.SnmpPDU, portNum string) string {
	row := loc[portNum]
	if row == nil {
		return portNum
	}
	if id := printable(pduString(row["3"])); id != "" {
		return id
	}
	if desc := printable(pduString(row["4"])); desc != "" {
		return desc
	}
	return portNum
}

// lldpID renders a chassis or port ID: MAC subtypes as aa:bb:.., network
// address subtypes (IANA family byte + address) as an IP, and anything
// else as text when printable or hex otherwise.
func lldpID(pdu gosnmp.SnmpPDU, subtype, macSubtype, netSubtype int) string {
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) == 0 {
		return ""
	}
	switch {
	case subtype == macSubtype && len(b) == 6:
		return net.HardwareAddr(b).String()
	case subtype == netSubtype && len(b) == 5 && b[0] == 1:
		return net.IP(b[1:]).String()
	case subtype == netSubtype && len(b) == 17 && b[0] == 2:
		return net.IP(b[1:]).String()
	}
	if s := printable(string(b)); s != "" {
		return s
	}
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(parts, ":")
}

// printable returns s trimmed of NULs and spaces, or "" when it contains
// non-printable characters.
func printable(s string) string {
	s = strings.Trim(s, "\x00 ")
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return ""
		}
	}
	return s
}

func pduInt(pdu gosnmp.SnmpPDU) int {
	if v, ok := pduUint64(pdu); ok && v < 1<<31 {
		return int(v)
	}
	return 0
}
