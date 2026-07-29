// Package vendorid identifies network-device vendors from locally collected
// discovery evidence. It deliberately makes no external lookup requests so a
// discovery scan remains fast and does not leak inventory data.
package vendorid

import (
	"sort"
	"strings"
)

// Evidence is the non-secret fingerprint material collected during discovery.
// Values may be empty; the identifier combines every available signal.
type Evidence struct {
	MACAddress      string
	SNMPSysObjectID string
	SNMPDescription string
	HTTPTitle       string
	HTTPServer      string
	TLSCommonName   string
	SSHBanner       string
}

type Result struct {
	Vendor     string   `json:"vendor,omitempty"`
	Confidence string   `json:"confidence,omitempty"` // high, medium, low
	Sources    []string `json:"sources,omitempty"`
}

type rule struct {
	vendor  string
	needles []string
}

var oidRules = []rule{
	{"Cisco", []string{"1.3.6.1.4.1.9"}}, {"HPE", []string{"1.3.6.1.4.1.11"}},
	{"Juniper", []string{"1.3.6.1.4.1.2636"}}, {"Aruba", []string{"1.3.6.1.4.1.14823"}},
	{"Fortinet", []string{"1.3.6.1.4.1.12356"}}, {"MikroTik", []string{"1.3.6.1.4.1.14988"}},
	{"Ubiquiti", []string{"1.3.6.1.4.1.41112"}}, {"TP-Link", []string{"1.3.6.1.4.1.11863"}},
	{"Huawei", []string{"1.3.6.1.4.1.2011"}}, {"Dell", []string{"1.3.6.1.4.1.674"}},
	{"Hikvision", []string{"1.3.6.1.4.1.39165"}}, {"Dahua", []string{"1.3.6.1.4.1.1004849"}},
	{"ZKTeco", []string{"1.3.6.1.4.1.34592"}},
}

var textRules = []rule{
	{"Cisco", []string{"cisco", "ios xe", "catalyst"}}, {"HPE", []string{"hewlett packard", "arubaos", "procurve"}},
	{"Juniper", []string{"juniper", "junos"}}, {"Aruba", []string{"aruba networks"}}, {"Fortinet", []string{"fortinet", "fortigate"}},
	//nolint:misspell
	{"MikroTik", []string{"mikrotik", "routeros"}}, {"Ubiquiti", []string{"ubiquiti", "unifi", "edgeos"}},
	{"TP-Link", []string{"tp-link", "tplink", "omada"}}, {"Huawei", []string{"huawei"}}, {"Dell", []string{"dell", "dell emc"}},
	{"Hikvision", []string{"hikvision", "hik-connect"}}, {"Dahua", []string{"dahua"}},
	{"Axis", []string{"axis communications", "axis network camera"}}, {"Hanwha", []string{"hanwha", "wisenet"}},
	{"ZKTeco", []string{"zkteco", "zkteco", "zk software"}}, {"Suprema", []string{"suprema", "biostar"}},
}

// Identify assigns stronger weight to authenticated/device-native signals
// (SNMP OID and system description) than UI branding or a MAC prefix.
func Identify(e Evidence) Result {
	scores := map[string]int{}
	sources := map[string]map[string]bool{}
	add := func(vendor, source string, score int) {
		if vendor == "" {
			return
		}
		scores[vendor] += score
		if sources[vendor] == nil {
			sources[vendor] = map[string]bool{}
		}
		sources[vendor][source] = true
	}
	for _, r := range oidRules {
		if hasAny(e.SNMPSysObjectID, r.needles) {
			add(r.vendor, "snmp_oid", 100)
		}
	}
	matchText := func(value, source string, score int) {
		for _, r := range textRules {
			if hasAny(value, r.needles) {
				add(r.vendor, source, score)
			}
		}
	}
	matchText(e.SNMPDescription, "snmp_description", 80)
	matchText(e.HTTPTitle, "http_title", 65)
	matchText(e.HTTPServer, "http_server", 55)
	matchText(e.TLSCommonName, "tls_certificate", 55)
	matchText(e.SSHBanner, "ssh_banner", 50)
	if vendor := VendorFromMAC(e.MACAddress); vendor != "" {
		add(vendor, "mac_oui", 35)
	}

	best, bestScore := "", 0
	for vendor, score := range scores {
		if score > bestScore || (score == bestScore && vendor < best) {
			best, bestScore = vendor, score
		}
	}
	if best == "" {
		return Result{}
	}
	confidence := "low"
	if bestScore >= 100 {
		confidence = "high"
	} else if bestScore >= 60 {
		confidence = "medium"
	}
	result := Result{Vendor: best, Confidence: confidence}
	for source := range sources[best] {
		result.Sources = append(result.Sources, source)
	}
	sort.Strings(result.Sources)
	return result
}

func hasAny(value string, needles []string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// VendorFromMAC covers the commonly deployed network, CCTV, and access-control
// device prefixes. It is intentionally a fallback: MAC ownership alone is not
// reliable enough to override a device's own SNMP or management fingerprints.
func VendorFromMAC(mac string) string {
	prefix := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(mac), "-", ":"))
	if len(prefix) < 8 {
		return ""
	}
	prefix = prefix[:8]
	for vendor, prefixes := range map[string][]string{
		"Cisco": {"00:01:42", "00:1A:2B", "00:1B:53", "00:25:45"}, "Juniper": {"00:05:85", "00:1B:17", "00:1B:54"},
		"Ubiquiti": {"00:27:22", "24:5A:4C", "68:72:51", "74:83:C2", "78:8A:20"}, "MikroTik": {"4C:5E:0C", "6C:3B:6B", "CC:2D:E0"},
		"TP-Link": {"14:CC:20", "50:C7:BF", "60:32:B1", "B0:4E:26"}, "Hikvision": {"28:57:BE", "44:19:B6", "54:C4:15", "C0:56:E3"},
		"Dahua": {"3C:EF:8C", "A0:BD:1D", "BC:32:5F"}, "Axis": {"00:40:8C"}, "ZKTeco": {"00:17:61"}, "Suprema": {"00:1B:1C"},
	} {
		for _, candidate := range prefixes {
			if prefix == candidate {
				return vendor
			}
		}
	}
	return ""
}
