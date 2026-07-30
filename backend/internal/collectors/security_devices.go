package collectors

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rayavriti/netmonitor-backend/internal/models"
)

// CameraCollector validates both the camera/NVR management endpoint and an
// RTSP endpoint. An RTSP 401 is healthy: it proves the stream service is up
// while correctly refusing unauthenticated access.
type CameraCollector struct{}

func (CameraCollector) Name() string { return "camera" }

func (CameraCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	started := time.Now()
	config := device.MonitorConfig
	managementPort := configuredPort(config, "managementPort", device.Port, 80)
	scheme := configuredString(config, "managementScheme", "http")
	path := configuredString(config, "managementPath", "/")
	managementUp, managementCode, err := probeManagement(ctx, device.IPAddress, managementPort, scheme, path)
	if err != nil {
		managementUp = false
	}

	rtspPort := configuredPort(config, "rtspPort", 554, 554)
	rtspPath := configuredString(config, "rtspPath", "/")
	rtspUp, rtspCode, rtspErr := probeRTSP(ctx, device.IPAddress, rtspPort, rtspPath)
	if rtspErr != nil {
		rtspUp = false
	}
	status := "up"
	if !managementUp {
		status = "down"
	} else if !rtspUp {
		status = "warning"
	}
	details := map[string]any{"profile": "camera", "management": endpointDetail(managementUp, managementCode, err), "rtsp": endpointDetail(rtspUp, rtspCode, rtspErr)}
	return &Result{Status: status, ResponseTime: f64(float64(time.Since(started).Milliseconds())), Details: details}, nil
}

// BiometricCollector monitors the local management UI and the conventional
// ZKTeco-compatible attendance port (4370). Deployments can change either
// endpoint through monitorConfig without needing a new collector.
type BiometricCollector struct{}

func (BiometricCollector) Name() string { return "biometric" }

func (BiometricCollector) Collect(ctx context.Context, device *models.Device) (*Result, error) {
	started := time.Now()
	config := device.MonitorConfig
	managementPort := configuredPort(config, "managementPort", device.Port, 80)
	scheme := configuredString(config, "managementScheme", "http")
	path := configuredString(config, "managementPath", "/")
	managementUp, managementCode, err := probeManagement(ctx, device.IPAddress, managementPort, scheme, path)
	if err != nil {
		managementUp = false
	}
	attendancePort := configuredPort(config, "attendancePort", 4370, 4370)
	attendanceUp, attendanceErr := probeTCP(ctx, device.IPAddress, attendancePort)
	status := "up"
	if !managementUp && !attendanceUp {
		status = "down"
	} else if !managementUp || !attendanceUp {
		status = "warning"
	}
	details := map[string]any{"profile": "biometric", "management": endpointDetail(managementUp, managementCode, err), "attendance": endpointDetail(attendanceUp, attendancePort, attendanceErr)}
	return &Result{Status: status, ResponseTime: f64(float64(time.Since(started).Milliseconds())), Details: details}, nil
}

func configuredString(config map[string]any, key, fallback string) string {
	if value, ok := config[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func configuredPort(config map[string]any, key string, value, fallback int) int {
	if raw, ok := config[key]; ok {
		switch n := raw.(type) {
		case float64:
			if n >= 1 && n <= 65535 {
				return int(n)
			}
		case int:
			if n >= 1 && n <= 65535 {
				return n
			}
		case string:
			if port, err := strconv.Atoi(n); err == nil && port >= 1 && port <= 65535 {
				return port
			}
		}
	}
	if value >= 1 && value <= 65535 {
		return value
	}
	return fallback
}

func endpointDetail(up bool, code any, err error) map[string]any {
	detail := map[string]any{"up": up}
	if code != nil {
		detail["code"] = code
	}
	if err != nil {
		detail["error"] = err.Error()
	}
	return detail
}

func probeManagement(ctx context.Context, host string, port int, scheme, path string) (bool, int, error) {
	if scheme != "https" {
		scheme = "http"
	}
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: path}
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // monitoring private appliances with self-signed certificates
	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	// 401/403 are common for healthy management UIs and must not cause a false outage.
	return resp.StatusCode < 500, resp.StatusCode, nil
}

func probeTCP(ctx context.Context, host string, port int) (bool, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false, err
	}
	return true, conn.Close()
}

func probeRTSP(ctx context.Context, host string, port int, streamPath string) (bool, int, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = conn.Close() }()
	path := streamPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	request := fmt.Sprintf("OPTIONS rtsp://%s:%d%s RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: Rayavriti-NetMonitor\r\n\r\n", host, port, path)
	if _, err := conn.Write([]byte(request)); err != nil {
		return false, 0, err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, 0, err
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return false, 0, fmt.Errorf("invalid RTSP response")
	}
	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, 0, fmt.Errorf("invalid RTSP status: %w", err)
	}
	return code >= 200 && code < 500, code, nil
}
