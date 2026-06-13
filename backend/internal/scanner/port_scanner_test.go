package scanner

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanPorts_OpenPort(t *testing.T) {
	t.Parallel()
	// Start a local listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	results := ScanPorts(context.Background(), "127.0.0.1", []int{port}, DefaultOptions)
	require.Len(t, results, 1)
	assert.Equal(t, port, results[0].Port)
	assert.True(t, results[0].Open)
}

func TestScanPorts_ClosedPort(t *testing.T) {
	t.Parallel()
	results := ScanPorts(context.Background(), "127.0.0.1", []int{19999}, DefaultOptions)
	require.Len(t, results, 1)
	assert.Equal(t, 19999, results[0].Port)
	assert.False(t, results[0].Open)
}

func TestScanPorts_MixedPorts(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	openPort := ln.Addr().(*net.TCPAddr).Port
	closedPort := 19998
	results := ScanPorts(context.Background(), "127.0.0.1", []int{openPort, closedPort}, DefaultOptions)
	require.Len(t, results, 2)
	assert.True(t, results[0].Open)
	assert.False(t, results[1].Open)
}

func TestScanPorts_DefaultOptions(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 100, DefaultOptions.Concurrency)
	assert.Equal(t, 2*time.Second, DefaultOptions.Timeout)
}

func TestScanPorts_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results := ScanPorts(ctx, "127.0.0.1", []int{19997}, DefaultOptions)
	require.Len(t, results, 1)
	assert.False(t, results[0].Open)
}

func TestScanPorts_EmptyPortList(t *testing.T) {
	t.Parallel()
	results := ScanPorts(context.Background(), "127.0.0.1", []int{}, DefaultOptions)
	assert.Empty(t, results)
}

func TestScanPorts_ConcurrencyLimit(t *testing.T) {
	t.Parallel()
	// Scan many ports with low concurrency — should complete without hanging
	ports := make([]int, 50)
	for i := range ports {
		ports[i] = 19000 + i
	}
	opts := ScanOptions{Concurrency: 5, Timeout: 500 * time.Millisecond}
	results := ScanPorts(context.Background(), "127.0.0.1", ports, opts)
	assert.Len(t, results, 50)
}

func TestCommonPorts(t *testing.T) {
	t.Parallel()
	assert.GreaterOrEqual(t, len(CommonPorts), 22)
	assert.Contains(t, CommonPorts, 22)
	assert.Contains(t, CommonPorts, 80)
	assert.Contains(t, CommonPorts, 443)
	assert.Contains(t, CommonPorts, 3306)
	assert.Contains(t, CommonPorts, 5432)
}
