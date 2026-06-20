package reports

import (
	"fmt"
	"html"
	"strings"
	"time"
)

func (g *Generator) availabilityHTML(req GenerateRequest, devices []deviceAvailability) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		".pill{display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:700}"+
		".ok{background:#dff1e4;color:#245a3a}.warn{background:#fff3cd;color:#856404}.err{background:#f8d7da;color:#721c24}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s &middot; Period: %s to %s</p>"+
		"<table><tr><th>Device</th><th>IP Address</th><th>Total Checks</th><th>Up</th><th>Uptime</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		req.PeriodFrom.Format("2006-01-02"), req.PeriodTo.Format("2006-01-02")))

	for _, d := range devices {
		pillClass := "ok"
		if d.UptimePct < 99 {
			pillClass = "warn"
		}
		if d.UptimePct < 95 {
			pillClass = "err"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td><span class=\"pill %s\">%.1f%%</span></td></tr>",
			html.EscapeString(d.Name), html.EscapeString(d.IPAddress),
			d.TotalChecks, d.UpChecks, pillClass, d.UptimePct))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func (g *Generator) slaHTML(req GenerateRequest, slas []slaRow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		".pill{display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:700}"+
		".ok{background:#dff1e4;color:#245a3a}.warn{background:#fff3cd;color:#856404}.err{background:#f8d7da;color:#721c24}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s</p>"+
		"<table><tr><th>SLA Name</th><th>Severity</th><th>Response (min)</th><th>Resolution (min)</th><th>Total</th><th>Breached</th><th>Compliance</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	for _, s := range slas {
		pillClass := "ok"
		if s.CompliancePct < 95 {
			pillClass = "warn"
		}
		if s.CompliancePct < 80 {
			pillClass = "err"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td><span class=\"pill %s\">%.1f%%</span></td></tr>",
			html.EscapeString(s.Name), html.EscapeString(s.Severity),
			s.ResponseTimeMin, s.ResolutionTimeMin, s.TotalIncidents, s.BreachedCount,
			pillClass, s.CompliancePct))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func (g *Generator) mttrHTML(req GenerateRequest, rows []mttrRow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s</p>"+
		"<table><tr><th>Severity</th><th>Incidents</th><th>Avg Resolve Time</th><th>Min</th><th>Max</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	for _, r := range rows {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(r.Severity), r.IncidentCount,
			formatDuration(r.AvgDurationSecs), formatDuration(r.MinDurationSecs), formatDuration(r.MaxDurationSecs)))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func (g *Generator) ispHTML(req GenerateRequest, links []ispRow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		".pill{display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:700}"+
		".ok{background:#dff1e4;color:#245a3a}.warn{background:#fff3cd;color:#856404}.err{background:#f8d7da;color:#721c24}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s</p>"+
		"<table><tr><th>Link</th><th>Provider</th><th>Avg Latency</th><th>Avg Jitter</th><th>Packet Loss</th><th>Down</th><th>Up</th><th>Uptime</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	for _, l := range links {
		pillClass := "ok"
		if l.UptimePct < 99 {
			pillClass = "warn"
		}
		if l.UptimePct < 95 {
			pillClass = "err"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%.1fms</td><td>%.1fms</td><td>%.2f%%</td><td>%.1f Mbps</td><td>%.1f Mbps</td><td><span class=\"pill %s\">%.1f%%</span></td></tr>",
			html.EscapeString(l.Name), html.EscapeString(l.Provider),
			l.AvgLatency, l.AvgJitter, l.AvgPacketLoss, l.AvgDownload, l.AvgUpload,
			pillClass, l.UptimePct))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func (g *Generator) topOffendersHTML(req GenerateRequest, offenders []offenderRow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		".pill{display:inline-block;padding:2px 8px;border-radius:999px;font-size:12px;font-weight:700}"+
		".ok{background:#dff1e4;color:#245a3a}.warn{background:#fff3cd;color:#856404}.err{background:#f8d7da;color:#721c24}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s</p>"+
		"<table><tr><th>#</th><th>Device</th><th>IP</th><th>Down Count</th><th>Total Checks</th><th>Uptime</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	for i, d := range offenders {
		pillClass := "ok"
		if d.UptimePct < 99 {
			pillClass = "warn"
		}
		if d.UptimePct < 95 {
			pillClass = "err"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td><span class=\"pill %s\">%.1f%%</span></td></tr>",
			i+1, html.EscapeString(d.Name), html.EscapeString(d.IPAddress),
			d.DownCount, d.Total, pillClass, d.UptimePct))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func (g *Generator) performanceHTML(req GenerateRequest, perfs []perfRow) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title>"+
		"<style>body{margin:0;font-family:system-ui,sans-serif;background:#faf8f4;color:#1f2421;padding:40px}"+
		"h1{font-size:24px;border-bottom:3px solid #1f2421;padding-bottom:12px}"+
		".meta{color:#6f6a5f;font-size:13px;margin-bottom:24px}"+
		"table{width:100%%;border-collapse:collapse;margin-top:16px}"+
		"th,td{padding:10px 12px;text-align:left;border-bottom:1px solid #d8d0bd;font-size:14px}"+
		"th{background:#f0ece4;font-weight:700;text-transform:uppercase;font-size:12px}"+
		"</style></head><body><h1>%s</h1>"+
		"<p class=\"meta\">Generated: %s</p>"+
		"<table><tr><th>Interval</th><th>Devices</th><th>Avg Response (ms)</th><th>Avg Packet Loss</th><th>Avg CPU</th><th>Avg Memory</th></tr>",
		html.EscapeString(req.Title), html.EscapeString(req.Title),
		time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	for _, p := range perfs {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%.1f</td><td>%.2f%%</td><td>%.1f%%</td><td>%.1f%%</td></tr>",
			html.EscapeString(p.Interval), p.DeviceCount,
			p.AvgResponse, p.AvgPacketLoss, p.AvgCPU, p.AvgMemory))
	}
	b.WriteString("</table></body></html>")
	return b.String()
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
