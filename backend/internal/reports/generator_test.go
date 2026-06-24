package reports

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator(nil, "/tmp/reports")
	if g.pool != nil {
		t.Error("expected nil pool")
	}
	if g.outputDir != "/tmp/reports" {
		t.Errorf("expected outputDir /tmp/reports, got %q", g.outputDir)
	}
}

func TestGenerate_UnknownReportType(t *testing.T) {
	g := NewGenerator(nil, t.TempDir())
	_, err := g.Generate(context.TODO(), GenerateRequest{ReportType: "nonexistent"})
	if err == nil {
		t.Error("expected error for unknown report type")
	}
	if !strings.Contains(err.Error(), "unknown report type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerate_SetsDefaultFormat(t *testing.T) {
	g := NewGenerator(nil, t.TempDir())
	_, err := g.Generate(context.TODO(), GenerateRequest{ReportType: "nonexistent"})
	if err == nil {
		t.Error("expected error for unknown report type")
	}
}

func TestAvailabilityHTML_EmptyDevices(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{
		Title:      "Test Report",
		PeriodFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:   time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC),
	}
	html := g.availabilityHTML(req, []deviceAvailability{})
	if !strings.Contains(html, "Test Report") {
		t.Error("HTML should contain title")
	}
	if !strings.Contains(html, "<table>") {
		t.Error("HTML should contain table")
	}
	if strings.Contains(html, "<tr><td>") {
		t.Error("HTML should not contain data rows")
	}
}

func TestAvailabilityHTML_WithDevices(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{
		Title:      "Availability",
		PeriodFrom: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodTo:   time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC),
	}
	devices := []deviceAvailability{
		{ID: 1, Name: "Router-A", IPAddress: "10.0.0.1", TotalChecks: 100, UpChecks: 99, UptimePct: 99.0},
		{ID: 2, Name: "Router-B", IPAddress: "10.0.0.2", TotalChecks: 100, UpChecks: 90, UptimePct: 90.0},
	}
	html := g.availabilityHTML(req, devices)
	if !strings.Contains(html, "Router-A") {
		t.Error("HTML should contain device name")
	}
	if !strings.Contains(html, "10.0.0.1") {
		t.Error("HTML should contain IP address")
	}
	if !strings.Contains(html, "pill ok") {
		t.Error("99% uptime should use 'ok' pill class")
	}
	if !strings.Contains(html, "pill err") {
		t.Error("90% uptime should use 'err' pill class")
	}
}

func TestAvailabilityHTML_WarningThreshold(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "Test"}
	devices := []deviceAvailability{
		{ID: 1, Name: "Dev", IPAddress: "1.2.3.4", TotalChecks: 100, UpChecks: 97, UptimePct: 97.0},
	}
	html := g.availabilityHTML(req, devices)
	if !strings.Contains(html, "pill warn") {
		t.Error("97% uptime should use 'warn' pill class")
	}
}

func TestSLAHTML_Empty(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "SLA Report"}
	html := g.slaHTML(req, []slaRow{})
	if !strings.Contains(html, "SLA Report") {
		t.Error("HTML should contain title")
	}
}

func TestSLAHTML_WithData(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "SLA"}
	slas := []slaRow{
		{Name: "Critical SLA", Severity: "critical", ResponseTimeMin: 15, ResolutionTimeMin: 60, TotalIncidents: 10, BreachedCount: 1, CompliancePct: 90.0},
	}
	html := g.slaHTML(req, slas)
	if !strings.Contains(html, "Critical SLA") {
		t.Error("HTML should contain SLA name")
	}
	if !strings.Contains(html, "pill warn") {
		t.Error("90% compliance should use 'warn' pill class (80-95%)")
	}
}

func TestSLAHTML_LowCompliance(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "SLA"}
	slas := []slaRow{
		{Name: "Bad SLA", Severity: "critical", ResponseTimeMin: 5, ResolutionTimeMin: 30, TotalIncidents: 10, BreachedCount: 8, CompliancePct: 20.0},
	}
	html := g.slaHTML(req, slas)
	if !strings.Contains(html, "pill err") {
		t.Error("20% compliance should use 'err' pill class (< 80%)")
	}
}

func TestSLAHTML_HighCompliance(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "SLA"}
	slas := []slaRow{
		{Name: "Normal SLA", Severity: "warning", ResponseTimeMin: 30, ResolutionTimeMin: 120, TotalIncidents: 20, BreachedCount: 0, CompliancePct: 100.0},
	}
	html := g.slaHTML(req, slas)
	if !strings.Contains(html, "pill ok") {
		t.Error("100% compliance should use 'ok' pill class")
	}
}

func TestMTTRHTML_Empty(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "MTTR Report"}
	html := g.mttrHTML(req, []mttrRow{})
	if !strings.Contains(html, "MTTR Report") {
		t.Error("HTML should contain title")
	}
	if !strings.Contains(html, "Avg Resolve Time") {
		t.Error("HTML should contain header")
	}
}

func TestMTTRHTML_WithData(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "MTTR"}
	rows := []mttrRow{
		{Severity: "critical", IncidentCount: 5, AvgDurationSecs: 3600, MinDurationSecs: 600, MaxDurationSecs: 7200},
		{Severity: "warning", IncidentCount: 10, AvgDurationSecs: 90, MinDurationSecs: 30, MaxDurationSecs: 300},
	}
	html := g.mttrHTML(req, rows)
	if !strings.Contains(html, "critical") {
		t.Error("HTML should contain severity")
	}
	if !strings.Contains(html, "1h 0m") {
		t.Error("HTML should contain formatted duration")
	}
	if !strings.Contains(html, "1m 30s") {
		t.Error("HTML should contain formatted duration for 90s")
	}
}

func TestISPHTML_Empty(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "ISP Report"}
	html := g.ispHTML(req, []ispRow{})
	if !strings.Contains(html, "ISP Report") {
		t.Error("HTML should contain title")
	}
}

func TestISPHTML_WithData(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "ISP"}
	links := []ispRow{
		{Name: "Primary Link", Provider: "ISP-A", AvgLatency: 15.5, AvgJitter: 2.1, AvgPacketLoss: 0.5, AvgDownload: 100.0, AvgUpload: 50.0, TotalProbes: 100, UpProbes: 99, UptimePct: 99.0},
	}
	html := g.ispHTML(req, links)
	if !strings.Contains(html, "Primary Link") {
		t.Error("HTML should contain link name")
	}
	if !strings.Contains(html, "ISP-A") {
		t.Error("HTML should contain provider")
	}
	if !strings.Contains(html, "pill ok") {
		t.Error("99% uptime should use 'ok' pill class")
	}
}

func TestISPHTML_DegradedLink(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "ISP"}
	links := []ispRow{
		{Name: "Bad Link", Provider: "ISP-B", AvgLatency: 100, AvgJitter: 20, AvgPacketLoss: 5, AvgDownload: 10, AvgUpload: 5, TotalProbes: 100, UpProbes: 90, UptimePct: 90.0},
	}
	html := g.ispHTML(req, links)
	if !strings.Contains(html, "pill err") {
		t.Error("90% uptime should use 'err' pill class")
	}
}

func TestTopOffendersHTML_Empty(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "Top Offenders"}
	html := g.topOffendersHTML(req, []offenderRow{})
	if !strings.Contains(html, "Top Offenders") {
		t.Error("HTML should contain title")
	}
}

func TestTopOffendersHTML_WithData(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "Offenders"}
	offenders := []offenderRow{
		{ID: 1, Name: "Bad Device", IPAddress: "10.0.0.99", DownCount: 50, Total: 100, UptimePct: 50.0},
		{ID: 2, Name: "Ok Device", IPAddress: "10.0.0.100", DownCount: 1, Total: 100, UptimePct: 99.0},
	}
	html := g.topOffendersHTML(req, offenders)
	if !strings.Contains(html, "Bad Device") {
		t.Error("HTML should contain device name")
	}
	if !strings.Contains(html, "50.0%") {
		t.Error("HTML should contain uptime percentage")
	}
	if !strings.Contains(html, "<td>1</td>") {
		t.Error("HTML should contain rank number")
	}
}

func TestPerformanceHTML_Empty(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "Performance"}
	html := g.performanceHTML(req, []perfRow{})
	if !strings.Contains(html, "Performance") {
		t.Error("HTML should contain title")
	}
}

func TestPerformanceHTML_WithData(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "Perf"}
	perfs := []perfRow{
		{Interval: "2025-01-01 00:00:00", DeviceCount: 10, AvgResponse: 45.5, AvgPacketLoss: 1.2, AvgCPU: 65.0, AvgMemory: 72.0},
	}
	html := g.performanceHTML(req, perfs)
	if !strings.Contains(html, "10") {
		t.Error("HTML should contain device count")
	}
	if !strings.Contains(html, "45.5") {
		t.Error("HTML should contain avg response time")
	}
}

func TestWriteCSV_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_report.csv")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"col1", "col2"})
	_ = w.Write([]string{"val1", "val2"})
	w.Flush()
	f.Close()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + data), got %d", len(lines))
	}
	if lines[0] != "col1,col2" {
		t.Errorf("header mismatch: %q", lines[0])
	}
	if lines[1] != "val1,val2" {
		t.Errorf("data mismatch: %q", lines[1])
	}
}

func TestWriteCSV_CreatesOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}
	filePath := filepath.Join(dir, "report.csv")

	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"a"})
	_ = w.Write([]string{"b"})
	w.Flush()
	f.Close()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("file should exist after write")
	}
}

func TestWriteHTML_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_report.html")
	htmlContent := "<html><body>test</body></html>"

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(htmlContent), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	if !strings.Contains(string(content), "test") {
		t.Error("file should contain HTML content")
	}
	if !strings.HasSuffix(filePath, ".html") {
		t.Errorf("file should have .html extension, got %q", filePath)
	}
}

func TestGenerateRequest_Defaults(t *testing.T) {
	req := GenerateRequest{}
	if req.Format != "" {
		t.Error("empty format should default to csv in Generate")
	}
	if !req.PeriodFrom.IsZero() {
		t.Error("zero PeriodFrom should be handled in Generate")
	}
	if !req.PeriodTo.IsZero() {
		t.Error("zero PeriodTo should be handled in Generate")
	}
}

func TestAvailabilityHTML_XSSProtection(t *testing.T) {
	g := NewGenerator(nil, "")
	req := GenerateRequest{Title: "<script>alert('xss')</script>"}
	devices := []deviceAvailability{
		{Name: "<img onerror=alert(1)>", IPAddress: "10.0.0.1"},
	}
	html := g.availabilityHTML(req, devices)
	if strings.Contains(html, "<script>") {
		t.Error("title should be HTML-escaped, raw <script> tag found")
	}
	if strings.Contains(html, "<img ") {
		t.Error("device name should be HTML-escaped, raw <img> tag found")
	}
	if !strings.Contains(html, "&lt;") {
		t.Error("HTML entities should be escaped")
	}
}
