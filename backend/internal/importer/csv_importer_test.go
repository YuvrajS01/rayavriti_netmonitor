package importer

import (
	"strings"
	"testing"
)

func TestTemplateCSV_NonEmpty(t *testing.T) {
	csv := TemplateCSV()
	if csv == "" {
		t.Fatal("expected non-empty template CSV")
	}
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) < 4 { // header + 3 example rows
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}
}

func TestTemplateCSV_HeaderMatchesExpected(t *testing.T) {
	csv := TemplateCSV()
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	header := lines[0]
	for _, h := range csvHeaders {
		if !strings.Contains(header, h) {
			t.Errorf("expected header to contain %q", h)
		}
	}
}

func TestTemplateHeaders(t *testing.T) {
	headers := TemplateHeaders()
	if len(headers) != len(csvHeaders) {
		t.Fatalf("expected %d headers, got %d", len(csvHeaders), len(headers))
	}
	for i, h := range headers {
		if h != csvHeaders[i] {
			t.Errorf("header %d: expected %q, got %q", i, csvHeaders[i], h)
		}
	}
}

// ---------- ParseCSV tests ----------

func TestParseCSV_Empty(t *testing.T) {
	svc := &ImportService{}
	_, err := svc.ParseCSV(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty CSV")
	}
}

func TestParseCSV_HeaderOnly(t *testing.T) {
	svc := &ImportService{}
	csv := strings.Join(csvHeaders, ",") + "\n"
	rows, err := svc.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 data rows, got %d", len(rows))
	}
}

func TestParseCSV_InvalidHeader(t *testing.T) {
	svc := &ImportService{}
	csv := "wrong,headers,here,too,few,cols,too,few,cols,too,few,cols\n"
	_, err := svc.ParseCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected error for invalid header")
	}
}

func TestParseCSV_MinimalValid(t *testing.T) {
	svc := &ImportService{}
	csv := strings.Join(csvHeaders, ",") + "\n" +
		`"TestDevice",10.0.0.1,ping,,workstation,,,AA:BB:CC:DD:EE:FF,,admin@test.com,"test notes"`
	rows, err := svc.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Name != "TestDevice" {
		t.Errorf("expected name 'TestDevice', got %q", r.Name)
	}
	if r.Host != "10.0.0.1" {
		t.Errorf("expected host '10.0.0.1', got %q", r.Host)
	}
	if r.Protocol != "ping" {
		t.Errorf("expected protocol 'ping', got %q", r.Protocol)
	}
	if r.DeviceCategory != "workstation" {
		t.Errorf("expected category 'workstation', got %q", r.DeviceCategory)
	}
	if r.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC 'AA:BB:CC:DD:EE:FF', got %q", r.MACAddress)
	}
	if r.ContactEmail != "admin@test.com" {
		t.Errorf("expected email 'admin@test.com', got %q", r.ContactEmail)
	}
	if r.Notes != "test notes" {
		t.Errorf("expected notes 'test notes', got %q", r.Notes)
	}
	if r.LineNumber != 2 {
		t.Errorf("expected line number 2, got %d", r.LineNumber)
	}
}

func TestParseCSV_MultipleRows(t *testing.T) {
	svc := &ImportService{}
	input := strings.Join(csvHeaders, ",") + "\n" +
		"Dev1,10.0.0.1,ping,,workstation,,10.0.0.100,AA:BB:CC:DD:EE:01,ASSET-001,admin@test.com,note1\n" +
		"Dev2,10.0.0.2,snmp,161,switch,,10.0.0.1,AA:BB:CC:DD:EE:02,ASSET-002,admin@test.com,note2\n"
	rows, err := svc.ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Name != "Dev1" {
		t.Errorf("expected name 'Dev1', got %q", rows[0].Name)
	}
	if rows[1].Name != "Dev2" {
		t.Errorf("expected name 'Dev2', got %q", rows[1].Name)
	}
	if rows[1].Port != 161 {
		t.Errorf("expected port 161, got %d", rows[1].Port)
	}
	if rows[0].LocationCode != "" {
		t.Errorf("expected empty location code, got %q", rows[0].LocationCode)
	}
	if rows[0].ParentDeviceHost != "10.0.0.100" {
		t.Errorf("expected parent '10.0.0.100', got %q", rows[0].ParentDeviceHost)
	}
}

func TestParseCSV_MostlyEmptyRow(t *testing.T) {
	svc := &ImportService{}
	input := strings.Join(csvHeaders, ",") + "\n" +
		"Short,10.0.0.1,,,,,,,,,\n"
	rows, err := svc.ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Name != "Short" {
		t.Errorf("expected name 'Short', got %q", rows[0].Name)
	}
	if rows[0].Host != "10.0.0.1" {
		t.Errorf("expected host '10.0.0.1', got %q", rows[0].Host)
	}
	if rows[0].Port != 0 {
		t.Errorf("expected port 0 (empty field), got %d", rows[0].Port)
	}
	if rows[0].Protocol != "" {
		t.Errorf("expected empty protocol, got %q", rows[0].Protocol)
	}
}

func TestParseCSV_PortParsing(t *testing.T) {
	svc := &ImportService{}
	csv := strings.Join(csvHeaders, ",") + "\n" +
		`"Dev",10.0.0.1,snmp,161,,,,,,,`
	rows, err := svc.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Port != 161 {
		t.Errorf("expected port 161, got %d", rows[0].Port)
	}
}

func TestParseCSV_InvalidPortIgnored(t *testing.T) {
	svc := &ImportService{}
	csv := strings.Join(csvHeaders, ",") + "\n" +
		`"Dev",10.0.0.1,snmp,abc,,,,,,,`
	rows, err := svc.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Port != 0 {
		t.Errorf("expected port 0 for invalid port, got %d", rows[0].Port)
	}
}

// ---------- isValidHost tests ----------

func TestIsValidHost_IPv4(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"", false},
		{"hostname.local", true},
		{"switch-01.college.edu", true},
		// These are accepted as valid hostnames (isHost is permissive —
		// validation warnings are issued at the row level, not rejected)
		{"999.999.999.999", true},
		{"not-an-ip", true},
	}
	for _, tt := range tests {
		got := isValidHost(tt.input)
		if got != tt.valid {
			t.Errorf("isValidHost(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

func TestIsValidHost_IPv6(t *testing.T) {
	if !isValidHost("::1") {
		t.Error("expected ::1 to be valid")
	}
	if !isValidHost("2001:db8::1") {
		t.Error("expected 2001:db8::1 to be valid")
	}
}

func TestIsValidHost_URL(t *testing.T) {
	if !isValidHost("https://example.com") {
		t.Error("expected https URL to be valid")
	}
}

// ---------- macRegex tests ----------

func TestMacRegex(t *testing.T) {
	valid := []string{
		"AA:BB:CC:DD:EE:FF",
		"aa:bb:cc:dd:ee:ff",
		"00:11:22:33:44:55",
		"AA:BB:CC:DD:EE:ff",
	}
	for _, mac := range valid {
		if !macRegex.MatchString(mac) {
			t.Errorf("expected %q to be valid MAC", mac)
		}
	}
	invalid := []string{
		"",
		"AA:BB:CC:DD:EE",
		"AA:BB:CC:DD:EE:FF:00",
		"GG:HH:II:JJ:KK:LL",
		"AA-BB-CC-DD-EE-FF",
	}
	for _, mac := range invalid {
		if macRegex.MatchString(mac) {
			t.Errorf("expected %q to be invalid MAC", mac)
		}
	}
}

// ---------- csvHeaders consistency ----------

func TestCSVHeadersLength(t *testing.T) {
	if len(csvHeaders) != 11 {
		t.Fatalf("expected 11 CSV headers, got %d", len(csvHeaders))
	}
}
