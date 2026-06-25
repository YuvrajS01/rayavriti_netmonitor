package importer

import "strings"

// templateRows contains the CSV header followed by realistic example data
// rows for a college campus network.
var templateRows = [][]string{
	// header
	{"name", "host", "protocol", "port", "device_category", "location_code",
		"parent_device_host", "mac_address", "asset_tag", "contact_email", "notes"},
	// example 1 – workstation
	{`"Lab1-PC01"`, "10.2.1.1", "ping", "", "workstation", "CS-L1",
		"", "AA:BB:CC:DD:EE:01", "ASSET-001", "admin@college.edu", `"Row 1 Window"`},
	// example 2 – managed switch
	{`"Lab1-Switch"`, "10.2.1.254", "snmp", "161", "switch", "CS-L1",
		"10.2.0.1", "AA:BB:CC:DD:EE:FE", "ASSET-SW-01", "netadmin@college.edu", `"48-port managed"`},
	// example 3 – gateway router
	{`"Gateway Router"`, "10.2.0.1", "ping", "", "router", "MC",
		"", "", "ASSET-GW-01", "netadmin@college.edu", `"Main campus gateway"`},
}

// TemplateCSV returns a complete CSV string (header + 3 example data rows)
// ready to be served as a downloadable template file.
func TemplateCSV() string {
	var b strings.Builder
	for _, row := range templateRows {
		b.WriteString(strings.Join(row, ","))
		b.WriteByte('\n')
	}
	return b.String()
}

// TemplateHeaders returns the expected CSV column headers as a string slice.
func TemplateHeaders() []string {
	out := make([]string, len(templateRows[0]))
	copy(out, templateRows[0])
	return out
}
