package reports

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Generator struct {
	pool      *pgxpool.Pool
	outputDir string
}

type deviceAvailability struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	IPAddress   string  `json:"ipAddress"`
	TotalChecks int     `json:"totalChecks"`
	UpChecks    int     `json:"upChecks"`
	UptimePct   float64 `json:"uptimePercent"`
}

type slaRow struct {
	Name              string  `json:"name"`
	Severity          string  `json:"severity"`
	ResponseTimeMin   int     `json:"responseTimeMinutes"`
	ResolutionTimeMin int     `json:"resolutionTimeMinutes"`
	TotalIncidents    int     `json:"totalIncidents"`
	BreachedCount     int     `json:"breachedCount"`
	CompliancePct     float64 `json:"compliancePercent"`
}

type mttrRow struct {
	Severity        string  `json:"severity"`
	IncidentCount   int     `json:"incidentCount"`
	AvgDurationSecs float64 `json:"avgDurationSeconds"`
	MinDurationSecs float64 `json:"minDurationSeconds"`
	MaxDurationSecs float64 `json:"maxDurationSeconds"`
}

type ispRow struct {
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	AvgLatency    float64 `json:"avgLatency"`
	AvgJitter     float64 `json:"avgJitter"`
	AvgPacketLoss float64 `json:"avgPacketLoss"`
	AvgDownload   float64 `json:"avgDownload"`
	AvgUpload     float64 `json:"avgUpload"`
	TotalProbes   int     `json:"totalProbes"`
	UpProbes      int     `json:"upProbes"`
	UptimePct     float64 `json:"uptimePercent"`
}

type offenderRow struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	IPAddress string  `json:"ipAddress"`
	DownCount int     `json:"downCount"`
	Total     int     `json:"totalChecks"`
	UptimePct float64 `json:"uptimePercent"`
}

type perfRow struct {
	Interval      string  `json:"interval"`
	DeviceCount   int     `json:"deviceCount"`
	AvgResponse   float64 `json:"avgResponseTime"`
	AvgPacketLoss float64 `json:"avgPacketLoss"`
	AvgCPU        float64 `json:"avgCPU"`
	AvgMemory     float64 `json:"avgMemory"`
}

func NewGenerator(pool *pgxpool.Pool, outputDir string) *Generator {
	return &Generator{pool: pool, outputDir: outputDir}
}

type GenerateRequest struct {
	ReportType        string
	Title             string
	Format            string
	PeriodFrom        time.Time
	PeriodTo          time.Time
	ScopeDesc         string
	Recipients        string
	GeneratedBy       string
	ScheduledReportID *int64
}

type GenerateResult struct {
	ID          int64  `json:"id"`
	FilePath    string `json:"filePath"`
	FileSize    int64  `json:"fileSizeBytes"`
	Title       string `json:"title"`
	ReportType  string `json:"reportType"`
	Format      string `json:"format"`
	GeneratedAt string `json:"generatedAt"`
}

func (g *Generator) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	if req.PeriodTo.IsZero() {
		req.PeriodTo = time.Now()
	}
	if req.PeriodFrom.IsZero() {
		req.PeriodTo = time.Now().AddDate(0, 0, -7)
	}
	if req.Format == "" {
		req.Format = "csv"
	}

	switch req.ReportType {
	case "availability":
		return g.generateAvailability(ctx, req)
	case "sla":
		return g.generateSLA(ctx, req)
	case "mttr":
		return g.generateMTTR(ctx, req)
	case "isp":
		return g.generateISP(ctx, req)
	case "top_offenders":
		return g.generateTopOffenders(ctx, req)
	case "performance":
		return g.generatePerformance(ctx, req)
	default:
		return nil, fmt.Errorf("unknown report type: %s", req.ReportType)
	}
}

func (g *Generator) generateAvailability(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT d.id, d.name, d.ip_address,
		 COUNT(m.id) as total,
		 COUNT(m.id) FILTER (WHERE m.status = 'up') as up
		 FROM devices d
		 LEFT JOIN metrics m ON m.device_id = d.id AND m.timestamp BETWEEN $1 AND $2
		 GROUP BY d.id, d.name, d.ip_address
		 ORDER BY d.name`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query availability: %w", err)
	}
	defer rows.Close()

	devices := []deviceAvailability{}
	for rows.Next() {
		var d deviceAvailability
		if err := rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.TotalChecks, &d.UpChecks); err != nil {
			continue
		}
		if d.TotalChecks > 0 {
			d.UptimePct = float64(d.UpChecks) / float64(d.TotalChecks) * 100
		} else {
			d.UptimePct = 0
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "Availability Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"id", "name", "ip_address", "total_checks", "up_checks", "uptime_percent"}, func(w *csv.Writer) error {
			for _, d := range devices {
				_ = w.Write([]string{
					fmt.Sprintf("%d", d.ID), d.Name, d.IPAddress,
					fmt.Sprintf("%d", d.TotalChecks), fmt.Sprintf("%d", d.UpChecks),
					fmt.Sprintf("%.2f", d.UptimePct),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.availabilityHTML(req, devices))
}

func (g *Generator) generateSLA(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT s.name, s.severity, s.response_time_minutes, s.resolution_time_minutes,
		 COUNT(i.id) as total,
		 COUNT(i.id) FILTER (WHERE i.sla_breached) as breached
		 FROM sla_definitions s
		 LEFT JOIN incidents i ON i.severity = s.severity AND i.started_at BETWEEN $1 AND $2
		 WHERE s.enabled = true
		 GROUP BY s.id ORDER BY s.id`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query SLA: %w", err)
	}
	defer rows.Close()

	slas := []slaRow{}
	for rows.Next() {
		var s slaRow
		if err := rows.Scan(&s.Name, &s.Severity, &s.ResponseTimeMin, &s.ResolutionTimeMin, &s.TotalIncidents, &s.BreachedCount); err != nil {
			continue
		}
		if s.TotalIncidents > 0 {
			s.CompliancePct = float64(s.TotalIncidents-s.BreachedCount) / float64(s.TotalIncidents) * 100
		} else {
			s.CompliancePct = 100
		}
		slas = append(slas, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "SLA Compliance Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"name", "severity", "response_time_min", "resolution_time_min", "total_incidents", "breached", "compliance_pct"}, func(w *csv.Writer) error {
			for _, s := range slas {
				_ = w.Write([]string{
					s.Name, s.Severity,
					fmt.Sprintf("%d", s.ResponseTimeMin), fmt.Sprintf("%d", s.ResolutionTimeMin),
					fmt.Sprintf("%d", s.TotalIncidents), fmt.Sprintf("%d", s.BreachedCount),
					fmt.Sprintf("%.1f", s.CompliancePct),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.slaHTML(req, slas))
}

func (g *Generator) generateMTTR(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT severity, COUNT(*) as cnt,
		 AVG(duration_seconds)::float,
		 MIN(duration_seconds)::float,
		 MAX(duration_seconds)::float
		 FROM incidents
		 WHERE duration_seconds IS NOT NULL AND status IN ('resolved','closed')
		 AND started_at BETWEEN $1 AND $2
		 GROUP BY severity ORDER BY severity`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query MTTR: %w", err)
	}
	defer rows.Close()

	rows_data := []mttrRow{}
	for rows.Next() {
		var r mttrRow
		if err := rows.Scan(&r.Severity, &r.IncidentCount, &r.AvgDurationSecs, &r.MinDurationSecs, &r.MaxDurationSecs); err != nil {
			continue
		}
		rows_data = append(rows_data, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "Mean Time To Resolve (MTTR) Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"severity", "incident_count", "avg_duration_sec", "min_duration_sec", "max_duration_sec"}, func(w *csv.Writer) error {
			for _, r := range rows_data {
				_ = w.Write([]string{
					r.Severity, fmt.Sprintf("%d", r.IncidentCount),
					fmt.Sprintf("%.0f", r.AvgDurationSecs), fmt.Sprintf("%.0f", r.MinDurationSecs),
					fmt.Sprintf("%.0f", r.MaxDurationSecs),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.mttrHTML(req, rows_data))
}

func (g *Generator) generateISP(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT l.name, l.provider,
		 COALESCE(AVG(m.latency_ms),0), COALESCE(AVG(m.jitter_ms),0),
		 COALESCE(AVG(m.packet_loss_percent),0),
		 COALESCE(AVG(m.download_speed_mbps),0), COALESCE(AVG(m.upload_speed_mbps),0),
		 COUNT(m.id),
		 COUNT(m.id) FILTER (WHERE m.status = 'up')
		 FROM isp_links l
		 LEFT JOIN isp_metrics m ON m.link_id = l.id AND m.created_at BETWEEN $1 AND $2
		 WHERE l.enabled = true
		 GROUP BY l.id, l.name, l.provider ORDER BY l.name`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query ISP: %w", err)
	}
	defer rows.Close()

	links := []ispRow{}
	for rows.Next() {
		var r ispRow
		if err := rows.Scan(&r.Name, &r.Provider, &r.AvgLatency, &r.AvgJitter, &r.AvgPacketLoss,
			&r.AvgDownload, &r.AvgUpload, &r.TotalProbes, &r.UpProbes); err != nil {
			continue
		}
		if r.TotalProbes > 0 {
			r.UptimePct = float64(r.UpProbes) / float64(r.TotalProbes) * 100
		}
		links = append(links, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "ISP Performance Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"name", "provider", "avg_latency_ms", "avg_jitter_ms", "avg_packet_loss", "avg_download_mbps", "avg_upload_mbps", "total_probes", "uptime_pct"}, func(w *csv.Writer) error {
			for _, r := range links {
				_ = w.Write([]string{
					r.Name, r.Provider,
					fmt.Sprintf("%.2f", r.AvgLatency), fmt.Sprintf("%.2f", r.AvgJitter),
					fmt.Sprintf("%.2f", r.AvgPacketLoss),
					fmt.Sprintf("%.2f", r.AvgDownload), fmt.Sprintf("%.2f", r.AvgUpload),
					fmt.Sprintf("%d", r.TotalProbes), fmt.Sprintf("%.1f", r.UptimePct),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.ispHTML(req, links))
}

func (g *Generator) generateTopOffenders(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT d.id, d.name, d.ip_address,
		 COUNT(m.id) FILTER (WHERE m.status IN ('down','critical')) as down_count,
		 COUNT(m.id) as total
		 FROM devices d
		 LEFT JOIN metrics m ON m.device_id = d.id AND m.timestamp BETWEEN $1 AND $2
		 GROUP BY d.id, d.name, d.ip_address
		 HAVING COUNT(m.id) > 0
		 ORDER BY down_count DESC, d.name
		 LIMIT 20`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query top offenders: %w", err)
	}
	defer rows.Close()

	offenders := []offenderRow{}
	for rows.Next() {
		var r offenderRow
		if err := rows.Scan(&r.ID, &r.Name, &r.IPAddress, &r.DownCount, &r.Total); err != nil {
			continue
		}
		if r.Total > 0 {
			r.UptimePct = float64(r.Total-r.DownCount) / float64(r.Total) * 100
		}
		offenders = append(offenders, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "Top Offenders Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"id", "name", "ip_address", "down_count", "total_checks", "uptime_percent"}, func(w *csv.Writer) error {
			for _, r := range offenders {
				_ = w.Write([]string{
					fmt.Sprintf("%d", r.ID), r.Name, r.IPAddress,
					fmt.Sprintf("%d", r.DownCount), fmt.Sprintf("%d", r.Total),
					fmt.Sprintf("%.1f", r.UptimePct),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.topOffendersHTML(req, offenders))
}

func (g *Generator) generatePerformance(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	rows, err := g.pool.Query(ctx,
		`SELECT date_trunc('hour', timestamp) as interval,
		 COUNT(DISTINCT device_id),
		 AVG(response_time_ms), AVG(packet_loss_pct),
		 AVG(cpu_usage_pct), AVG(memory_usage_pct)
		 FROM metrics
		 WHERE timestamp BETWEEN $1 AND $2
		 GROUP BY interval ORDER BY interval`, req.PeriodFrom, req.PeriodTo)
	if err != nil {
		return nil, fmt.Errorf("query performance: %w", err)
	}
	defer rows.Close()

	perfs := []perfRow{}
	for rows.Next() {
		var r perfRow
		if err := rows.Scan(&r.Interval, &r.DeviceCount, &r.AvgResponse, &r.AvgPacketLoss, &r.AvgCPU, &r.AvgMemory); err != nil {
			continue
		}
		perfs = append(perfs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if req.Title == "" {
		req.Title = "Performance Report"
	}

	if req.Format == "csv" {
		return g.writeCSV(req, []string{"interval", "device_count", "avg_response_ms", "avg_packet_loss", "avg_cpu_pct", "avg_memory_pct"}, func(w *csv.Writer) error {
			for _, r := range perfs {
				_ = w.Write([]string{
					r.Interval, fmt.Sprintf("%d", r.DeviceCount),
					fmt.Sprintf("%.2f", r.AvgResponse), fmt.Sprintf("%.2f", r.AvgPacketLoss),
					fmt.Sprintf("%.2f", r.AvgCPU), fmt.Sprintf("%.2f", r.AvgMemory),
				})
			}
			return nil
		})
	}
	return g.writeHTML(req, g.performanceHTML(req, perfs))
}

func (g *Generator) writeCSV(req GenerateRequest, headers []string, writeRows func(*csv.Writer) error) (*GenerateResult, error) {
	if err := os.MkdirAll(g.outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.csv", req.ReportType, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(g.outputDir, filename)

	f, err := os.Create(filePath) //nolint:gosec // filePath constructed from safe components
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	_ = w.Write(headers)
	if err := writeRows(w); err != nil {
		return nil, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	info, _ := f.Stat()
	fileSize := int64(0)
	if info != nil {
		fileSize = info.Size()
	}

	return g.recordReport(req, filePath, fileSize)
}

func (g *Generator) writeHTML(req GenerateRequest, htmlContent string) (*GenerateResult, error) {
	if err := os.MkdirAll(g.outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.html", req.ReportType, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(g.outputDir, filename)

	if err := os.WriteFile(filePath, []byte(htmlContent), 0o600); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	info, _ := os.Stat(filePath)
	fileSize := int64(0)
	if info != nil {
		fileSize = info.Size()
	}

	return g.recordReport(req, filePath, fileSize)
}

func (g *Generator) recordReport(req GenerateRequest, filePath string, fileSize int64) (*GenerateResult, error) {
	var id int64
	err := g.pool.QueryRow(context.Background(),
		`INSERT INTO generated_reports(scheduled_report_id, report_type, title, format, file_path, file_size_bytes, scope_description, period_from, period_to, recipients, generated_by)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		req.ScheduledReportID, req.ReportType, req.Title, req.Format,
		filePath, fileSize, req.ScopeDesc, req.PeriodFrom, req.PeriodTo,
		req.Recipients, req.GeneratedBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("record report: %w", err)
	}

	slog.Info("Report generated", "id", id, "type", req.ReportType, "format", req.Format, "file", filePath, "size", fileSize)

	return &GenerateResult{
		ID:          id,
		FilePath:    filePath,
		FileSize:    fileSize,
		Title:       req.Title,
		ReportType:  req.ReportType,
		Format:      req.Format,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
