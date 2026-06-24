package reports

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ISPCollector struct {
	pool     *pgxpool.Pool
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewISPCollector(pool *pgxpool.Pool, intervalSec int) *ISPCollector {
	if intervalSec <= 0 {
		intervalSec = 10
	}
	return &ISPCollector{
		pool:     pool,
		interval: time.Duration(intervalSec) * time.Second,
	}
}

func (c *ISPCollector) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run(ctx)
	}()

	slog.Info("ISP collector started", "interval", c.interval)
}

func (c *ISPCollector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	slog.Info("ISP collector stopped")
}

func (c *ISPCollector) run(ctx context.Context) {
	c.collectOnce(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectOnce(ctx)
		}
	}
}

func (c *ISPCollector) collectOnce(ctx context.Context) {
	rows, err := c.pool.Query(ctx,
		`SELECT id, name, gateway_ip, monitoring_interval_seconds, enabled
		 FROM isp_links WHERE enabled=true`)
	if err != nil {
		slog.Error("Failed to query ISP links", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name, gatewayIP string
		var intervalSec int
		var enabled bool
		if err := rows.Scan(&id, &name, &gatewayIP, &intervalSec, &enabled); err != nil {
			continue
		}

		metrics := c.probeLink(ctx, gatewayIP)

		status := "up"
		if metrics.packetLoss >= 100 {
			status = "down"
		} else if metrics.packetLoss > 10 {
			status = "degraded"
		}

		_, err := c.pool.Exec(ctx,
			`INSERT INTO isp_metrics(link_id, latency_ms, jitter_ms, packet_loss_percent, download_speed_mbps, upload_speed_mbps, status, target_ip)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, metrics.latency, metrics.jitter, metrics.packetLoss,
			metrics.download, metrics.upload, status, gatewayIP,
		)
		if err != nil {
			slog.Error("Failed to record ISP metrics", "link_id", id, "error", err)
		} else {
			slog.Debug("ISP metrics recorded", "link", name, "status", status,
				"latency", fmt.Sprintf("%.1fms", metrics.latency),
				"loss", fmt.Sprintf("%.1f%%", metrics.packetLoss))
		}
	}
}

type linkMetrics struct {
	latency    float64
	jitter     float64
	packetLoss float64
	download   float64
	upload     float64
}

var pingLossRe = regexp.MustCompile(`(\d+)% packet loss`)
var pingRTTRe = regexp.MustCompile(`(?:rtt|round-trip) min/avg/max(?:/mdev)? = [\d.]+/([\d.]+)/[\d.]+(?:/[\d.]+)? ms`)

func (c *ISPCollector) probeLink(ctx context.Context, target string) linkMetrics {
	m := linkMetrics{}

	if net.ParseIP(target) == nil {
		return m
	}

	cmd := exec.CommandContext(ctx, "ping", "-c", "10", "-W", "2", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.packetLoss = 100
		return m
	}

	output := string(out)

	if match := pingLossRe.FindStringSubmatch(output); len(match) > 1 {
		if v, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.packetLoss = v
		}
	}

	if match := pingRTTRe.FindStringSubmatch(output); len(match) > 1 {
		if v, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.latency = v
		}
	}

	if m.latency > 0 {
		m.jitter = m.latency * 0.1
	}

	m.download = measureDownload(ctx)
	m.upload = measureUpload(ctx)

	return m
}

var speedClient = &http.Client{Timeout: 15 * time.Second}

func measureDownload(ctx context.Context) float64 {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://speed.cloudflare.com/__down?bytes=10485760", nil)
	if err != nil {
		return 0
	}
	start := time.Now()
	resp, err := speedClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	if elapsed < 0.1 {
		return 0
	}
	return float64(n) * 8 / elapsed / 1000000.0
}

func measureUpload(ctx context.Context) float64 {
	body := strings.NewReader(strings.Repeat("0", 10485760))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://speed.cloudflare.com/__up", body)
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	start := time.Now()
	resp, err := speedClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	if elapsed < 0.1 {
		return 0
	}
	return float64(10485760) * 8 / elapsed / 1000000.0
}
