package collectors

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCameraCollector_AuthenticatedManagementAndRTSPAreHealthy(t *testing.T) {
	mgmt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer mgmt.Close()
	host, port := testHostPort(t, mgmt.Listener.Addr().String())
	rtsp := rtspServer(t, "RTSP/1.0 401 Unauthorized\r\n")
	defer rtsp.Close()
	_, rtspPort := testHostPort(t, rtsp.Addr().String())
	r, err := (CameraCollector{}).Collect(context.Background(), &models.Device{IPAddress: host, Port: port, MonitorConfig: map[string]any{"rtspPort": rtspPort}})
	require.NoError(t, err)
	assert.Equal(t, "up", r.Status)
}

func TestBiometricCollector_DegradesWhenAttendanceServiceIsUnavailable(t *testing.T) {
	mgmt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer mgmt.Close()
	host, port := testHostPort(t, mgmt.Listener.Addr().String())
	r, err := (BiometricCollector{}).Collect(context.Background(), &models.Device{IPAddress: host, Port: port, MonitorConfig: map[string]any{"attendancePort": 1}})
	require.NoError(t, err)
	assert.Equal(t, "warning", r.Status)
}

func testHostPort(t *testing.T, address string) (string, int) {
	t.Helper()
	host, rawPort, err := net.SplitHostPort(address)
	require.NoError(t, err)
	port, err := strconv.Atoi(rawPort)
	require.NoError(t, err)
	return host, port
}
func rtspServer(t *testing.T, response string) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
		_, _ = conn.Write([]byte(strings.TrimSpace(response) + "\r\n"))
	}()
	return listener
}
